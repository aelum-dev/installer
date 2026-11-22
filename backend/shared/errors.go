package shared

import "errors"

// ErrInterrupted reports that the installation was cancelled while it ran.
//
// Every layer returns this same error for that one event: the command
// primitives (a command killed, or a next one refused), the streaming pacstrap
// in installbase, and the step loop that stops the run. Callers test for it
// with errors.Is rather than matching text, and the CLI turns it into the
// signal exit code.
var ErrInterrupted = errors.New("installation interrupted")
