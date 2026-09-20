package command

import "errors"

// ExitCodeFailure is the status every command failure gets unless it asks for
// something more specific.
const ExitCodeFailure = 1

// ExitCoder lets a command distinguish one failure from another in the only
// channel a shell can read cheaply: the process exit status. A wrapper script
// that retries a command needs to know whether the failure is worth retrying,
// and grepping stderr for a phrase is not a contract anyone should depend on.
//
// Implement it on an error a RunE returns; the root command maps it through
// ExitCodeFor.
type ExitCoder interface {
	error
	ExitCode() int
}

// ExitCodeFor resolves the process exit status for a command error: 0 for no
// error, the code an ExitCoder anywhere in the chain asks for, ExitCodeFailure
// otherwise.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	if coder, ok := errors.AsType[ExitCoder](err); ok {
		if code := coder.ExitCode(); code != 0 {
			return code
		}
	}
	return ExitCodeFailure
}

// WithExitCode attaches an exit status to err. A nil err stays nil so callers
// can wrap unconditionally.
func WithExitCode(err error, code int) error {
	if err == nil {
		return nil
	}
	return exitCodeError{err: err, code: code}
}

type exitCodeError struct {
	err  error
	code int
}

func (e exitCodeError) Error() string { return e.err.Error() }
func (e exitCodeError) Unwrap() error { return e.err }
func (e exitCodeError) ExitCode() int { return e.code }
