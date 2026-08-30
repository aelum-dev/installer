package verify

import (
	"testing"

	"aelum-installer/backend/shared"
)

// TestRun_SystemNotMounted verifies that Run fails cleanly when the installed
// system is not mounted (the normal state during unit tests).
func TestRun_SystemNotMounted(t *testing.T) {
	ctx := &shared.InstallContext{
		DiskID:   "/dev/vda",
		Username: "testuser",
		Hostname: "testhost",
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid",
			EFIUUID:  "efi-uuid",
		},
	}

	if err := Run(ctx); err == nil {
		t.Error("expected error when /mnt is not mounted, got nil")
	}
}

// TestRequiredPackagesNonEmpty guards against accidentally clearing the list
// of packages that must be present in a bootable system.
func TestExpectedPackagesNonEmpty(t *testing.T) {
	if len(expectedPackages) == 0 {
		t.Error("expected a non-empty expectedPackages list")
	}
}
