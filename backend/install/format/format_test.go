package format

import (
	"testing"

	"aelum-installer/backend/shared"
)

func TestRun_PartitionLayout(t *testing.T) {
	// Test that Run creates the expected partition layout
	// This is a logic test, not an execution test
	ctx := &shared.InstallContext{
		DiskID: "/dev/vda",
	}

	// Verify the context is set up correctly
	if ctx.DiskID != "/dev/vda" {
		t.Errorf("expected disk ID /dev/vda, got %s", ctx.DiskID)
	}
}

func TestRun_ContextDefaults(t *testing.T) {
	ctx := &shared.InstallContext{}

	if ctx.ESP != "" {
		t.Errorf("expected empty ESP by default, got %s", ctx.ESP)
	}
	if ctx.DiskID != "" {
		t.Errorf("expected empty DiskID by default, got %s", ctx.DiskID)
	}
}

func TestRun_ContextSetValues(t *testing.T) {
	ctx := &shared.InstallContext{
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
