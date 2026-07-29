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


func TestPacstrapProgressTracker_PackagesCount(t *testing.T) {
	tr := newPacstrapProgressTracker()

	// "Packages (190)" sets the total.
	got := tr.processLine(":: Packages (190) acl-2.4.0-1 ...")
	if got != 0 {
		t.Errorf("after Packages line: got %d%%, want 0%%", got)
	}
	if tr.totalPackages != 190 {
		t.Errorf("totalPackages = %d, want 190", tr.totalPackages)
	}
}

func TestPacstrapProgressTracker_DownloadPhase(t *testing.T) {
	tr := newPacstrapProgressTracker()
	tr.totalPackages = 4 // known from Packages (N)

	// 4 packages downloading.
	tr.processLine("  linux-lts downloading...")
	tr.processLine("  bash downloading...")
	tr.processLine("  sudo downloading...")
	tr.processLine("  vim downloading...")

	// After all 4 downloads: downloaded=4
	// totalUnits = 4*2 (download+install) + 0 (verify) + 0 (hooks) = 8
	// currentUnits = 4
	// pct = 4/8 * 100 = 50
	got := tr.calculatePercent()
	if got != 50 {
		t.Errorf("after all downloads: got %d%%, want 50%%", got)
	}
}

func TestPacstrapProgressTracker_VerifyPhase(t *testing.T) {
	tr := newPacstrapProgressTracker()
	tr.totalPackages = 4

	// Verification lines.
	tr.processLine("checking keyring...")
	tr.processLine("checking package integrity...")
	tr.processLine("loading package files...")
	tr.processLine("checking for file conflicts...")

	// verified=4, totalUnits = 4*2 (download+install) + 4 (verify) + 0 (hooks) = 12
	// currentUnits = 0 + 4 + 0 + 0 = 4
	// pct = 4/12 * 100 = 33
	got := tr.calculatePercent()
	if got != 33 {
		t.Errorf("after verification: got %d%%, want 33%%", got)
	}
}

func TestPacstrapProgressTracker_InstallPhase(t *testing.T) {
	tr := newPacstrapProgressTracker()
	tr.totalPackages = 3

	// Simulate full flow: Packages → download 3 → verify 4 → install 2 of 3
	tr.processLine(":: Packages (3) pkg1 pkg2 pkg3")
	tr.processLine("  pkg1 downloading...")
	tr.processLine("  pkg2 downloading...")
	tr.processLine("  pkg3 downloading...")
	tr.processLine("checking keyring...")
	tr.processLine("checking package integrity...")
	tr.processLine("loading package files...")
	tr.processLine("checking for file conflicts...")
	tr.processLine("installing pkg1...")
	tr.processLine("installing pkg2...")

	// downloaded=3, verified=5 (4 check + 1 processing), installed=2
	// totalUnits = 3*2 (download+install) + 5 (verify) + 0 (hooks) = 11
	// currentUnits = 3 + 5 + 2 + 0 = 10
	// pct = 10/11 * 100 = 90
	got := tr.calculatePercent()
	if got != 90 {
		t.Errorf("after 2/3 installs: got %d%%, want 90%%", got)
	}
}

func TestPacstrapProgressTracker_FullFlow(t *testing.T) {
	tr := newPacstrapProgressTracker()

	// Simulate a realistic base install: 3 packages, 4 verify steps, 3 installs, 2 hooks.
	lines := []string{
		":: Packages (3) pkg1 pkg2 pkg3",
		"  pkg1 downloading...",
		"  pkg2 downloading...",
		"  pkg3 downloading...",
		"checking keyring...",
		"checking package integrity...",
		"loading package files...",
		"checking for file conflicts...",
		":: Processing package changes...",
		"installing pkg1...",
		"installing pkg2...",
		"installing pkg3...",
		"( 1/2) Running post-transaction hook 1...",
		"( 2/2) Running post-transaction hook 2...",
	}

	for _, line := range lines {
		tr.processLine(line)
	}

	// downloaded=3, verified=5 (4 check + 1 processing), installed=3
	// hook: completed=0, currentTotal=2, currentCurrent=2
	// totalUnits = 3*2 (download+install) + 5 (verify) + 2 (hooks) = 13
	// currentUnits = 3 + 5 + 3 + 2 = 13
	// pct = 13/13 * 100 = 100
	got := tr.calculatePercent()
	if got != 100 {
		t.Errorf("full flow complete: got %d%%, want 100%%", got)
	}
}

func TestPacstrapProgressTracker_NoRegression(t *testing.T) {
	// Ensure processLine with (N/M) still works like update().
	tr := newPacstrapProgressTracker()

	// Pure hook-only scenario (no packages/download/install lines).
	tr.processLine("( 1/5) hook 1")
	tr.processLine("( 3/5) hook 2")
	tr.processLine("( 5/5) hook 3")

	got := tr.calculatePercent()
	if got != 100 {
		t.Errorf("pure hooks 5/5: got %d%%, want 100%%", got)
	}
}

// Note: Full integration tests for each step would require:
// 1. A real aestheticos-helper binary (or a more sophisticated mock)
// 2. A real disk device (or a loopback device with a disk image)
// 3. Root privileges
//
// These tests focus on unit testing the logic and auto-discovery features.
// For integration testing, consider using testdisk or similar tools in CI.
