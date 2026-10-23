package format

import (
	"testing"

	"aelum-installer/backend/shared"
)

// TestRun_EmptyDiskID verifies the step rejects a missing disk before doing any
// work.
func TestRun_EmptyDiskID(t *testing.T) {
	if err := Run(&shared.InstallContext{}); err == nil {
		t.Fatal("expected an error for an empty disk ID, got nil")
	}
}

// TestRun_UnsupportedFilesystem verifies an unknown filesystem is rejected up
// front, before any partitioning command runs.
func TestRun_UnsupportedFilesystem(t *testing.T) {
	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: "xfs"}
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for an unsupported filesystem, got nil")
	}
}

func TestPartitionTypes(t *testing.T) {
	tests := []struct {
		name  string
		parts []shared.PartitionEntry
		want  string
	}{
		{"empty", nil, "none"},
		{"fstype preferred", []shared.PartitionEntry{{Fstype: "vfat"}, {Fstype: "ext4"}}, "vfat, ext4"},
		{"deduplicates", []shared.PartitionEntry{{Fstype: "ext4"}, {Fstype: "ext4"}}, "ext4"},
		{"falls back to type", []shared.PartitionEntry{{Type: "part"}}, "part"},
		{"unknown when both empty", []shared.PartitionEntry{{}}, "unknown"},
	}
	for _, tt := range tests {
		if got := partitionTypes(tt.parts); got != tt.want {
			t.Errorf("%s: partitionTypes = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFindPartitionsByFstype(t *testing.T) {
	parts := []shared.PartitionEntry{
		{Path: "/dev/vda1", Fstype: "ntfs"},
		{Path: "/dev/vda2", Fstype: "vfat"},
		{Path: "/dev/vda3", Fstype: "ext4"},
		{Path: "/dev/vda4", Fstype: "btrfs"},
	}

	efi, root := findPartitionsByFstype(parts)
	if efi == nil || efi.Path != "/dev/vda2" {
		t.Errorf("EFI partition = %+v, want /dev/vda2", efi)
	}
	if root == nil || root.Path != "/dev/vda3" {
		t.Errorf("root partition = %+v, want the first ext4/btrfs (/dev/vda3)", root)
	}

	if efi, root := findPartitionsByFstype([]shared.PartitionEntry{{Fstype: "ntfs"}}); efi != nil || root != nil {
		t.Errorf("expected no matches, got efi=%+v root=%+v", efi, root)
	}
}

func TestFindPartitionsByIndex(t *testing.T) {
	parts := []shared.PartitionEntry{{Path: "/dev/vda1"}, {Path: "/dev/vda2"}}

	efi, root := findPartitionsByIndex(parts)
	if efi == nil || efi.Path != "/dev/vda1" {
		t.Errorf("EFI (index 0) = %+v, want /dev/vda1", efi)
	}
	if root == nil || root.Path != "/dev/vda2" {
		t.Errorf("root (index 1) = %+v, want /dev/vda2", root)
	}

	efi, root = findPartitionsByIndex(parts[:1])
	if efi == nil || root != nil {
		t.Errorf("single partition: efi=%+v root=%+v, want efi set and root nil", efi, root)
	}
}

// newInfo mirrors the PartitionInfo NewPartitionInfo builds, with the
// mount-point map initialized (applyRootSetup writes into it).
func newInfo() *shared.PartitionInfo {
	return &shared.PartitionInfo{MountPoints: map[string]string{}}
}

// TestApplyRootSetup covers the LUKS/plain device and subvolume mapping the
// mount and fstab steps consume.
func TestApplyRootSetup(t *testing.T) {
	t.Run("plain ext4", func(t *testing.T) {
		info := newInfo()
		applyRootSetup(info, &shared.InstallContext{FileSystem: shared.FileSystemExt4},
			RootSetup{RootDevice: "/dev/vda2", RootUUID: "root-uuid"}, shared.FileSystemExt4)

		if info.RootUUID != "root-uuid" {
			t.Errorf("RootUUID = %q, want root-uuid", info.RootUUID)
		}
		if info.RootMapper != "" {
			t.Errorf("plain ext4 must not set the mapper field: %+v", info)
		}
		if len(info.MountPoints) != 0 {
			t.Errorf("ext4 has no subvolumes, got %v", info.MountPoints)
		}
	})

	t.Run("plain btrfs", func(t *testing.T) {
		info := newInfo()
		applyRootSetup(info, &shared.InstallContext{FileSystem: shared.FileSystemBtrfs},
			RootSetup{RootDevice: "/dev/vda2", RootUUID: "root-uuid"}, shared.FileSystemBtrfs)

		if _, ok := info.MountPoints["/"]; ok {
			t.Error("the root mount point / must not be in the subvolume map")
		}
		if want := len(shared.GetBtrfsSubvolumes()) - 1; len(info.MountPoints) != want {
			t.Errorf("MountPoints has %d entries, want %d", len(info.MountPoints), want)
		}
		for mp, dev := range info.MountPoints {
			if dev != "/dev/vda2" {
				t.Errorf("MountPoints[%q] = %q, want /dev/vda2", mp, dev)
			}
		}
	})

	t.Run("luks", func(t *testing.T) {
		info := newInfo()
		applyRootSetup(info, &shared.InstallContext{Luks: true, FileSystem: shared.FileSystemExt4},
			RootSetup{RootDevice: shared.LuksMapperPath, RootUUID: "mapper-uuid", LuksUUID: "luks-uuid"},
			shared.FileSystemExt4)

		if info.RootMapper != shared.LuksMapperPath {
			t.Errorf("RootMapper = %q, want %q", info.RootMapper, shared.LuksMapperPath)
		}
		if info.LuksUUID != "luks-uuid" {
			t.Errorf("LuksUUID = %q, want luks-uuid", info.LuksUUID)
		}
	})

}
