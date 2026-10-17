package partition

import (
	"testing"

	"aelum-installer/backend/shared"
)

// Real sgdisk -p output from a guest: a whitespace-aligned table whose first
// three columns are Number, Start (sector), End (sector). This is the format
// sgdisk actually prints; the old parser expected a "Partition #N is start
// sector..." prose line that sgdisk no longer emits.
const realSgdiskOut = `Disk /dev/vda: 41943040 sectors, 20.0 GiB
Sector size (logical/physical): 512/512 bytes
Partition table holds up to 128 entries
Main partition table begins at sector 2 and ends at sector 33
First usable sector is 34, last usable sector is 41943006
Partitions will be aligned on 2048-sector boundaries
Total free space is 15269821 sectors (7.3 GiB)

Number  Start (sector)    End (sector)  Size       Code  Name
   1            2048         2099199   1024.0 MiB  EF00  EFI
   2         2099200        26675199   11.7 GiB    8304  aelum`

// TestParsePartitionSectorsRealFormat guards the sgdisk -p table parser: it must
// read the numbered partition rows and skip the header and free-space summary.
func TestParsePartitionSectorsRealFormat(t *testing.T) {
	got := parsePartitionSectors(realSgdiskOut)
	if len(got) != 2 {
		t.Fatalf("expected 2 partitions parsed, got %d: %+v", len(got), got)
	}
	if got[0].num != 1 || got[0].start != 2048 || got[0].end != 2099199 {
		t.Errorf("partition 1 wrong: %+v", got[0])
	}
	if got[1].num != 2 || got[1].start != 2099200 || got[1].end != 26675199 {
		t.Errorf("partition 2 wrong: %+v", got[1])
	}
}

// TestBuildActionsESPOnExistingNonFirst reproduces the reported bug: assigning
// EFI to an existing partition that is not the first on the disk, then adding a
// new root. With a working sector parser the new root must be carved after the
// existing partitions; with the old (empty) parser it was placed at sector 2048
// and sgdisk rejected it with "Could not create partition N from 2048 ...".
func TestBuildActionsESPOnExistingNonFirst(t *testing.T) {
	diskID := "/dev/vda"
	layout := &shared.ManualLayout{
		Disk: diskID, DiskSizeMB: 23000,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 1024},                        // preserve (existing EFI)
			{Index: 1, Path: "/dev/vda2", SizeMB: 11972, Type: shared.TypeESP}, // reuse as EFI
			{Index: 2, Path: "", SizeMB: 0, Type: shared.TypeSystem},           // new root, use remaining
		},
	}
	actions, err := buildActions(diskID, layout, shared.FileSystemExt4, parsePartitionSectors(realSgdiskOut))
	if err != nil {
		t.Fatalf("buildActions error: %v", err)
	}
	for _, a := range actions {
		if a.isNew && a.startSector <= 2099200 {
			t.Errorf("new partition %d placed at %d, overlaps existing partition 2 (ends at 26675199)", a.num, a.startSector)
		}
	}
}
