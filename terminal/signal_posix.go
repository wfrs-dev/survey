//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package terminal

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// interruptedSignals are the signals whose default action terminates the
// process, and whose arrival therefore cannot wait for the caller's deferred
// RestoreTermMode to run.
//
// SIGQUIT belongs here even though it behaves differently from the others: the
// Go runtime installs its own handler for it, so once this one stops listening
// the re-raised signal reaches that handler, which prints the goroutine dump and
// exits with status 2 instead of dying by the signal. Restoring first is exactly
// the point — a stack dump is unreadable on a terminal left in raw mode.
var interruptedSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}

// signalRestorer returns the terminal to the mode SetTermMode replaced when the
// process is interrupted while the terminal is in raw mode.
//
// It owns a goroutine of its own on purpose: the interruption normally arrives
// while the caller is blocked in a read on that very terminal, so the restore
// cannot be left to the reading goroutine.
//
// The handler lives here rather than being opt-in because this package is the
// only place that knows both the mode it replaced and the window in which the
// terminal is raw, and because an opt-in would need a new exported symbol. It
// is armed by SetTermMode and dropped by RestoreTermMode, so on a normal run it
// exists only while the terminal is actually raw.
type signalRestorer struct {
	fd   int
	term *unix.Termios
	sigs chan os.Signal
	stop chan struct{}
	done chan struct{}

	stopOnce sync.Once
}

func newSignalRestorer(fd int, term *unix.Termios) *signalRestorer {
	r := &signalRestorer{
		fd:   fd,
		term: term,
		sigs: make(chan os.Signal, 1),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	signal.Notify(r.sigs, interruptedSignals...)
	go r.watch()
	return r
}

// watch restores the terminal and then lets the signal do what it would have
// done without this handler.
func (r *signalRestorer) watch() {
	defer close(r.done)

	select {
	case sig := <-r.sigs:
		// The restore is a plain ioctl on the fd, so it does not depend on the
		// reading goroutine making progress. Writing the same state twice is
		// harmless, which is what keeps repeated signals safe.
		_ = unix.IoctlSetTermios(r.fd, ioctlWriteTermios, r.term)

		// Stop listening on our own channel before re-raising. signal.Stop only
		// detaches this channel; it does not strip handlers that the embedding
		// application installed, so its own shutdown hook still runs. When
		// nothing else is listening, the default disposition applies and the
		// process dies as it would have without us.
		signal.Stop(r.sigs)
		if s, ok := sig.(syscall.Signal); ok {
			_ = syscall.Kill(os.Getpid(), s)
		}
	case <-r.stop:
	}
}

// close stops watching for interruptions. It is safe to call more than once.
func (r *signalRestorer) close() {
	r.stopOnce.Do(func() {
		signal.Stop(r.sigs)
		close(r.stop)
		<-r.done
	})
}

// startSignalRestore arms the interruption handler for this reader's terminal.
func (s *runeReaderState) startSignalRestore(fd int) {
	s.stopSignalRestore()
	s.restorer = newSignalRestorer(fd, &s.term)
}

// stopSignalRestore drops the interruption handler, if any.
func (s *runeReaderState) stopSignalRestore() {
	if s.restorer != nil {
		s.restorer.close()
		s.restorer = nil
	}
}
