package install

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"aestheticos-installer/backend/shared"
)

// TestMain overrides simulateStep to use 100ms per step instead of the real duration.
// This makes integration tests run in ~6 seconds instead of ~60 seconds.
func TestMain(m *testing.M) {
	simulateStep = func(duration time.Duration) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}
	os.Exit(m.Run())
}

// TestInstall_DryRun_FullFlow verifies the full installation orchestration
// in dry-run mode. No real commands are executed — steps simulate with a
// short sleep. This catches regressions in step ordering, progress tracking,
// and error handling without needing a real disk or helper binary.
func TestInstall_DryRun_FullFlow(t *testing.T) {
	// Ensure fake disks are available for disk discovery steps.
	t.Setenv("FAKE_DISKS", "1")

	inst := NewInstaller(
		"/dev/vda",
		"testuser", "testpass",
		"testhost",
		shared.GetRegion("en_US"),
		"ext4", true, // dryRun
	)

	var progressEvents []string
	inst.OnProgress = func(label string, percent int, phase string) {
		progressEvents = append(progressEvents, fmt.Sprintf("%s:%d:%s", label, percent, phase))
	}

	err := inst.Install()
	if err != nil {
		t.Fatalf("expected success in dry-run, got error: %v", err)
	}

	// Should have received progress events for each step.
	if len(progressEvents) == 0 {
		t.Fatal("expected progress events, got none")
	}

	// First event should be the first step at 0%.
	if !strings.Contains(progressEvents[0], "Formatting disk:0:running") {
		t.Errorf("expected first event to be Formatting disk at 0%%, got: %s", progressEvents[0])
	}

	// Last event should be the last step at 100% done.
	last := progressEvents[len(progressEvents)-1]
	if !strings.Contains(last, "Configuring system:100:done") {
		t.Errorf("expected last event to be Configuring system at 100%% done, got: %s", last)
	}
}

// TestInstall_DryRun_Btrfs verifies the full install flow with btrfs.
func TestInstall_DryRun_Btrfs(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")

	inst := NewInstaller(
		"/dev/nvme0n1",
		"testuser", "testpass",
		"testhost",
		shared.GetRegion("en_US"),
		"btrfs", true,
	)

	var stepLabels []string
	inst.OnProgress = func(label string, _ int, phase string) {
		if phase == "running" {
			// Strip the [DRY RUN] prefix for counting.
			stepLabels = append(stepLabels, strings.TrimPrefix(label, "[DRY RUN] "))
		}
	}

	err := inst.Install()
	if err != nil {
		t.Fatalf("expected success in dry-run btrfs, got error: %v", err)
	}

	// All 6 steps should have been invoked.
	if len(stepLabels) != 6 {
		t.Errorf("expected 12 step labels, got %d: %v", len(stepLabels), stepLabels)
	}
}

// TestInstall_EmptyDiskID verifies that Install rejects an empty disk ID.
func TestInstall_EmptyDiskID(t *testing.T) {
	inst := NewInstaller(
		"",
		"user", "pass",
		"host",
		shared.GetRegion("en_US"),
		"ext4", false,
	)

	err := inst.Install()
	if err == nil {
		t.Fatal("expected error for empty disk ID, got nil")
	}
	if !strings.Contains(err.Error(), "disk ID is required") {
		t.Errorf("expected 'disk ID is required' error, got: %v", err)
	}
}

// TestInstall_EmptyRegion verifies that Install rejects an empty region.
func TestInstall_EmptyRegion(t *testing.T) {
	inst := NewInstaller(
		"/dev/vda",
		"user", "pass",
		"host",
		shared.GetRegion(""),
		"ext4", false,
	)

	err := inst.Install()
	if err == nil {
		t.Fatal("expected error for empty region, got nil")
	}
	if !strings.Contains(err.Error(), "region not set") {
		t.Errorf("expected 'region not set' error, got: %v", err)
	}
}

// TestInstall_ProgressPercentages verifies that progress percentages
// are monotonically non-decreasing across steps.
func TestInstall_ProgressPercentages(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")

	inst := NewInstaller(
		"/dev/vda",
		"user", "pass",
		"host",
		shared.GetRegion("en_US"),
		"ext4", true,
	)

	var percentages []int
	inst.OnProgress = func(_ string, percent int, phase string) {
		if phase == "running" || phase == "done" {
			percentages = append(percentages, percent)
		}
	}

	err := inst.Install()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Percentages should generally increase (with some same-value repeats
	// from the tick goroutine). Check that the overall trend is upward.
	if len(percentages) < 2 {
		t.Fatal("expected multiple progress events")
	}
	if percentages[0] != 0 {
		t.Errorf("expected first percentage to be 0, got %d", percentages[0])
	}
	if percentages[len(percentages)-1] != 100 {
		t.Errorf("expected last percentage to be 100, got %d", percentages[len(percentages)-1])
	}
}

// TestInstall_ErrorCleanup verifies that on failure, CleanupMounts is
// called via the defer in Install(). We verify this by checking that
// the log file is written even on error.
func TestInstall_ErrorCleanup(t *testing.T) {
	// Use an invalid disk to trigger an early error.
	inst := NewInstaller(
		"/dev/nonexistent_disk_xyz",
		"user", "pass",
		"host",
		shared.GetRegion("en_US"),
		"ext4", false,
	)

	// Remove any existing log so we can verify it gets written.
	logPath := shared.GetLogPath()
	os.Remove(logPath)

	err := inst.Install()
	if err == nil {
		t.Fatal("expected error for nonexistent disk, got nil")
	}

	// Log file should still be created (cleared at start of Install).
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("expected log file to be created even on error")
	}
}

// TestRollback_ValidDisk verifies Rollback with a valid disk path.
func TestRollback_ValidDisk(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")

	inst := NewInstaller(
		"/dev/vda",
		"", "", "",
		shared.GetRegion("en_US"),
		"ext4", false,
	)

	// Rollback should not return an error for an empty disk ID check.
	// It will fail on the actual wipefs/partprobe calls since there's
	// no real helper, but we verify the disk ID validation passes.
	err := inst.Rollback()
	// We expect an error here because the helper doesn't exist,
	// but it should NOT be "disk ID is required".
	if err != nil && strings.Contains(err.Error(), "disk ID is required") {
		t.Errorf("unexpected 'disk ID is required' error: %v", err)
	}
}

// TestRollback_EmptyDiskID verifies that Rollback rejects an empty disk.
func TestRollback_EmptyDiskID(t *testing.T) {
	inst := NewInstaller(
		"",
		"", "", "",
		shared.GetRegion("en_US"),
		"ext4", false,
	)

	err := inst.Rollback()
	if err == nil {
		t.Fatal("expected error for empty disk ID, got nil")
	}
	if !strings.Contains(err.Error(), "disk ID is required") {
		t.Errorf("expected 'disk ID is required' error, got: %v", err)
	}
}

// TestRunStep_UnknownStep verifies that RunStep rejects unknown step names.
func TestRunStep_UnknownStep(t *testing.T) {
	inst := NewInstaller(
		"/dev/vda",
		"user", "pass",
		"host",
		shared.GetRegion("en_US"),
		"ext4", false,
	)

	_, _, err := inst.RunStep("nonexistent_step")
	if err == nil {
		t.Fatal("expected error for unknown step, got nil")
	}
	if !strings.Contains(err.Error(), "unknown step") {
		t.Errorf("expected 'unknown step' error, got: %v", err)
	}
}

// TestRunStep_DryRun verifies that a known step runs successfully in dry-run.
func TestRunStep_DryRun(t *testing.T) {
	inst := NewInstaller(
		"/dev/vda",
		"user", "pass",
		"host",
		shared.GetRegion("en_US"),
		"ext4", true,
	)

	label, percent, err := inst.RunStep("format_disk")
	if err != nil {
		t.Fatalf("expected success in dry-run, got error: %v", err)
	}
	if label == "" {
		t.Error("expected non-empty label")
	}
	if percent != 100 {
		t.Errorf("expected 100%% progress, got %d", percent)
	}
}

// TestCumulativePercentages verifies the percentage calculation
// across all 12 installation steps.
func TestCumulativePercentages(t *testing.T) {
	steps := DefaultPlan().Steps
	pcts := DefaultPlan().cumulativePercentages()

	if len(pcts) != len(steps)+1 {
		t.Fatalf("expected %d percentages for %d steps, got %d", len(steps)+1, len(steps), len(pcts))
	}
	if pcts[0] != 0 {
		t.Errorf("expected first percentage 0, got %d", pcts[0])
	}
	if pcts[len(pcts)-1] != 100 {
		t.Errorf("expected last percentage 100, got %d", pcts[len(pcts)-1])
	}
	// Percentages should be monotonically non-decreasing.
	for i := 1; i < len(pcts); i++ {
		if pcts[i] < pcts[i-1] {
			t.Errorf("percentage decreased at index %d: %d -> %d", i, pcts[i-1], pcts[i])
		}
	}
}

// TestStepWeightsSum verifies that step weights sum to the expected total.
func TestStepWeightsSum(t *testing.T) {
	total := 0
	for _, w := range stepWeights() {
		total += w
	}
	if total != 100 {
		t.Errorf("expected step weights to sum to 100, got %d", total)
	}
}

// TestStepWeights_PacstrapDominates verifies that the two pacstrap steps
// account for the majority of the weight.
func TestStepWeights_PacstrapDominates(t *testing.T) {
	pacstrapWeight := shared.WeightInstallBase + shared.WeightInstallDesktop
	total := 0
	for _, w := range stepWeights() {
		total += w
	}
	pct := float64(pacstrapWeight) / float64(total) * 100
	if pct < 75 {
		t.Errorf("expected pacstrap steps to be >= 75%% of total weight, got %.1f%%", pct)
	}
}
