package progress

import (
	"testing"
)

func TestNewPacstrapProgressTracker(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	if tr.TotalPackages() != 0 {
		t.Errorf("expected 0 total packages initially, got %d", tr.TotalPackages())
	}
}

func TestPacstrapProgressTracker_SetTotalPackages(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	tr.SetTotalPackages(42)
	if tr.TotalPackages() != 42 {
		t.Errorf("expected 42, got %d", tr.TotalPackages())
	}
}

func TestPacstrapProgressTracker_ProcessLine_Unclassified(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	// A line that matches nothing should return 0.
	got := tr.ProcessLine("some random output")
	if got != 0 {
		t.Errorf("expected 0%% for unclassified line, got %d%%", got)
	}
}

func TestPacstrapProgressTracker_EmptyTracker(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	// With no packages set, CalculatePercent should return 0 (avoid div-by-zero).
	got := tr.CalculatePercent()
	if got != 0 {
		t.Errorf("expected 0%% with empty tracker, got %d%%", got)
	}
}

func TestPacstrapProgressTracker_DownloadThenInstall(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	tr.SetTotalPackages(2)

	// Download both packages.
	tr.ProcessLine("  pkg1 downloading...")
	tr.ProcessLine("  pkg2 downloading...")

	// downloaded=2, totalUnits=2*2=4, currentUnits=2 → 50%
	got := tr.CalculatePercent()
	if got != 50 {
		t.Errorf("after 2 downloads: got %d%%, want 50%%", got)
	}

	// Install both packages.
	tr.ProcessLine("installing pkg1...")
	tr.ProcessLine("installing pkg2...")

	// downloaded=2, installed=2, totalUnits=4, currentUnits=4 → 100%
	got = tr.CalculatePercent()
	if got != 100 {
		t.Errorf("after 2 installs: got %d%%, want 100%%", got)
	}
}

func TestPacstrapProgressTracker_VerifyLines(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	tr.SetTotalPackages(1)

	tr.ProcessLine("checking keyring...")
	tr.ProcessLine("checking package integrity...")
	tr.ProcessLine("loading package files...")
	tr.ProcessLine("checking for file conflicts...")
	tr.ProcessLine(":: Processing package changes...")

	// verified=5, totalUnits=1*2+5=7, currentUnits=5 → 71%
	got := tr.CalculatePercent()
	if got != 71 {
		t.Errorf("after 5 verify lines: got %d%%, want 71%%", got)
	}
}

func TestPacstrapProgressTracker_HookPhaseTransition(t *testing.T) {
	tr := NewPacstrapProgressTracker()

	// Phase 1: 3 items, fully completed.
	tr.UpdateHook(1, 3)
	tr.UpdateHook(2, 3)
	tr.UpdateHook(3, 3)

	// Phase 2: 5 items, 2 completed.
	got := tr.UpdateHook(2, 5)

	// completedPhases=3, currentTotal=5, currentCurrent=2
	// totalUnits=3+5=8, currentUnits=3+2=5 → 62%
	want := 62
	if got != want {
		t.Errorf("phase transition: got %d%%, want %d%%", got, want)
	}
}

func TestPacstrapProgressTracker_HookPhaseIncomplete(t *testing.T) {
	tr := NewPacstrapProgressTracker()

	// Phase 1: 5 items, only reached 3 before jumping.
	tr.UpdateHook(1, 5)
	tr.UpdateHook(2, 5)
	tr.UpdateHook(3, 5)

	// Phase 2: 10 items, 4 completed.
	got := tr.UpdateHook(4, 10)

	// completedPhases=3 (highest N observed), currentTotal=10, currentCurrent=4
	// totalUnits=3+10=13, currentUnits=3+4=7 → 53%
	want := 53
	if got != want {
		t.Errorf("incomplete phase jump: got %d%%, want %d%%", got, want)
	}
}

func TestPacstrapProgressTracker_AdvancePercentBounds(t *testing.T) {
	tr := NewPacstrapProgressTracker()

	// Simulate going past 100% — should clamp.
	tr.SetTotalPackages(1)
	tr.ProcessLine("  pkg1 downloading...")
	tr.ProcessLine("installing pkg1...")
	tr.ProcessLine("( 1/1) hook")
	tr.ProcessLine("( 1/1) hook") // duplicate hook should not increase

	got := tr.CalculatePercent()
	if got > 100 {
		t.Errorf("progress exceeded 100%%: got %d%%", got)
	}
	if got < 0 {
		t.Errorf("progress went negative: got %d%%", got)
	}
}

func TestStepProgressTracker_Advance(t *testing.T) {
	tr := NewStepProgressTracker(5)

	expected := []int{20, 40, 60, 80, 100, 100, 100}
	for i, want := range expected {
		got := tr.Advance()
		if got != want {
			t.Errorf("advance(%d): got %d%%, want %d%%", i+1, got, want)
		}
	}
}

func TestStepProgressTracker_ZeroTicks(t *testing.T) {
	tr := NewStepProgressTracker(0)
	// Zero ticks should return 100 immediately.
	got := tr.Advance()
	if got != 100 {
		t.Errorf("expected 100%% with 0 ticks, got %d%%", got)
	}
}

func TestStepProgressTracker_NegativeTicks(t *testing.T) {
	tr := NewStepProgressTracker(-1)
	got := tr.Advance()
	if got != 100 {
		t.Errorf("expected 100%% with negative ticks, got %d%%", got)
	}
}
