//go:build !windows
// +build !windows

package survey

import (
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
	pseudotty "github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/wfrs-dev/survey/v2/terminal"
)

// testProcedureTimeout bounds how long an interactive procedure may take to
// reach its last expectation before the test fails.
const testProcedureTimeout = 10 * time.Second

func RunTest(t *testing.T, procedure func(expectConsole), test func(terminal.Stdio) error) {
	t.Helper()
	t.Parallel()

	pty, tty, err := pseudotty.Open()
	if err != nil {
		t.Fatalf("failed to open pseudotty: %v", err)
	}

	term := vt10x.New(vt10x.WithWriter(tty))
	c, err := expect.NewConsole(expect.WithStdin(pty), expect.WithStdout(term), expect.WithCloser(pty, tty))
	if err != nil {
		t.Fatalf("failed to create console: %v", err)
	}
	defer c.Close()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		procedure(&consoleWithErrorHandling{console: c, t: t})
	}()

	stdio := terminal.Stdio{In: c.Tty(), Out: c.Tty(), Err: c.Tty()}
	if err := test(stdio); err != nil {
		t.Error(err)
	}

	if err := c.Tty().Close(); err != nil {
		t.Errorf("error closing Tty: %v", err)
	}

	// The procedure runs in its own goroutine and blocks on the pty. Without a
	// bound, a single unmet expectation prevents the test from ever finishing
	// and the whole package dies against the `go test -timeout` deadline with no
	// indication of which expectation did not match. Fail here instead.
	select {
	case <-donec:
	case <-time.After(testProcedureTimeout):
		t.Fatalf("prompt procedure did not finish within %s", testProcedureTimeout)
	}
}
