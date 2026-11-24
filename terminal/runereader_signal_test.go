//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package terminal_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/wfrs-dev/survey/v2/terminal"
	"golang.org/x/sys/unix"
)

// signalChildEnv marks the re-executed test binary as the process under test,
// and carries the signal it must receive.
const signalChildEnv = "SURVEY_TERMINAL_SIGNAL_CHILD"

// The child exits with these codes when a step of the protocol fails, so a
// failure is reported as a message instead of a puzzling signal death.
const (
	childExitSetTermMode = 3
	childExitWaitToken   = 4
	childExitKillFailed  = 5
	childExitStillAlive  = 6
)

func termiosOf(t *testing.T, f *os.File) *unix.Termios {
	t.Helper()
	term, err := unix.IoctlGetTermios(int(f.Fd()), testReadTermios)
	if err != nil {
		t.Fatalf("IoctlGetTermios(%s): %v", f.Name(), err)
	}
	return term
}

func wantSignal(t *testing.T, name string) syscall.Signal {
	t.Helper()
	switch name {
	case "SIGINT":
		return syscall.SIGINT
	case "SIGTERM":
		return syscall.SIGTERM
	case "SIGHUP":
		return syscall.SIGHUP
	}
	t.Fatalf("unknown helper signal %q", name)
	return 0
}

// TestTermModeRestoredOnSignal asserts that a process interrupted while its
// terminal is in raw mode leaves that terminal usable again. The child blocks a
// read on the terminal before it is signalled, so the restore cannot be coming
// from the reading goroutine.
func TestTermModeRestoredOnSignal(t *testing.T) {
	for _, name := range []string{"SIGINT", "SIGTERM", "SIGHUP"} {
		t.Run(name, func(t *testing.T) {
			testTermModeRestoredOnSignal(t, name)
		})
	}
}

func testTermModeRestoredOnSignal(t *testing.T, sigName string) {
	t.Helper()

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("failed to open pseudotty: %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	defer func() { _ = tty.Close() }()

	before := termiosOf(t, tty)

	cmd := exec.Command(os.Args[0], "-test.run=^TestSignalHelperProcess$", "-test.timeout=60s")
	cmd.Env = append(os.Environ(), signalChildEnv+"="+sigName)
	// The child inspects and mutates the very terminal the parent keeps open.
	cmd.ExtraFiles = []*os.File{tty}
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// The child reports raw mode only after SetTermMode succeeded, which also
	// means the interruption handler is armed.
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("helper process did not report raw mode: %v", err)
	}
	if got := strings.TrimSpace(line); got != "raw" {
		t.Fatalf("helper process reported %q, want %q", got, "raw")
	}

	raw := termiosOf(t, tty)
	if raw.Lflag&(unix.ECHO|unix.ICANON|unix.ISIG) != 0 {
		t.Fatalf("terminal is not in raw mode while the helper reads: Lflag=%#x", raw.Lflag)
	}

	// Release the child so it signals itself.
	if _, err := io.WriteString(stdin, "go"); err != nil {
		t.Fatalf("failed to release helper process: %v", err)
	}

	err = cmd.Wait()
	want := wantSignal(t, sigName)
	status, ok := exitStatus(err)
	if !ok {
		t.Fatalf("helper process did not die from a signal (want %v): err=%v", want, err)
	}
	if !status.Signaled() || status.Signal() != want {
		t.Fatalf("helper process terminated with %v, want death by %v", status, want)
	}

	after := termiosOf(t, tty)
	if after.Lflag != before.Lflag {
		t.Fatalf("terminal left with Lflag=%#x after %s, want the previous %#x (ECHO=%v ICANON=%v ISIG=%v)",
			after.Lflag, sigName, before.Lflag,
			after.Lflag&unix.ECHO != 0, after.Lflag&unix.ICANON != 0, after.Lflag&unix.ISIG != 0)
	}
	if after.Cc[unix.VMIN] != before.Cc[unix.VMIN] || after.Cc[unix.VTIME] != before.Cc[unix.VTIME] {
		t.Fatalf("terminal left with VMIN=%d VTIME=%d after %s, want the previous VMIN=%d VTIME=%d",
			after.Cc[unix.VMIN], after.Cc[unix.VTIME], sigName,
			before.Cc[unix.VMIN], before.Cc[unix.VTIME])
	}
}

// TestSetAndRestoreTermMode pins the untouched part of the contract: raw mode
// is what the reader needs, and every restore puts back exactly the mode that
// was there before, no matter how often it is asked to.
func TestSetAndRestoreTermMode(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("failed to open pseudotty: %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	defer func() { _ = tty.Close() }()

	before := termiosOf(t, tty)

	rr := terminal.NewRuneReader(terminal.Stdio{In: tty, Out: tty, Err: tty})
	if err := rr.SetTermMode(); err != nil {
		t.Fatalf("SetTermMode: %v", err)
	}

	raw := termiosOf(t, tty)
	if raw.Lflag&(unix.ECHO|unix.ECHONL|unix.ICANON|unix.ISIG) != 0 {
		t.Fatalf("SetTermMode left Lflag=%#x, want ECHO, ECHONL, ICANON and ISIG cleared", raw.Lflag)
	}
	if raw.Cc[unix.VMIN] != 1 || raw.Cc[unix.VTIME] != 0 {
		t.Fatalf("SetTermMode left VMIN=%d VTIME=%d, want 1 and 0", raw.Cc[unix.VMIN], raw.Cc[unix.VTIME])
	}

	// Restoring twice must be harmless, which is what makes the interruption
	// handler safe to run next to a caller's deferred restore.
	for i := 1; i <= 2; i++ {
		if err := rr.RestoreTermMode(); err != nil {
			t.Fatalf("RestoreTermMode #%d: %v", i, err)
		}
		if got := termiosOf(t, tty); got.Lflag != before.Lflag {
			t.Fatalf("RestoreTermMode #%d left Lflag=%#x, want %#x", i, got.Lflag, before.Lflag)
		}
	}

	// A second session on the same reader must restore the same mode.
	if err := rr.SetTermMode(); err != nil {
		t.Fatalf("second SetTermMode: %v", err)
	}
	if got := termiosOf(t, tty); got.Lflag&(unix.ECHO|unix.ICANON|unix.ISIG) != 0 {
		t.Fatalf("second SetTermMode left Lflag=%#x, want the raw mode again", got.Lflag)
	}
	if err := rr.RestoreTermMode(); err != nil {
		t.Fatalf("second RestoreTermMode: %v", err)
	}
	if got := termiosOf(t, tty); got.Lflag != before.Lflag {
		t.Fatalf("second RestoreTermMode left Lflag=%#x, want %#x", got.Lflag, before.Lflag)
	}
}

func exitStatus(err error) (syscall.WaitStatus, bool) {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 0, false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return status, ok
}

// TestSignalHelperProcess is the body of the child process used by
// TestTermModeRestoredOnSignal. It is skipped in a normal run.
func TestSignalHelperProcess(t *testing.T) {
	sigName := os.Getenv(signalChildEnv)
	if sigName == "" {
		t.Skip("helper process for TestTermModeRestoredOnSignal")
	}

	tty := os.NewFile(3, "pseudotty-slave")
	if tty == nil {
		t.Fatal("helper process has no pseudotty on fd 3")
	}

	rr := terminal.NewRuneReader(terminal.Stdio{In: tty, Out: tty, Err: os.Stderr})
	if err := rr.SetTermMode(); err != nil {
		fmt.Fprintf(os.Stderr, "SetTermMode: %v\n", err)
		os.Exit(childExitSetTermMode)
	}

	// A prompt waiting for a key: this goroutine never returns before the
	// signal arrives, so the restore must come from somewhere else.
	go func() {
		_, _, _ = rr.ReadRune()
	}()

	fmt.Println("raw")

	// Wait for the parent's token so the test observes the raw state on its
	// own terms instead of racing the signal.
	var token [1]byte
	if _, err := os.Stdin.Read(token[:]); err != nil {
		fmt.Fprintf(os.Stderr, "waiting for token: %v\n", err)
		os.Exit(childExitWaitToken)
	}

	if err := syscall.Kill(os.Getpid(), wantSignal(t, sigName)); err != nil {
		fmt.Fprintf(os.Stderr, "kill: %v\n", err)
		os.Exit(childExitKillFailed)
	}

	// The default disposition must terminate the process; reaching this point
	// means the signal was swallowed.
	time.Sleep(10 * time.Second)
	fmt.Fprintln(os.Stderr, "the signal did not terminate the process")
	os.Exit(childExitStillAlive)
}
