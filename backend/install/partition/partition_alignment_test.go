package partition

import (
	"testing"

	"aelum-installer/backend/shared"
)

// newLayout builds a ManualLayout with the given size and rows.
func newLayout(diskSizeMB int64, rows ...shared.ManualPartition) *shared.ManualLayout {
	return &shared.ManualLayout{Disk: "/dev/vda", DiskSizeMB: diskSizeMB, Partitions: rows}
}

// TestBuildActionsEmptyDiskNoStartGap verifies that when the disk is wiped
// (no existing partitions) the first new partition starts exactly on the first
// usable GPT sector (2048), not one alignment step past it (4096). The older
// alignUp(maxEnd+1, 2048) logic produced the 4096 start and a visible ~2M free
// region at the front of the disk.
func TestBuildActionsEmptyDiskNoStartGap(t *testing.T) {
	layout := newLayout(20480,
		shared.ManualPartition{SizeMB: shared.EFIPartitionSizeMB, Type: shared.TypeESP},
		shared.ManualPartition{SizeMB: 0, Type: shared.TypeSystem},
	)

	actions, err := buildActions("/dev/vda", layout, shared.FileSystemExt4, nil)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	esp := actions[0]
	if !esp.isNew || esp.pType != shared.TypeESP {
		t.Fatalf("action[0] should be the new ESP, got %+v", esp)
	}
	if esp.startSector != shared.GPTPartitionStartSector {
		t.Errorf("ESP start sector = %d, want %d (no 2M start gap)", esp.startSector, shared.GPTPartitionStartSector)
	}

	// Root must sit edge-to-edge after the EFI partition: right after its end
	// sector, with no spurious 1 MiB gap.
	wantRootStart := esp.startSector + (shared.EFIPartitionSizeMB*1024*1024)/512
	root := actions[1]
	if !root.isNew || root.pType != shared.TypeSystem {
		t.Fatalf("action[1] should be the new system, got %+v", root)
	}
	if root.startSector != wantRootStart {
		t.Errorf("system start sector = %d, want %d (contiguous, no 1M gap)", root.startSector, wantRootStart)
	}
}

// TestBuildActionsContiguousAfterAlignedPartition verifies that a new partition
// placed after an existing partition that ends exactly on a 1 MiB boundary is
// positioned contiguously (at end+1), not one MiB later.
func TestBuildActionsContiguousAfterAlignedPartition(t *testing.T) {
	// Existing 512 MiB ESP ending exactly on a 1 MiB boundary: inclusive end
	// sector 1052671, so the next free sector 1052672 is already aligned.
	sectors := []partitionSector{{num: 1, start: 2048, end: 1052671}}
	layout := newLayout(20480,
		shared.ManualPartition{Path: "/dev/vda1", SizeMB: 0, Type: shared.TypeESP},
		shared.ManualPartition{SizeMB: 0, Type: shared.TypeSystem},
	)

	actions, err := buildActions("/dev/vda", layout, shared.FileSystemExt4, sectors)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}

	var root *formatAction
	for i := range actions {
		if actions[i].pType == shared.TypeSystem {
			root = &actions[i]
		}
	}
	if root == nil {
		t.Fatal("no system action returned")
	}
	if root.startSector != 1052672 {
		t.Errorf("system start sector = %d, want %d (contiguous after aligned partition)", root.startSector, 1052672)
	}
}
