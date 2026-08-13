package install

import (
	"testing"

	"aestheticos-installer/backend/shared"
)

func TestInstaller_StepNames(t *testing.T) {
	inst := &Installer{}
	steps := inst.installationSteps()

	expectedSteps := []string{
		"format_disk",
		"mount_filesystem",
		"install_base",
		"install_desktop",
		"setup_users",
		"configure_system",
		"clear_cache",
	}

	if len(steps) != len(expectedSteps) {
		t.Fatalf("expected %d steps, got %d", len(expectedSteps), len(steps))
	}

	for i, s := range steps {
		if s.Name() != expectedSteps[i] {
			t.Errorf("step %d: expected name %q, got %q", i, expectedSteps[i], s.Name())
		}
		if s.Label() == "" {
			t.Errorf("step %d (%s): label should not be empty", i, s.Name())
		}
	}
}

func TestInstaller_RunStep_UnknownStep(t *testing.T) {
	inst := &Installer{
		diskID: "/dev/vda",
	}

	_, _, err := inst.RunStep("nonexistent_step")
	if err == nil {
		t.Error("expected error for unknown step, got nil")
	}
}

func TestInstaller_Context(t *testing.T) {
	inst := &Installer{
		diskID:   "/dev/sda",
		username: "testuser",
		password: []byte("testpass"),
		region:   shared.GetRegion("en_US"),
		esp:      "/mnt/boot",
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
		"format_disk":      false,
		"mount_filesystem": false,
		"install_base":     false,
		"install_desktop":  false,
		"setup_users":      false,
		"configure_system": false,
		"clear_cache":      false,
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
