package install

import (
	"testing"

	"aestheticos-installer/backend/shared"
)

func TestDefaultPlan_StepCount(t *testing.T) {
	plan := DefaultPlan()
	if len(plan.Steps) != 6 {
		t.Fatalf("expected 6 steps, got %d", len(plan.Steps))
	}
	if len(plan.Weights) != 6 {
		t.Fatalf("expected 6 weights, got %d", len(plan.Weights))
	}
}

func TestDefaultPlan_WeightSum(t *testing.T) {
	plan := DefaultPlan()
	total := 0
	for _, w := range plan.Weights {
		total += w
	}
	if total != 100 {
		t.Fatalf("expected total weight 100, got %d", total)
	}
}

func TestDefaultPlan_StepOrder(t *testing.T) {
	plan := DefaultPlan()
	expectedNames := []string{
		"format_disk",
		"mount_filesystem",
		"install_base",
		"install_desktop",
		"setup_users",
		"configure_system",
	}
	for i, step := range plan.Steps {
		if step.Name() != expectedNames[i] {
			t.Errorf("step %d: expected %q, got %q", i, expectedNames[i], step.Name())
		}
	}
}

func TestDefaultPlan_WeightValues(t *testing.T) {
	plan := DefaultPlan()
	expected := map[int]int{
		0: shared.WeightFormatDisk,
		1: shared.WeightMountFilesystem,
		2: shared.WeightInstallBase,
		3: shared.WeightInstallDesktop,
		4: shared.WeightSetupUsers,
		5: shared.WeightConfigureSystem,
	}
	for i, want := range expected {
		if plan.Weights[i] != want {
			t.Errorf("weight[%d]: expected %d, got %d", i, want, plan.Weights[i])
		}
	}
}

func TestPlan_CumulativePercentages(t *testing.T) {
	plan := DefaultPlan()
	pcts := plan.cumulativePercentages()

	if len(pcts) != 7 {
		t.Fatalf("expected 7 cumulative percentages (n+1), got %d", len(pcts))
	}
	if pcts[0] != 0 {
		t.Errorf("expected pcts[0]=0, got %d", pcts[0])
	}
	if pcts[6] != 100 {
		t.Errorf("expected pcts[6]=100, got %d", pcts[6])
	}

	// Verify monotonic increase
	for i := 1; i < len(pcts); i++ {
		if pcts[i] < pcts[i-1] {
			t.Errorf("cumulative percentages not monotonically increasing at index %d: %d < %d", i, pcts[i], pcts[i-1])
		}
	}
}

func TestStepWeights(t *testing.T) {
	weights := stepWeights()
	if len(weights) != 6 {
		t.Fatalf("expected 6 weights, got %d", len(weights))
	}
	for i, w := range weights {
		if w <= 0 {
			t.Errorf("weight[%d] should be positive, got %d", i, w)
		}
	}
}
