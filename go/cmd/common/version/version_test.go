package version_test

import (
	"io"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"

	cmdversion "go.5x5.cz/inventario/cmd/common/version"
	"go.5x5.cz/inventario/internal/version"
)

// TestVersionCommand_WritesToStdout is the regression test for #2452.
//
// Cobra's Print family writes to OutOrStderr, which falls back to os.Stderr
// only when no writer is set on the command. So the bug is invisible to a test
// that calls SetOut: cobra then writes to that writer either way and the
// assertion passes against the broken code. The real descriptors have to be
// captured instead, with the command left as the binary runs it.
func TestVersionCommand_WritesToStdout(t *testing.T) {
	c := qt.New(t)

	stdout, stderr := captureStdio(c, func() {
		cmd := cmdversion.New()
		// Empty, not nil: cobra falls back to os.Args[1:] when args are nil
		// and the binary is not named cobra.test, so a nil here would hand the
		// command whatever flags `go test` was invoked with.
		cmd.SetArgs([]string{})
		c.Assert(cmd.Execute(), qt.IsNil)
	})

	c.Assert(stdout, qt.Equals, version.String()+"\n")
	c.Assert(stderr, qt.Equals, "",
		qt.Commentf("the version is the command's answer, not a diagnostic"))
}

// captureStdio swaps os.Stdout and os.Stderr for pipes, runs fn, and returns
// what each received. Both are drained concurrently: a command writing more
// than the pipe buffer would otherwise block forever on the write.
func captureStdio(c *qt.C, fn func()) (stdout, stderr string) {
	c.Helper()

	outR, outW, err := os.Pipe()
	c.Assert(err, qt.IsNil)
	errR, errW, err := os.Pipe()
	c.Assert(err, qt.IsNil)

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	outCh := readAll(outR)
	errCh := readAll(errR)

	fn()

	c.Assert(outW.Close(), qt.IsNil)
	c.Assert(errW.Close(), qt.IsNil)

	return <-outCh, <-errCh
}

func readAll(r *os.File) <-chan string {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		b, _ := io.ReadAll(r)
		ch <- string(b)
	}()
	return ch
}
