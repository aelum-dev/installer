package install

import (
	"strings"
	"testing"

	"aestheticos-installer/backend/internal/shared"
)

func TestGenerateExt4Fstab_Contents(t *testing.T) {
	ctx := &InstallContext{
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

	// Should contain swap entry
	if !strings.Contains(content, "/swapfile  none  swap") {
		t.Errorf("expected swap entry, got:\n%s", content)
	}
}

func TestGenerateExt4Fstab_NoDuplicateEntries(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateExt4Fstab(ctx)

	// Count swap entries — should be exactly 1 (the "none  swap" type field)
	swapTypeCount := strings.Count(content, "none  swap")
	if swapTypeCount != 1 {
		t.Errorf("expected exactly 1 swap type entry, got %d:\n%s", swapTypeCount, content)
	}

	// Count /boot entries — should be exactly 1
	bootCount := strings.Count(content, "/boot")
	if bootCount != 1 {
		t.Errorf("expected exactly 1 /boot entry, got %d:\n%s", bootCount, content)
	}
}

func TestGenerateBtrfsFstab_Contents(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// Should contain all subvolumes except @swap (handled separately)
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

	// Should contain @swap mount entry
	if !strings.Contains(content, "UUID=root-uuid-123  /swap  btrfs") {
		t.Errorf("expected @swap mount entry, got:\n%s", content)
	}

	// Should contain swap file entry
	if !strings.Contains(content, "/swap/swapfile  none  swap") {
		t.Errorf("expected swap file entry, got:\n%s", content)
	}
}

func TestGenerateBtrfsFstab_NoDuplicateSwapEntries(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// Count /swap entries — should be exactly 2: one mount, one swap file
	swapMountCount := strings.Count(content, "UUID=root-uuid-123  /swap  btrfs")
	if swapMountCount != 1 {
		t.Errorf("expected exactly 1 @swap mount entry, got %d:\n%s", swapMountCount, content)
	}

	swapFileCount := strings.Count(content, "/swap/swapfile  none  swap")
	if swapFileCount != 1 {
		t.Errorf("expected exactly 1 swap file entry, got %d:\n%s", swapFileCount, content)
	}

	// Total swap-related lines should be 2
	totalSwapLines := swapMountCount + swapFileCount
	if totalSwapLines != 2 {
		t.Errorf("expected exactly 2 swap-related entries, got %d:\n%s", totalSwapLines, content)
	}
}

func TestGenerateBtrfsFstab_NodatacowOnSwap(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// @swap should have nodatacow option
	swapMountLine := ""
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "UUID=root-uuid-123  /swap  btrfs") {
			swapMountLine = line
			break
		}
	}
	if swapMountLine == "" {
		t.Fatalf("could not find @swap mount line")
	}
	if !strings.Contains(swapMountLine, "nodatacow") {
		t.Errorf("expected nodatacow on @swap mount, got:\n%s", swapMountLine)
	}
}

func TestGenerateBtrfsFstab_SwapAfterMount(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	// Find line numbers for @swap mount and swap file
	swapMountLine := -1
	swapFileLine := -1
	for i, line := range lines {
		if strings.Contains(line, "UUID=root-uuid-123  /swap  btrfs") {
			swapMountLine = i
		}
		if strings.Contains(line, "/swap/swapfile  none  swap") {
			swapFileLine = i
		}
	}

	if swapMountLine < 0 {
		t.Fatalf("could not find @swap mount line")
	}
	if swapFileLine < 0 {
		t.Fatalf("could not find swap file line")
	}
	if swapFileLine <= swapMountLine {
		t.Errorf("swap file entry must come after @swap mount entry (mount at line %d, swap at line %d)", swapMountLine, swapFileLine)
	}
}

func TestGetSwapFstabEntry_Ext4(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
		},
	}

	entry := GetSwapFstabEntry(ctx)
	expected := "/swapfile  none  swap  defaults  0  0"
	if entry != expected {
		t.Errorf("GetSwapFstabEntry(ext4) = %q, want %q", entry, expected)
	}
}

func TestGetSwapFstabEntry_Btrfs(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
		},
	}

	entry := GetSwapFstabEntry(ctx)
	// For btrfs, GetSwapFstabEntry now returns the generic swap file entry
	// (the @swap mount is handled in generateBtrfsFstab)
	expected := "/swapfile  none  swap  defaults  0  0"
	if entry != expected {
		t.Errorf("GetSwapFstabEntry(btrfs) = %q, want %q", entry, expected)
	}
}

func TestGenerateBtrfsFstab_AllSubvolumesPresent(t *testing.T) {
	ctx := &InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{
			RootUUID: "root-uuid-123",
			EFIUUID:  "efi-uuid-456",
		},
	}

	content := generateBtrfsFstab(ctx)

	// All 7 subvolumes should be represented (6 in loop + 1 @swap mount)
	subvolumes := shared.GetBtrfsSubvolumes()
	if len(subvolumes) != 7 {
		t.Fatalf("expected 7 subvolumes, got %d", len(subvolumes))
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
	ctx := &InstallContext{
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

	// @cache should NOT have compress (it has NoDataCow)
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
	if strings.Contains(cacheLine, "compress") {
		t.Errorf("expected no compression on @cache (nodatacow), got:\n%s", cacheLine)
	}
	if !strings.Contains(cacheLine, "nodatacow") {
		t.Errorf("expected nodatacow on @cache, got:\n%s", cacheLine)
	}
}
