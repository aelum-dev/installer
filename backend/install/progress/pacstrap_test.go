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

	// Raw ratio would be: completedPhases=3, currentTotal=5, currentCurrent=2
	// totalUnits=3+5=8, currentUnits=3+2=5 → 62%. But phase 1 already reached
	// 100%, so the monotonic clamp keeps the reported progress at 100% (the
	// bar must never move backwards when a new hook phase enlarges the total).
	want := 100
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

	// Raw ratio would be: completedPhases=3 (highest N observed),
	// currentTotal=10, currentCurrent=4 → totalUnits=13, currentUnits=7 → 53%.
	// Phase 1 had already reported 60%, so the monotonic clamp holds at 60%.
	want := 60
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

// TestPacstrapProgressTracker_MonotonicAcrossHooks ensures the reported
// percentage never moves backwards when pacstrap transitions from the install
// phase into the post-transaction hook phase — the denominator jumps by the
// new phase's total (M) before the numerator can keep up, which used to make
// the progress bar visibly jump backwards (e.g. 93% -> 90% -> 93%).
func TestPacstrapProgressTracker_MonotonicAcrossHooks(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	tr.SetTotalPackages(150)

	// Verify phase.
	for i := 0; i < 15; i++ {
		tr.ProcessLine("checking step...")
	}

	// Download + install all packages (reaches ~100% of the install work).
	for i := 0; i < 150; i++ {
		tr.ProcessLine("pkg downloading...")
		tr.ProcessLine("installing pkg...")
	}

	last := tr.CalculatePercent()
	if last < 99 {
		t.Fatalf("expected install phase to reach ~100%%, got %d%%", last)
	}

	// Enter the post-transaction hook phase with a large M. The denominator
	// jumps by M before the numerator can follow, which used to cause a
	// backwards jump. Progress must stay >= what we had before the hooks.
	for n := 1; n <= 30; n++ {
		got := tr.UpdateHook(n, 30)
		if got < last {
			t.Errorf("progress moved backwards at hook (%d/30): got %d%%, was %d%%", n, got, last)
		}
		last = got
	}

	if last != 100 {
		t.Errorf("expected final progress to reach 100%%, got %d%%", last)
	}
}

// TestPacstrapProgressTracker_MonotonicMultiPhaseHooks checks monotonicity
// across several hook phases with differing totals.
func TestPacstrapProgressTracker_MonotonicMultiPhaseHooks(t *testing.T) {
	tr := NewPacstrapProgressTracker()
	tr.SetTotalPackages(100)

	for i := 0; i < 100; i++ {
		tr.ProcessLine("pkg downloading...")
		tr.ProcessLine("installing pkg...")
	}
	last := tr.CalculatePercent()

	// Multiple hook phases, each with a different (and larger) total.
	phases := [][2]int{{1, 5}, {5, 5}, {1, 20}, {20, 20}, {1, 40}, {40, 40}}
	for _, p := range phases {
		got := tr.UpdateHook(p[0], p[1])
		if got < last {
			t.Errorf("progress moved backwards: got %d%%, was %d%%", got, last)
		}
		last = got
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
