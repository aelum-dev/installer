package core

import (
	"testing"

	"aestheticos-installer/backend/internal/shared"
)

func TestInstaller_StepNames(t *testing.T) {
	inst := &Installer{}
	steps := inst.installationSteps()

	expectedSteps := []string{
		"format_disk",
		"mount_filesystem",
		"install",
		"install_desktop",
		"create_swap",
		"generate_fstab",
		"configure_bootloader",
		"setup_users",
		"configure_system",
		"configure_greetd",
		"enable_services",
	}

	if len(steps) != len(expectedSteps) {
		t.Fatalf("expected %d steps, got %d", len(expectedSteps), len(steps))
	}

	for i, s := range steps {
		if s.name != expectedSteps[i] {
			t.Errorf("step %d: expected name %q, got %q", i, expectedSteps[i], s.name)
		}
		if s.label == "" {
			t.Errorf("step %d (%s): label should not be empty", i, s.name)
		}
	}
}

func TestInstaller_RunStep_UnknownStep(t *testing.T) {
	inst := &Installer{
		diskID:   "/dev/vda",
		helperPath: "/fake/helper",
	}

	_, _, err := inst.RunStep("nonexistent_step")
	if err == nil {
		t.Error("expected error for unknown step, got nil")
	}
}

func TestInstaller_Context(t *testing.T) {
	inst := &Installer{
		diskID:     "/dev/sda",
		helperPath: "/usr/bin/helper",
		username:   "testuser",
		password:   "testpass",
		region:     shared.GetRegion("en_US"),
		esp:        "/mnt/boot",
	}

	ctx := inst.context()
	if ctx.DiskID != "/dev/sda" {
		t.Errorf("expected DiskID /dev/sda, got %s", ctx.DiskID)
	}
	if ctx.ESP != "/mnt/boot" {
		t.Errorf("expected ESP /mnt/boot, got %s", ctx.ESP)
	}
	if ctx.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", ctx.Username)
	}
}

func TestValidStepNames(t *testing.T) {
	names := validStepNames()
	if len(names) == 0 {
		t.Error("expected non-empty step names")
	}

	// Check that all expected names are present
	expected := map[string]bool{
		"format_disk":            false,
		"mount_filesystem":       false,
		"install":                false,
		"install_desktop":        false,
		"create_swap":            false,
		"generate_fstab":         false,
		"configure_bootloader":   false,
		"setup_users":            false,
		"configure_system":       false,
		"configure_greetd":       false,
		"enable_services":        false,
	}

	for _, name := range names {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected step name %q not found", name)
		}
	}
}
