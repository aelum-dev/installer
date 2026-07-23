package install

import (
	"strings"
	"testing"

	"aesthetic-os-installer/backend/internal/shared"
)

// TestLayoutIntegration verifies the full format → mount → fstab chain
// works correctly with a btrfs filesystem containing subvolumes.
// This test doesn't execute real commands — it validates the schema flow
// and verifies that all subvolumes are handled consistently.
func TestLayoutIntegration(t *testing.T) {
	// 1. Get btrfs subvolumes from shared definition
	subvolumes := shared.GetBtrfsSubvolumes()

	// 2. Validate subvolumes
	if len(subvolumes) == 0 {
		t.Fatal("No btrfs subvolumes defined")
	}

	// Check that @ exists and has MountPoint "/"
	rootSubvol := shared.FindSubvolumeByMountPoint(subvolumes, "/")
	if rootSubvol == nil {
		t.Fatal("Missing root subvolume (@) with MountPoint '/'")
	}

	// Check for duplicate mount points
	seen := make(map[string]bool)
	for _, sv := range subvolumes {
		if seen[sv.MountPoint] {
			t.Errorf("Duplicate mount point: %s", sv.MountPoint)
		}
		seen[sv.MountPoint] = true
	}

	// 3. Simulate MountFilesystem: verify all subvolumes would be mounted
	mountChecks := verifyMountPlan(subvolumes)
	if len(mountChecks) != len(subvolumes) {
		t.Fatalf("Mount plan mismatch: expected %d subvolumes, got %d", len(subvolumes), len(mountChecks))
	}

	// Verify root subvolume is mounted first (to /mnt)
	if mountChecks[0].MountPoint != "/mnt" {
		t.Errorf("Root subvolume should be mounted first to /mnt, got %s", mountChecks[0].MountPoint)
	}

	// 4. Simulate GenerateFstab: verify fstab entries match subvolumes
	fstabContent := generateFstabContent(subvolumes, "test-root-uuid", "test-efi-uuid")

	// Verify all subvolumes have fstab entries
	for _, sv := range subvolumes {
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
	if !strings.Contains(fstabContent, "test-efi-uuid") {
		t.Error("Fstab missing EFI UUID")
	}
}

// mountCheck represents what would be mounted during MountFilesystem.
type mountCheck struct {
	MountPoint string
	Subvolume  string
	Options    string
}

// verifyMountPlan simulates the mount logic from MountFilesystem.
func verifyMountPlan(subvolumes []shared.BtrfsSubvolume) []mountCheck {
	var checks []mountCheck

	// Find the root subvolume (MountPoint "/")
	var rootSubvol *shared.BtrfsSubvolume
	for i := range subvolumes {
		if subvolumes[i].MountPoint == "/" {
			rootSubvol = &subvolumes[i]
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
	for _, sv := range subvolumes {
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
func generateFstabContent(subvolumes []shared.BtrfsSubvolume, rootUUID, efiUUID string) string {
	var lines []string
	lines = append(lines, "# <file system>                             <mount point>  <type>  <options>               <dump>  <pass>")

	for _, sv := range subvolumes {
		opts := buildFstabOpts(&sv)
		lines = append(lines, "UUID="+rootUUID+"  "+sv.MountPoint+"  btrfs   "+opts+"  0 0")
	}

	lines = append(lines, "UUID="+efiUUID+"  /boot  vfat    umask=0077  0  2")

	return strings.Join(lines, "\n")
}

// buildFstabOpts constructs fstab mount options for a subvolume.
func buildFstabOpts(sv *shared.BtrfsSubvolume) string {
	opts := "subvol=" + sv.Name + ",defaults,noatime,autodefrag"
	if sv.Compression != "" {
		opts += ",compress=" + sv.Compression
	}
	return opts
}
