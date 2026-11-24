//go:build linux

package terminal_test

import "golang.org/x/sys/unix"

// testReadTermios is the request the tests use to observe the terminal mode of
// a pseudotty without going through the package under test.
const testReadTermios = unix.TCGETS
