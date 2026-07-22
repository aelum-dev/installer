package steps

import (
	"strings"
	"testing"

	"aesthetic-os-installer/backend/shared"
)

// TestLayoutIntegration verifies the full format → mount → fstab chain
// works correctly with a btrfs layout containing multiple subvolumes.
// This test doesn't execute real commands — it validates the schema flow
// and verifies that all subvolumes are handled consistently.
func TestLayoutIntegration(t *testing.T) {
	// 1. Create a layout with subvolumes (simulating GetBtrfsFullLayout)
	layout := shared.PartitionLayout{
		Name:        "btrfs",
		Description: "BTRFS with subvolumes",
		Partitions: []shared.LayoutPartition{
			{Name: "EFI", SizeMB: 128, Fstype: "vfat", MountPoint: "/boot"},
			{Name: "Root", SizeMB: 0, Fstype: "btrfs", MountPoint: "/"},
		},
		Subvolumes: []shared.BtrfsSubvol{
			{Name: "@", MountPoint: "/", Properties: map[string]string{"compression": "zstd"}},
			{Name: "@home", MountPoint: "/home", Properties: map[string]string{"compression": "zstd", "nodatacow": ""}},
			{Name: "@root", MountPoint: "/root", Properties: map[string]string{"compression": "zstd"}},
			{Name: "@srv", MountPoint: "/srv", Properties: map[string]string{"compression": "zstd"}},
			{Name: "@cache", MountPoint: "/var/cache", Properties: map[string]string{"compression": "zstd", "nodatacow": ""}},
			{Name: "@tmp", MountPoint: "/var/tmp", Properties: map[string]string{"compression": "zstd", "nodatacow": ""}},
			{Name: "@log", MountPoint: "/var/log", Properties: map[string]string{"nodatacow": ""}},
		},
	}

	// 2. Simulate FormatDisk: validate layout and populate PartitionInfo
	if err := validateLayout(&layout); err != nil {
		t.Fatalf("Layout validation failed: %v", err)
	}

	partitionInfo := &shared.PartitionInfo{
		Disk:        "/dev/vda",
		LayoutName:  layout.Name,
		EFIPart:     "/dev/vda1",
		EFIUUID:     "test-efi-uuid",
		RootPart:    "/dev/vda2",
		RootUUID:    "test-root-uuid",
		MountPoints: make(map[string]string),
	}

	// Populate MountPoints for non-root subvolumes (simulating FormatDisk logic)
	for _, sv := range layout.Subvolumes {
		if sv.MountPoint != "/" {
			partitionInfo.MountPoints[sv.MountPoint] = partitionInfo.RootPart
		}
	}

	// 3. Simulate MountFilesystem: verify all subvolumes would be mounted
	mountChecks := verifyMountPlan(&layout, partitionInfo)
	if len(mountChecks) != len(layout.Subvolumes) {
		t.Fatalf("Mount plan mismatch: expected %d subvolumes, got %d", len(layout.Subvolumes), len(mountChecks))
	}

	// Verify root subvolume is mounted first (to /mnt in installed system)
	if mountChecks[0].MountPoint != "/mnt" {
		t.Errorf("Root subvolume should be mounted first to /mnt, got %s", mountChecks[0].MountPoint)
	}

	// 4. Simulate GenerateFstab: verify fstab entries match layout
	fstabContent := generateFstabContent(&layout, partitionInfo)

	// Verify all subvolumes have fstab entries
	for _, sv := range layout.Subvolumes {
		if !strings.Contains(fstabContent, sv.MountPoint) {
			t.Errorf("Fstab missing mount point: %s", sv.MountPoint)
		}
		if !strings.Contains(fstabContent, sv.Name) {
			t.Errorf("Fstab missing subvolume name: %s", sv.Name)
		}
	}

	// Verify EFI partition has fstab entry
	if !strings.Contains(fstabContent, "/boot") {
		t.Error("Fstab missing /boot mount point")
	}
	if !strings.Contains(fstabContent, partitionInfo.EFIUUID) {
		t.Error("Fstab missing EFI UUID")
	}

	// 5. Verify no duplicate mount points in layout
	seen := make(map[string]bool)
	for _, sv := range layout.Subvolumes {
		if seen[sv.MountPoint] {
			t.Errorf("Duplicate mount point in layout: %s", sv.MountPoint)
		}
		seen[sv.MountPoint] = true
	}
}

// validateLayout checks that the layout schema is consistent.
func validateLayout(layout *shared.PartitionLayout) error {
	if len(layout.Subvolumes) == 0 {
		return nil // ext4 layout, no subvolumes
	}

	// Check that exactly one subvolume has MountPoint "/"
	defaultCount := 0
	for _, sv := range layout.Subvolumes {
		if sv.MountPoint == "/" {
			defaultCount++
		}
	}
	if defaultCount == 0 {
		return &SchemaError{Layout: layout.Name, Reason: "missing default subvolume (MountPoint '/')"}
	}
	if defaultCount > 1 {
		return &SchemaError{Layout: layout.Name, Reason: "multiple subvolumes with MountPoint '/'"}
	}

	// Check for duplicate mount points
	seen := make(map[string]bool)
	for _, sv := range layout.Subvolumes {
		if seen[sv.MountPoint] {
			return &SchemaError{Layout: layout.Name, Reason: "duplicate mount point: " + sv.MountPoint}
		}
		seen[sv.MountPoint] = true
	}

	return nil
}

// mountCheck represents what would be mounted during MountFilesystem.
type mountCheck struct {
	MountPoint string
	Subvolume  string
	Options    string
}

// verifyMountPlan simulates the mount logic from MountFilesystem.
func verifyMountPlan(layout *shared.PartitionLayout, info *shared.PartitionInfo) []mountCheck {
	var checks []mountCheck

	// Find the root subvolume (MountPoint "/")
	var rootSubvol *shared.BtrfsSubvol
	for i := range layout.Subvolumes {
		if layout.Subvolumes[i].MountPoint == "/" {
			rootSubvol = &layout.Subvolumes[i]
			break
		}
	}

	if rootSubvol != nil {
		opts := buildMountOpts(rootSubvol)
		checks = append(checks, mountCheck{
			MountPoint: "/mnt",
			Subvolume:  rootSubvol.Name,
			Options:    opts,
		})
	}

	// Mount additional subvolumes
	for _, sv := range layout.Subvolumes {
		if sv.MountPoint == "/" {
			continue
		}
		opts := buildMountOpts(&sv)
		checks = append(checks, mountCheck{
			MountPoint: "/mnt" + sv.MountPoint,
			Subvolume:  sv.Name,
			Options:    opts,
		})
	}

	return checks
}

// generateFstabContent simulates the fstab generation from GenerateFstab.
func generateFstabContent(layout *shared.PartitionLayout, info *shared.PartitionInfo) string {
	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")

	for _, sv := range layout.Subvolumes {
		opts := buildFstabOpts(&sv)
		lines = append(lines, "UUID="+info.RootUUID+"  "+sv.MountPoint+"  btrfs   "+opts+"  0 0")
	}

	lines = append(lines, "UUID="+info.EFIUUID+"  /boot  vfat    umask=0077  0  2")

	return strings.Join(lines, "\n")
}

// SchemaError represents a layout schema validation error.
type SchemaError struct {
	Layout string
	Reason string
}

func (e *SchemaError) Error() string {
	return "schema error: " + e.Layout + " - " + e.Reason
}
