package shared

import "testing"

// TestInstallContext_Cancelled: the step loop asks this before every step and the
// command primitives ask it before every command, so most of the time they are
// asking about a context that has no channel at all — a unit test, or one step
// run by hand. That answer must be "not cancelled".
func TestInstallContext_Cancelled(t *testing.T) {
	if (&InstallContext{}).Cancelled() {
		t.Error("a context with no cancellation channel is never cancelled")
	}

	open := make(chan struct{})
	ctx := &InstallContext{Cancel: open}
	if ctx.Cancelled() {
		t.Error("an open channel is not a cancellation")
	}

	close(open)
	if !ctx.Cancelled() {
		t.Error("a closed channel is a cancellation")
	}
}
