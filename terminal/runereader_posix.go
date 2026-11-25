//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

// The terminal mode manipulation code is derived heavily from:
// https://github.com/golang/crypto/blob/master/ssh/terminal/util.go:
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terminal

import (
	"bufio"
	"bytes"
	"fmt"

	"golang.org/x/sys/unix"
)

const (
	normalKeypad      = '['
	applicationKeypad = 'O'
)

type runeReaderState struct {
	term     unix.Termios
	reader   *bufio.Reader
	buf      *bytes.Buffer
	restorer *signalRestorer
}

func newRuneReaderState(input FileReader) runeReaderState {
	buf := new(bytes.Buffer)
	return runeReaderState{
		reader: bufio.NewReader(&BufferedReader{
			In:     input,
			Buffer: buf,
		}),
		buf: buf,
	}
}

func (rr *RuneReader) Buffer() *bytes.Buffer {
	return rr.state.buf
}

// For reading runes we just want to disable echo.
func (rr *RuneReader) SetTermMode() error {
	fd := int(rr.stdio.In.Fd())

	saved, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return err
	}

	// Drop the handler of a previous SetTermMode before the state it protects
	// is rewritten, so the two never race.
	rr.state.stopSignalRestore()
	rr.state.term = *saved

	newState := *saved
	newState.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG
	// Because we are clearing canonical mode, we need to ensure VMIN & VTIME are
	// set to the values we expect. This combination puts things in standard
	// "blocking read" mode (see termios(3)).
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0

	// Arm the interruption handler before the terminal is mutated: from here on
	// every raw window has something able to undo it.
	rr.state.startSignalRestore(fd)

	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, &newState); err != nil {
		rr.state.stopSignalRestore()
		return err
	}

	return nil
}

func (rr *RuneReader) RestoreTermMode() error {
	// Restore before dropping the handler: an interruption in that window then
	// finds either a handler that writes this same state or a terminal that is
	// already restored, so neither order can leave it raw.
	err := unix.IoctlSetTermios(int(rr.stdio.In.Fd()), ioctlWriteTermios, &rr.state.term)
	rr.state.stopSignalRestore()
	return err
}

// ReadRune Parse escape sequences such as ESC [ A for arrow keys.
// See https://vt100.net/docs/vt102-ug/appendixc.html
func (rr *RuneReader) ReadRune() (rune, int, error) {
	r, size, err := rr.state.reader.ReadRune()
	if err != nil {
		return r, size, err
	}

	if r != KeyEscape {
		return r, size, err
	}

	if rr.state.reader.Buffered() == 0 {
		// no more characters so must be `Esc` key
		return KeyEscape, 1, nil
	}

	r, size, err = rr.state.reader.ReadRune()
	if err != nil {
		return r, size, err
	}

	// ESC O ... or ESC [ ...?
	if r != normalKeypad && r != applicationKeypad {
		return r, size, fmt.Errorf("unexpected escape sequence from terminal: %q", []rune{KeyEscape, r})
	}

	keypad := r

	r, size, err = rr.state.reader.ReadRune()
	if err != nil {
		return r, size, err
	}

	switch r {
	case 'A': // ESC [ A or ESC O A
		return KeyArrowUp, 1, nil
	case 'B': // ESC [ B or ESC O B
		return KeyArrowDown, 1, nil
	case 'C': // ESC [ C or ESC O C
		return KeyArrowRight, 1, nil
	case 'D': // ESC [ D or ESC O D
		return KeyArrowLeft, 1, nil
	case 'F': // ESC [ F or ESC O F
		return SpecialKeyEnd, 1, nil
	case 'H': // ESC [ H or ESC O H
		return SpecialKeyHome, 1, nil
	case '3': // ESC [ 3
		if keypad == normalKeypad {
			// discard the following '~' key from buffer
			_, _ = rr.state.reader.Discard(1)
			return SpecialKeyDelete, 1, nil
		}
	}

	// discard the following '~' key from buffer
	_, _ = rr.state.reader.Discard(1)
	return IgnoreKey, 1, nil
}
