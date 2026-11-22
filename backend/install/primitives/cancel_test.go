package primitives

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aelum-installer/backend/shared"
)

// TestExecRunner_CancelledBeforeStartStartsNothing: the point of refusing a
// command is that a cancelled install writes nothing further to the disk.
func TestExecRunner_CancelledBeforeStartStartsNothing(t *testing.T) {
	defer SetCommandRunner(execRunner{})()

	closed := make(chan struct{})
	close(closed)
	defer SetCancellation(closed)()

	path := filepath.Join(t.TempDir(), "written-by-the-cancelled-command")
	if _, err := Run("touch", path).Output(); !errors.Is(err, shared.ErrInterrupted) {
		t.Fatalf("Run(touch) after cancel = %v, want the interrupted error", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a cancelled run started its command anyway: %v", err)
	}
}

// TestExecRunner_KillsRunningCommandOnCancel: without this, cancelling waits for
// whatever command the step happens to be waiting on — an mkinitcpio or a
// cryptsetup — and only then stops.
func TestExecRunner_KillsRunningCommandOnCancel(t *testing.T) {
	defer SetCommandRunner(execRunner{})()

	cancel := make(chan struct{})
	defer SetCancellation(cancel)()
	time.AfterFunc(50*time.Millisecond, func() { close(cancel) })

	start := time.Now()
	err := Run("sleep", "30").Run()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run(sleep 30) after cancel = nil, want a killed command")
	}
	if !strings.Contains(err.Error(), "killed") {
		t.Errorf("Run() error = %v, want the command to have been killed", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("the command was not cancelled until %v — the run waits on it", elapsed)
	}
}

// TestExecRunner_CancelKillsTheWholeTree is the failure a cancelled install on the
// test disk measured: the steps start shell scripts (pacstrap, arch-chroot), and the
// grandchild — pacman inside arch-chroot — is what does the work and what holds the
// output pipe. Killing only the script left the step waiting for the very command the
// cancellation was meant to stop, 50 seconds of it in that run.
func TestExecRunner_CancelKillsTheWholeTree(t *testing.T) {
	defer SetCommandRunner(execRunner{})()

	writes := filepath.Join(t.TempDir(), "surviving-child")

	cancel := make(chan struct{})
	defer SetCancellation(cancel)()
	time.AfterFunc(300*time.Millisecond, func() { close(cancel) })

	// A shell that leaves a child behind. The child inherits the shell's stdout —
	// the pipe Output reads — so Output cannot return while any generation lives.
	script := `( i=0; while [ $i -lt 100 ]; do echo x >> '` + writes + `'; sleep 0.1; i=$((i+1)); done ) & wait`

	start := time.Now()
	_, err := Run("sh", "-c", script).Output()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run after cancel = nil, want the command to have been killed")
	}
	if !strings.Contains(err.Error(), "killed") {
		t.Errorf("Run() error = %v, want the command to have been killed", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Output returned only after %v: a child outliving the shell kept the output pipe open", elapsed)
	}

	// Returning is not the point — the work has to stop. A child orphaned behind
	// the killed shell would go on writing, and go on writing to the disk.
	before := fileSize(t, writes)
	time.Sleep(500 * time.Millisecond)
	if after := fileSize(t, writes); after != before {
		t.Errorf("the cancelled command is still working: %d bytes had grown to %d after the kill", before, after)
	}
}

// fileSize reports the size of path, or 0 when it has not been created yet.
func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}

// TestSetCancellation_Restore: clearing the registration is what lets the
// cleanup path run its own umount and kill commands after a cancellation.
func TestSetCancellation_Restore(t *testing.T) {
	defer SetCommandRunner(execRunner{})()

	closed := make(chan struct{})
	close(closed)
	restore := SetCancellation(closed)
	if !cancelled() {
		t.Fatal("a closed channel must read as cancelled")
	}
	restore()

	if cancelled() {
		t.Fatal("clearing the registration must restore normal execution")
	}
	if err := Run("true").Run(); err != nil {
		t.Errorf("Run(true) after restore = %v, want no error", err)
	}
}

// TestSetCancellation_NoRegistrationIsNeverCancelled mirrors the rule every test
// and single-step caller relies on.
func TestSetCancellation_NoRegistrationIsNeverCancelled(t *testing.T) {
	if cancelled() {
		t.Error("with no install loop registered, nothing is cancelled")
	}
}
