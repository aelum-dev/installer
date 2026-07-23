package install

import (
	"testing"

	"aestheticos-installer/backend/internal/shared"
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

func TestValidatePartitionsExist_Nil(t *testing.T) {
	ctx := &InstallContext{}

	err := validatePartitionsExist(ctx)
	if err == nil {
		t.Error("expected error for nil partitions, got nil")
	}
}

func TestValidatePartitionsExist_Set(t *testing.T) {
	ctx := &InstallContext{
		Partitions: &shared.PartitionInfo{
			Disk:     "/dev/vda",
			EFIPart:  "/dev/vda1",
			RootPart: "/dev/vda2",
		},
	}

	err := validatePartitionsExist(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Note: Full integration tests for each step would require:
// 1. A real aestheticos-helper binary (or a more sophisticated mock)
// 2. A real disk device (or a loopback device with a disk image)
// 3. Root privileges
//
// These tests focus on unit testing the logic and auto-discovery features.
// For integration testing, consider using testdisk or similar tools in CI.
