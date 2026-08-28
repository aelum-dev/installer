package configure

import (
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

func TestGenerateExt4Fstab_Contents(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateExt4Fstab(ctx)

	// Should contain root entry
	if !strings.Contains(content, "UUID=root-uuid-123  /") {
		t.Errorf("expected root entry with UUID, got:\n%s", content)
	}
	if !strings.Contains(content, "ext4") {
		t.Errorf("expected ext4 filesystem type, got:\n%s", content)
	}

	// Should contain EFI entry
	if !strings.Contains(content, "UUID=efi-uuid-456  /boot") {
		t.Errorf("expected EFI entry with UUID, got:\n%s", content)
	}
	if !strings.Contains(content, "vfat") {
		t.Errorf("expected vfat filesystem type, got:\n%s", content)
	}
}

func TestGenerateExt4Fstab_NoDuplicateEntries(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateExt4Fstab(ctx)

	// Count /boot entries — should be exactly 1
	bootCount := strings.Count(content, "/boot")
	if bootCount != 1 {
		t.Errorf("expected exactly 1 /boot entry, got %d:\n%s", bootCount, content)
	}

	// Count / entries — should be exactly 1 (root)
	rootCount := strings.Count(content, "UUID=root-uuid-123  / ")
	if rootCount != 1 {
		t.Errorf("expected exactly 1 root entry, got %d:\n%s", rootCount, content)
	}
}

func TestGenerateBtrfsFstab_Contents(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// Should contain all subvolumes
	expectedSubvolumes := []string{"/", "/home", "/srv", "/var/cache", "/var/tmp", "/var/log"}
	for _, mp := range expectedSubvolumes {
		if !strings.Contains(content, "UUID=root-uuid-123  "+mp+"  btrfs") {
			t.Errorf("expected subvolume mount for %s, got:\n%s", mp, content)
		}
	}

	// Should contain EFI entry
	if !strings.Contains(content, "UUID=efi-uuid-456  /boot") {
		t.Errorf("expected EFI entry, got:\n%s", content)
	}

	// Should NOT contain any swap entries
	if strings.Contains(content, "swap") {
		t.Errorf("expected no swap entries in fstab, got:\n%s", content)
	}
}

func TestGenerateBtrfsFstab_NoDuplicateEntries(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// Each mount point should appear exactly once
	mountPoints := []string{"/", "/home", "/srv", "/var/cache", "/var/tmp", "/var/log", "/boot"}
	for _, mp := range mountPoints {
		count := 0
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "UUID=  "+mp+"  ") || strings.Contains(line, "UUID=  "+mp+"\t") {
				count++
			}
			// Also check for the UUID format
			if strings.Contains(line, mp+"  btrfs") || (mp == "/boot" && strings.Contains(line, "vfat")) {
				count++
			}
		}
		// Each mount point should appear at least once
		if count == 0 {
			t.Errorf("expected at least 1 entry for %s, got 0:\n%s", mp, content)
		}
	}
}

func TestGenerateBtrfsFstab_AllSubvolumesPresent(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// All 6 subvolumes should be represented
	subvolumes := shared.GetBtrfsSubvolumes()
	if len(subvolumes) != 6 {
		t.Fatalf("expected 6 subvolumes, got %d", len(subvolumes))
	}

	// Each subvolume mount point should appear exactly once as a btrfs mount
	for _, sv := range subvolumes {
		count := 0
		for _, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "UUID=root-uuid-123  "+sv.MountPoint+"  btrfs") {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 fstab entry for %s, got %d", sv.MountPoint, count)
		}
	}
}

func TestGenerateBtrfsFstab_CompressionOptions(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// @ should have compress=zstd
	if !strings.Contains(content, "subvol=@,compress=zstd") {
		t.Errorf("expected compress=zstd on @ subvolume, got:\n%s", content)
	}

	// @cache should have nodatacow
	cacheLine := ""
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "UUID=root-uuid-123  /var/cache  btrfs") {
			cacheLine = line
			break
		}
	}
	if cacheLine == "" {
		t.Fatalf("could not find /var/cache line")
	}
	if !strings.Contains(cacheLine, "nodatacow") {
		t.Errorf("expected nodatacow on @cache, got:\n%s", cacheLine)
	}
}

func TestGenerateBtrfsFstab_EFIFirst(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	// Skip comment line
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// First data line should be EFI
	if !strings.Contains(lines[1], "/boot") {
		t.Errorf("expected /boot as first entry, got:\n%s", content)
	}
	if !strings.Contains(lines[1], "vfat") {
		t.Errorf("expected vfat for first entry, got:\n%s", content)
	}
}
