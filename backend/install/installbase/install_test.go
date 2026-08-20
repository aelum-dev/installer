package installbase

import (
	"testing"

	"aestheticos-installer/backend/install/progress"
)

func TestBasePackages_NotEmpty(t *testing.T) {
	if len(basePackages) == 0 {
		t.Error("expected non-empty basePackages")
	}
}

func TestBasePackages_ContainsEssential(t *testing.T) {
	expected := []string{"base", "linux-lts", "linux-firmware"}
	for _, pkg := range expected {
		found := false
		for _, p := range basePackages {
			if p == pkg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected basePackages to contain %q", pkg)
		}
	}
}

func TestPacstrapProgressTracker_BasicProgress(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()

	// Single phase: (1/5), (2/5), (3/5), (4/5), (5/5)
	tests := []struct {
		n, m    int
		wantPct int
	}{
		{1, 5, 20},
		{2, 5, 40},
		{3, 5, 60},
		{4, 5, 80},
		{5, 5, 100},
	}

	for _, tt := range tests {
		got := tr.UpdateHook(tt.n, tt.m)
		if got != tt.wantPct {
			t.Errorf("update(%d, %d) = %d, want %d", tt.n, tt.m, got, tt.wantPct)
		}
	}
}

func TestPacstrapProgressTracker_PhaseTransition(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()

	// Phase 1: checking keys (3 items total)
	// Pacstrap outputs: (1/3), (2/3), (3/3) — phase completes fully
	tr.UpdateHook(1, 3)
	tr.UpdateHook(2, 3)
	tr.UpdateHook(3, 3)

	// Phase 2: installing packages (10 items total)
	// (1/10), (2/10), (3/10)
	tr.UpdateHook(1, 10)
	tr.UpdateHook(2, 10)
	tr.UpdateHook(3, 10)

	// After phase 1 completed fully (3/3) and phase 2 at 3/10:
	// Raw ratio: completedPhasesTotal=3, currentPhaseTotal=10, currentPhaseCurrent=3
	// overall: (3+3)/(3+10) = 46%. But phase 1 already reported 100%, so the
	// monotonic clamp keeps the displayed progress at 100% (the bar must never
	// move backwards when a new hook phase enlarges the total work).
	got := tr.UpdateHook(3, 10)
	want := 100
	if got != want {
		t.Errorf("after phase transition: got %d%%, want %d%%", got, want)
	}
}

func TestPacstrapProgressTracker_PhaseJump(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()

	// Phase 1: checking keys (5 items total)
	// Pacstrap only outputs up to (3/5) before jumping to next phase
	tr.UpdateHook(1, 5)
	tr.UpdateHook(2, 5)
	tr.UpdateHook(3, 5)

	// Phase 2: installing packages (20 items total)
	// Pacstrap jumps here without finishing phase 1
	tr.UpdateHook(1, 20)
	tr.UpdateHook(2, 20)

	// Raw ratio: completedPhasesTotal is 3 (highest N observed in phase 1),
	// giving (3+2)/(3+20) = 21%. Phase 1 had already reported 60%, so the
	// monotonic clamp keeps the displayed progress at 60% (forward-only bar).
	got := tr.UpdateHook(2, 20)
	if got != 60 {
		t.Errorf("phase jump: got %d%%, want 60%% (raw ratio would be 21%%)", got)
	}
}

func TestPacstrapProgressTracker_NoOvershoot(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()

	// Normal single phase
	for i := 1; i <= 10; i++ {
		got := tr.UpdateHook(i, 10)
		if got < 0 || got > 100 {
			t.Errorf("update(%d, 10) = %d, out of range [0, 100]", i, got)
		}
	}
}

func TestPacstrapProgressTracker_DefensiveN(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()

	// N goes backward (shouldn't happen, but be defensive)
	tr.UpdateHook(5, 10)
	tr.UpdateHook(3, 10) // N decreased
	tr.UpdateHook(7, 10) // N increased past previous max

	// completed=0, currentTotal=10, currentCurrent=7
	// overall: (0+7)/(0+10) = 70%
	got := tr.UpdateHook(7, 10)
	want := 70
	if got != want {
		t.Errorf("defensive N: got %d%%, want %d%%", got, want)
	}
}

func TestPacstrapProgressTracker_PackagesCount(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()

	// "Packages (190)" sets the total.
	got := tr.ProcessLine(":: Packages (190) acl-2.4.0-1 ...")
	if got != 0 {
		t.Errorf("after Packages line: got %d%%, want 0%%", got)
	}
	if tr.TotalPackages() != 190 {
		t.Errorf("totalPackages = %d, want 190", tr.TotalPackages())
	}
}

func TestPacstrapProgressTracker_DownloadPhase(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()
	tr.SetTotalPackages(4) // known from Packages (N)

	// 4 packages downloading.
	tr.ProcessLine("  linux-lts downloading...")
	tr.ProcessLine("  bash downloading...")
	tr.ProcessLine("  sudo downloading...")
	tr.ProcessLine("  vim downloading...")

	// After all 4 downloads: downloaded=4
	// totalUnits = 4*2 (download+install) + 0 (verify) + 0 (hooks) = 8
	// currentUnits = 4
	// pct = 4/8 * 100 = 50
	got := tr.CalculatePercent()
	if got != 50 {
		t.Errorf("after all downloads: got %d%%, want 50%%", got)
	}
}

func TestPacstrapProgressTracker_VerifyPhase(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()
	tr.SetTotalPackages(4)

	// Verification lines.
	tr.ProcessLine("checking keyring...")
	tr.ProcessLine("checking package integrity...")
	tr.ProcessLine("loading package files...")
	tr.ProcessLine("checking for file conflicts...")

	// verified=4, totalUnits = 4*2 (download+install) + 4 (verify) + 0 (hooks) = 12
	// currentUnits = 0 + 4 + 0 + 0 = 4
	// pct = 4/12 * 100 = 33
	got := tr.CalculatePercent()
	if got != 33 {
		t.Errorf("after verification: got %d%%, want 33%%", got)
	}
}

func TestPacstrapProgressTracker_InstallPhase(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()
	tr.SetTotalPackages(3)

	// Simulate full flow: Packages → download 3 → verify 4 → install 2 of 3
	tr.ProcessLine(":: Packages (3) pkg1 pkg2 pkg3")
	tr.ProcessLine("  pkg1 downloading...")
	tr.ProcessLine("  pkg2 downloading...")
	tr.ProcessLine("  pkg3 downloading...")
	tr.ProcessLine("checking keyring...")
	tr.ProcessLine("checking package integrity...")
	tr.ProcessLine("loading package files...")
	tr.ProcessLine("checking for file conflicts...")
	tr.ProcessLine("installing pkg1...")
	tr.ProcessLine("installing pkg2...")

	// downloaded=3, verified=5 (4 check + 1 processing), installed=2
	// totalUnits = 3*2 (download+install) + 5 (verify) + 0 (hooks) = 11
	// currentUnits = 3 + 5 + 2 + 0 = 10
	// pct = 10/11 * 100 = 90
	got := tr.CalculatePercent()
	if got != 90 {
		t.Errorf("after 2/3 installs: got %d%%, want 90%%", got)
	}
}

func TestPacstrapProgressTracker_FullFlow(t *testing.T) {
	tr := progress.NewPacstrapProgressTracker()

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
		tr.ProcessLine(line)
	}

	// downloaded=3, verified=5 (4 check + 1 processing), installed=3
	// hook: completed=0, currentTotal=2, currentCurrent=2
	// totalUnits = 3*2 (download+install) + 5 (verify) + 2 (hooks) = 13
	// currentUnits = 3 + 5 + 3 + 2 = 13
	// pct = 13/13 * 100 = 100
	got := tr.CalculatePercent()
	if got != 100 {
		t.Errorf("full flow complete: got %d%%, want 100%%", got)
	}
}

func TestPacstrapProgressTracker_NoRegression(t *testing.T) {
	// Ensure ProcessLine with (N/M) still works like UpdateHook().
	tr := progress.NewPacstrapProgressTracker()

	// Pure hook-only scenario (no packages/download/install lines).
	tr.ProcessLine("( 1/5) hook 1")
	tr.ProcessLine("( 3/5) hook 2")
	tr.ProcessLine("( 5/5) hook 3")

	got := tr.CalculatePercent()
	if got != 100 {
		t.Errorf("pure hooks 5/5: got %d%%, want 100%%", got)
	}
}
