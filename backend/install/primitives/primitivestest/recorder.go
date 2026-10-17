// Package primitivestest provides a scripted primitives.CommandRunner for
// unit-testing install steps without root and without touching real devices.
//
// Typical use:
//
//	rec := primitivestest.NewRecorder().
//		OnError("findmnt", errors.New("not mounted"))
//	defer primitives.SetCommandRunner(rec)()
//
//	err := someStep.Run(ctx)
//	if !rec.Called("findmnt") { ... }
package primitivestest

import (
	"io"

	"aelum-installer/backend/install/primitives"
)

// Call records one invoked command: the full argv (Args[0] is the program name)
// and the bytes supplied on stdin, if any.
type Call struct {
	Args  []string
	Stdin string
}

// Result is the canned outcome returned for a matching command.
type Result struct {
	Output []byte
	Err    error
}

// rule pairs an argv prefix with the result to return when it matches.
type rule struct {
	prefix []string
	result Result
}

// Recorder is a primitives.CommandRunner that records every command it is
// asked to run and returns scripted results. A command matches the most
// recently registered rule whose argv prefix it starts with; unmatched
// commands succeed with empty output.
type Recorder struct {
	calls []Call
	rules []rule
}

// NewRecorder returns an empty Recorder. Unmatched commands succeed silently.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// On registers a result for commands whose program name (Args[0]) is name.
func (r *Recorder) On(name string, res Result) *Recorder {
	return r.OnArgs([]string{name}, res)
}

// OnArgs registers a result for commands whose argv starts with prefix. Use it
// to distinguish subcommands that share a program (e.g. cryptsetup luksFormat
// vs cryptsetup luksUUID). The most recently registered matching rule wins.
func (r *Recorder) OnArgs(prefix []string, res Result) *Recorder {
	r.rules = append(r.rules, rule{prefix: append([]string(nil), prefix...), result: res})
	return r
}

// OnOutput registers a successful result carrying output for a program name.
func (r *Recorder) OnOutput(name, output string) *Recorder {
	return r.On(name, Result{Output: []byte(output)})
}

// OnError registers a failing result for a program name.
func (r *Recorder) OnError(name string, err error) *Recorder {
	return r.On(name, Result{Err: err})
}

// OnArgsError registers a failing result for an argv prefix.
func (r *Recorder) OnArgsError(prefix []string, err error) *Recorder {
	return r.OnArgs(prefix, Result{Err: err})
}

// OnArgsOutput registers a successful result carrying output for an argv prefix.
func (r *Recorder) OnArgsOutput(prefix []string, output string) *Recorder {
	return r.OnArgs(prefix, Result{Output: []byte(output)})
}

// hasPrefix reports whether argv starts with prefix.
func hasPrefix(argv, prefix []string) bool {
	if len(argv) < len(prefix) {
		return false
	}
	for i := range prefix {
		if argv[i] != prefix[i] {
			return false
		}
	}
	return true
}

// resolve returns the result for cmd, scanning rules from most recent to least.
func (r *Recorder) resolve(cmd *primitives.Command) Result {
	for i := len(r.rules) - 1; i >= 0; i-- {
		if hasPrefix(cmd.Args, r.rules[i].prefix) {
			return r.rules[i].result
		}
	}
	return Result{}
}

// record captures the command (argv + stdin) and returns its scripted result.
func (r *Recorder) record(cmd *primitives.Command) Result {
	call := Call{Args: append([]string(nil), cmd.Args...)}
	if cmd.Stdin != nil {
		if data, err := io.ReadAll(cmd.Stdin); err == nil {
			call.Stdin = string(data)
		}
	}
	r.calls = append(r.calls, call)
	return r.resolve(cmd)
}

// Run implements primitives.CommandRunner.
func (r *Recorder) Run(cmd *primitives.Command) error {
	return r.record(cmd).Err
}

// Output implements primitives.CommandRunner.
func (r *Recorder) Output(cmd *primitives.Command) ([]byte, error) {
	res := r.record(cmd)
	return res.Output, res.Err
}

// CombinedOutput implements primitives.CommandRunner.
func (r *Recorder) CombinedOutput(cmd *primitives.Command) ([]byte, error) {
	res := r.record(cmd)
	return res.Output, res.Err
}

// Calls returns every command recorded so far, in invocation order.
func (r *Recorder) Calls() []Call {
	return append([]Call(nil), r.calls...)
}

// Called reports whether a command whose program name is name was invoked.
func (r *Recorder) Called(name string) bool {
	return len(r.CallsFor(name)) > 0
}

// CallsFor returns the argv of every command whose program name is name.
func (r *Recorder) CallsFor(name string) [][]string {
	var out [][]string
	for _, c := range r.calls {
		if len(c.Args) > 0 && c.Args[0] == name {
			out = append(out, c.Args)
		}
	}
	return out
}

// LastCall returns the most recently recorded command, or false if none.
func (r *Recorder) LastCall() (Call, bool) {
	if len(r.calls) == 0 {
		return Call{}, false
	}
	return r.calls[len(r.calls)-1], true
}

// Reset clears all recorded calls and rules.
func (r *Recorder) Reset() {
	r.calls = nil
	r.rules = nil
}

var _ primitives.CommandRunner = (*Recorder)(nil)
