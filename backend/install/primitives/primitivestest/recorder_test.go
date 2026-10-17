package primitivestest

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
)

func cmd(args ...string) *primitives.Command {
	return &primitives.Command{Args: args}
}

// TestRecorder_DefaultSuccess verifies unmatched commands succeed with no output.
func TestRecorder_DefaultSuccess(t *testing.T) {
	r := NewRecorder()
	out, err := r.CombinedOutput(cmd("echo", "hi"))
	if err != nil {
		t.Fatalf("CombinedOutput() err = %v, want nil", err)
	}
	if len(out) != 0 {
		t.Errorf("output = %q, want empty", out)
	}
}

// TestRecorder_ErrorResult verifies a scripted error surfaces unchanged through
// Run, Output, and CombinedOutput.
func TestRecorder_ErrorResult(t *testing.T) {
	sentinel := errors.New("boom")
	r := NewRecorder().OnError("false", sentinel)

	if err := r.Run(cmd("false")); !errors.Is(err, sentinel) {
		t.Errorf("Run() err = %v, want sentinel", err)
	}
	if _, err := r.Output(cmd("false")); !errors.Is(err, sentinel) {
		t.Errorf("Output() err = %v, want sentinel", err)
	}
	if _, err := r.CombinedOutput(cmd("false")); !errors.Is(err, sentinel) {
		t.Errorf("CombinedOutput() err = %v, want sentinel", err)
	}
}

// TestRecorder_MostRecentRuleWins verifies a later, more specific argv rule
// overrides an earlier program-name rule for matching commands.
func TestRecorder_MostRecentRuleWins(t *testing.T) {
	r := NewRecorder().
		OnOutput("echo", "generic").
		OnArgsOutput([]string{"echo", "special"}, "specific")

	if out, _ := r.Output(cmd("echo", "special")); string(out) != "specific" {
		t.Errorf("specific call = %q, want %q", out, "specific")
	}
	if out, _ := r.Output(cmd("echo", "other")); string(out) != "generic" {
		t.Errorf("generic call = %q, want %q", out, "generic")
	}
}

// TestRecorder_CapturesStdin verifies the stdin bytes are recorded.
func TestRecorder_CapturesStdin(t *testing.T) {
	r := NewRecorder()
	c := cmd("cat")
	c.Stdin = strings.NewReader("secret")

	if _, err := r.Output(c); err != nil {
		t.Fatalf("Output() err = %v", err)
	}
	call, ok := r.LastCall()
	if !ok {
		t.Fatal("LastCall() reported no call")
	}
	if call.Stdin != "secret" {
		t.Errorf("recorded stdin = %q, want %q", call.Stdin, "secret")
	}
}

// TestRecorder_CalledAndCallsFor verifies the query helpers.
func TestRecorder_CalledAndCallsFor(t *testing.T) {
	r := NewRecorder()
	_ = r.Run(cmd("a", "1"))
	_ = r.Run(cmd("b"))
	_ = r.Run(cmd("a", "2"))

	if !r.Called("a") {
		t.Error("Called(\"a\") = false, want true")
	}
	if r.Called("c") {
		t.Error("Called(\"c\") = true, want false")
	}
	if got := r.CallsFor("a"); len(got) != 2 {
		t.Errorf("CallsFor(\"a\") = %v, want 2 calls", got)
	}
	last, ok := r.LastCall()
	if !ok || last.Args[0] != "a" || last.Args[1] != "2" {
		t.Errorf("LastCall() = %+v, want the final a 2", last)
	}
}

// TestRecorder_Reset verifies calls and rules are cleared.
func TestRecorder_Reset(t *testing.T) {
	r := NewRecorder().OnOutput("echo", "x")
	_ = r.Run(cmd("echo"))
	r.Reset()

	if r.Called("echo") {
		t.Error("calls were not cleared by Reset")
	}
	if out, _ := r.Output(cmd("echo")); len(out) != 0 {
		t.Errorf("rules were not cleared by Reset: output %q", out)
	}
}
