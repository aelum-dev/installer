package install

import (
	"errors"
	"strings"
	"testing"
	"time"

	"aelum-installer/backend/shared"
)

// cancelTestInstaller is a dry-run installer with just enough of an answer set to
// reach the step loop.
func cancelTestInstaller(t *testing.T) *Installer {
	t.Helper()
	t.Setenv("FAKE_DISKS", "1")
	return NewInstaller(InstallerConfig{
		DiskID:     "/dev/vda",
		Username:   "testuser",
		Password:   "testpass",
		Hostname:   "testhost",
		Locale:     "en_US.UTF-8",
		Region:     shared.GetRegion("us"),
		FileSystem: "ext4",
		DryRun:     true,
	})
}

// overrideSimulate replaces the dry-run step body for one test and restores the
// package default afterwards. The suite's TestMain installs its own fast stub,
// so the whole point of these tests is that no step ever runs for real.
func overrideSimulate(t *testing.T, fn func(cancel chan struct{}, d time.Duration) error) {
	t.Helper()
	previous := simulateStep
	simulateStep = fn
	t.Cleanup(func() { simulateStep = previous })
}

// TestInstall_CancelStopsRemainingSteps is the bug this covers: the step loop
// used to ignore the cancellation channel, so a cancelled run finished the step
// it was in and then ran every step after it, ending with a completed install
// reported as a failure. Here the base step returns cleanly while cancelled —
// only the loop can be what stops the run.
func TestInstall_CancelStopsRemainingSteps(t *testing.T) {
	inst := cancelTestInstaller(t)

	var started []string
	inst.OnProgress = func(stepID, _ string, _ int, phase string) {
		if phase != "running" {
			return
		}
		started = append(started, stepID)
		if stepID == "install_base" {
			inst.Cancel()
		}
	}

	err := inst.Install()
	if !errors.Is(err, shared.ErrInterrupted) {
		t.Fatalf("Install() error = %v, want the interrupted error", err)
	}
	if !inst.Interrupted() {
		t.Error("a cancelled run must report itself interrupted, so the CLI exits 130")
	}

	want := []string{"format_disk", "mount_filesystem", "install_base"}
	if strings.Join(started, " ") != strings.Join(want, " ") {
		t.Errorf("steps started = %v, want only %v", started, want)
	}
}

// TestInstall_CancelledStepIsNotReportedAsFailure covers the other half: a step
// that notices the cancellation dies with a killed command, and that must not be
// written up as a step that failed.
func TestInstall_CancelledStepIsNotReportedAsFailure(t *testing.T) {
	inst := cancelTestInstaller(t)
	overrideSimulate(t, func(_ chan struct{}, _ time.Duration) error {
		inst.Cancel()
		return errors.New("signal: killed")
	})

	err := inst.Install()
	if !errors.Is(err, shared.ErrInterrupted) {
		t.Fatalf("Install() error = %v, want the interrupted error", err)
	}
	if strings.Contains(err.Error(), "failed") {
		t.Errorf("a cancelled run must not read like a broken installer: %v", err)
	}
	if !strings.Contains(err.Error(), "Formatting disk") {
		t.Errorf("the error should name where the run stopped, got: %v", err)
	}
	if !inst.Interrupted() {
		t.Error("a cancelled run must report itself interrupted")
	}
}

// TestRunStep_CancelledStepIsNotReportedAsFailure is the same rule on the
// single-step path the CLI's --format-disk and friends take.
func TestRunStep_CancelledStepIsNotReportedAsFailure(t *testing.T) {
	inst := cancelTestInstaller(t)
	overrideSimulate(t, func(_ chan struct{}, _ time.Duration) error {
		inst.Cancel()
		return errors.New("signal: killed")
	})

	_, _, err := inst.RunStep("format_disk")
	if !errors.Is(err, shared.ErrInterrupted) {
		t.Fatalf("RunStep() error = %v, want the interrupted error", err)
	}
	if strings.Contains(err.Error(), "failed") {
		t.Errorf("a cancelled step must not read like a failed one: %v", err)
	}
	if !inst.Interrupted() {
		t.Error("a cancelled run must report itself interrupted")
	}
}

// TestCancel_CoversEveryRun holds the shape of the cancel channel across runs.
// Guarding the close with a sync.Once made the first Cancel the last one for the
// whole Installer: a retried run got a fresh channel that nothing could close,
// and could not be cancelled at all.
func TestCancel_CoversEveryRun(t *testing.T) {
	inst := cancelTestInstaller(t)

	first := inst.context().Cancel
	inst.Cancel()
	select {
	case <-first:
	default:
		t.Fatal("Cancel did not close the current run's channel")
	}

	second := inst.context().Cancel
	if second == first {
		t.Fatal("each run must get a fresh cancellation channel")
	}
	select {
	case <-second:
		t.Fatal("a new run must not start cancelled")
	default:
	}

	inst.Cancel()
	select {
	case <-second:
	default:
		t.Error("Cancel must close the channel of the current run, not only the first one")
	}
}

// TestContext_CancelledWithoutChannel: a context with no channel is a unit test
// that runs one step, and it is never cancelled.
func TestContext_CancelledWithoutChannel(t *testing.T) {
	if (&shared.InstallContext{}).Cancelled() {
		t.Error("a context with no cancellation channel must report it is not cancelled")
	}
}
