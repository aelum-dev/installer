package partition

import (
	"testing"

	"aelum-installer/backend/shared"
)

// TestBuildActions_ClampedRootBelowMinimumErrors reproduces a fragmentation case:
// the free space on disk is large enough in total, but split across two gaps by
// a kept partition, so no single contiguous region fits a requested 8 GiB root.
// buildActions now refuses to clamp the root down to the largest gap (~6 GiB) —
// below the 8 GiB minimum — and returns an error, because Validate only checks
// the requested size against the minimum and cannot see the fragmentation.
func TestBuildActions_ClampedRootBelowMinimumErrors(t *testing.T) {
	const mib = int64(1024 * 1024)
	diskMB := int64(30000)
	// Existing 512 MiB ESP at the front, and a large kept partition in the middle
	// that splits the remaining free space into two gaps, each smaller than 8 GiB
	// even though together they exceed it.
	sectors := []partitionSector{
		{num: 1, start: 2048, end: 2048 + 512*2048 - 1}, // existing ESP (512 MiB)
		{num: 2, start: 12000000, end: 48000000},        // kept partition (~17.6 GiB)
	}
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    diskMB,
		DiskSizeBytes: diskMB * mib,
		Partitions: []shared.ManualPartition{
			{Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * mib, Type: shared.TypeESP, ExistingFstype: "vfat"}, // existing ESP kept
			// SizeBytes matches the exact sector span (48000000-12000000+1) so Validate
			// and buildActions agree on how much space the kept partition occupies.
			{Path: "/dev/vda2", SizeMB: 17578, SizeBytes: 36000001 * 512}, // kept, preserve (empty type)
			{Path: "", SizeMB: 8192, Type: shared.TypeSystem},             // new root, 8 GiB
		},
	}

	// Validate must pass: requested 8 GiB meets the floor and total free space
	// (12000 MB) covers it. If this fails the test setup is wrong, not the code.
	if err := Validate(layout); err != nil {
		t.Fatalf("setup: Validate returned unexpected error: %v", err)
	}

	if _, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, sectors); err == nil {
		t.Error("buildActions: expected an error when the root is clamped below the 8 GiB minimum by fragmentation, got nil")
	}
}

// TestBuildActions_ClampedRootFitsNoError confirms the guard only fires when the
// resolved size is genuinely below the minimum: a root that fits in a single gap
// is produced without error.
func TestBuildActions_ClampedRootFitsNoError(t *testing.T) {
	const mib = int64(1024 * 1024)
	diskMB := int64(30000)
	// vda2 spans 12000 MiB; its front and rear gaps are each large enough for a
	// 6000 MiB root, so no clamping below the minimum occurs.
	keptStart := int64(262410 + 8000*2048)
	keptEnd := keptStart + 12000*2048 - 1
	keptSpan := keptEnd - keptStart + 1
	sectors := []partitionSector{
		{num: 1, start: 2048, end: 2048 + 512*2048 - 1}, // existing ESP (512 MiB)
		{num: 2, start: keptStart, end: keptEnd},        // kept partition (12000 MiB)
	}
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    diskMB,
		DiskSizeBytes: diskMB * mib,
		Partitions: []shared.ManualPartition{
			{Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * mib, Type: shared.TypeESP, ExistingFstype: "vfat"},
			{Path: "/dev/vda2", SizeMB: 12000, SizeBytes: keptSpan * 512}, // kept, preserve (empty type)
			{Path: "", SizeMB: 9000, Type: shared.TypeSystem},             // fits the rear gap, above the 8 GiB floor
		},
	}
	if err := Validate(layout); err != nil {
		t.Fatalf("setup: Validate returned unexpected error: %v", err)
	}
	actions, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, sectors)
	if err != nil {
		t.Fatalf("buildActions: unexpected error: %v", err)
	}
	var root *formatAction
	for _, a := range actions {
		if a.pType == shared.TypeSystem {
			root = &a
		}
	}
	if root == nil || root.sizeMB < shared.MinSystemPartitionBytes/mib {
		t.Fatalf("expected a root at or above the 8 GiB minimum, got %+v", root)
	}
}
