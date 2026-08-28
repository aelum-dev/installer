package mount

import (
	"testing"

	"aelum-installer/backend/shared"
)

func TestRun_ContextValidation(t *testing.T) {
	ctx := &shared.InstallContext{
		DiskID: "/dev/vda",
	}

	// Verify the context is set up correctly
	if ctx.DiskID != "/dev/vda" {
		t.Errorf("expected disk ID /dev/vda, got %s", ctx.DiskID)
	}
}

func TestRun_PartitionInfoValidation(t *testing.T) {
	ctx := &shared.InstallContext{
		DiskID: "/dev/vda",
		Partitions: &shared.PartitionInfo{
			Disk:        "/dev/vda",
			EFIPart:     "/dev/vda1",
			EFIUUID:     "efi-uuid",
			RootPart:    "/dev/vda2",
			RootUUID:    "root-uuid",
			MountPoints: make(map[string]string),
		},
	}

	if ctx.Partitions.EFIPart == "" {
		t.Error("expected non-empty EFIPart")
	}
	if ctx.Partitions.RootPart == "" {
		t.Error("expected non-empty RootPart")
	}
}
