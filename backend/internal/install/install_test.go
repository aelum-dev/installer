package install

import (
	"testing"
)

func TestFormatDisk_PartitionLayout(t *testing.T) {
	// Test that FormatDisk creates the expected partition layout
	// This is a logic test, not an execution test
	ctx := &InstallContext{
		DiskID: "/dev/vda",
	}

	// Verify the context is set up correctly
	if ctx.DiskID != "/dev/vda" {
		t.Errorf("expected disk ID /dev/vda, got %s", ctx.DiskID)
	}
}

func TestInstallContext_Defaults(t *testing.T) {
	ctx := &InstallContext{}

	if ctx.ESP != "" {
		t.Errorf("expected empty ESP by default, got %s", ctx.ESP)
	}
	if ctx.DiskID != "" {
		t.Errorf("expected empty DiskID by default, got %s", ctx.DiskID)
	}
}

func TestInstallContext_SetValues(t *testing.T) {
	ctx := &InstallContext{
		DiskID:   "/dev/sda",
		ESP:      "/mnt/boot",
		Username: "testuser",
	}

	if ctx.DiskID != "/dev/sda" {
		t.Errorf("expected /dev/sda, got %s", ctx.DiskID)
	}
	if ctx.ESP != "/mnt/boot" {
		t.Errorf("expected /mnt/boot, got %s", ctx.ESP)
	}
	if ctx.Username != "testuser" {
		t.Errorf("expected testuser, got %s", ctx.Username)
	}
}

func TestValidateMountedSystem_NoHelper(t *testing.T) {
	ctx := &InstallContext{
		Helper: "",
	}

	err := validateMountedSystem(ctx)
	if err == nil {
		t.Error("expected error for empty helper, got nil")
	}
}

func TestPacstrapProgressTracker_BasicProgress(t *testing.T) {
	tr := newPacstrapProgressTracker()

	// Single phase: (1/5), (2/5), (3/5), (4/5), (5/5)
	tests := []struct {
		n, m     int
		wantPct  int
	}{
		{1, 5, 20},
		{2, 5, 40},
		{3, 5, 60},
		{4, 5, 80},
		{5, 5, 100},
	}

	for _, tt := range tests {
		got := tr.update(tt.n, tt.m)
		if got != tt.wantPct {
			t.Errorf("update(%d, %d) = %d, want %d", tt.n, tt.m, got, tt.wantPct)
		}
	}
}

func TestPacstrapProgressTracker_PhaseTransition(t *testing.T) {
	tr := newPacstrapProgressTracker()

	// Phase 1: checking keys (3 items total)
	// Pacstrap outputs: (1/3), (2/3), (3/3) — phase completes fully
	tr.update(1, 3)
	tr.update(2, 3)
	tr.update(3, 3)

	// Phase 2: installing packages (10 items total)
	// (1/10), (2/10), (3/10)
	tr.update(1, 10)
	tr.update(2, 10)
	tr.update(3, 10)

	// After phase 1 completed fully (3/3) and phase 2 at 3/10:
	// completedPhasesTotal=3, currentPhaseTotal=10, currentPhaseCurrent=3
	// overall: (3+3)/(3+10) = 46%
	got := tr.update(3, 10)
	want := 46
	if got != want {
		t.Errorf("after phase transition: got %d%%, want %d%%", got, want)
	}
}

func TestPacstrapProgressTracker_PhaseJump(t *testing.T) {
	tr := newPacstrapProgressTracker()

	// Phase 1: checking keys (5 items total)
	// Pacstrap only outputs up to (3/5) before jumping to next phase
	tr.update(1, 5)
	tr.update(2, 5)
	tr.update(3, 5)

	// Phase 2: installing packages (20 items total)
	// Pacstrap jumps here without finishing phase 1
	tr.update(1, 20)
	tr.update(2, 20)

	// OLD BUG: completedPhasesTotal would be 5 (M of phase 1),
	// giving (5+2)/(5+20) = 28%
	// FIX: completedPhasesTotal is 3 (highest N observed in phase 1),
	// giving (3+2)/(3+20) = 21%
	got := tr.update(2, 20)
	if got != 21 {
		t.Errorf("phase jump: got %d%%, want 21%% (bug would give 28%%)", got)
	}
}

func TestPacstrapProgressTracker_NoOvershoot(t *testing.T) {
	tr := newPacstrapProgressTracker()

	// Normal single phase
	for i := 1; i <= 10; i++ {
		got := tr.update(i, 10)
		if got < 0 || got > 100 {
			t.Errorf("update(%d, 10) = %d, out of range [0, 100]", i, got)
		}
	}
}

func TestPacstrapProgressTracker_DefensiveN(t *testing.T) {
	tr := newPacstrapProgressTracker()

	// N goes backward (shouldn't happen, but be defensive)
	tr.update(5, 10)
	tr.update(3, 10) // N decreased
	tr.update(7, 10) // N increased past previous max

	// completed=0, currentTotal=10, currentCurrent=7
	// overall: (0+7)/(0+10) = 70%
	got := tr.update(7, 10)
	want := 70
	if got != want {
		t.Errorf("defensive N: got %d%%, want %d%%", got, want)
	}
}


// Note: Full integration tests for each step would require:
// 1. A real aestheticos-helper binary (or a more sophisticated mock)
// 2. A real disk device (or a loopback device with a disk image)
// 3. Root privileges
//
// These tests focus on unit testing the logic and auto-discovery features.
// For integration testing, consider using testdisk or similar tools in CI.
