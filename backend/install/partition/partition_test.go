package partition

import (
	"testing"

	"aelum-installer/backend/shared"
)

func TestBytesToMB(t *testing.T) {
	cases := []struct {
		bytes  int64
		wantMB int64
	}{
		{0, 0},
		{-5, 0},
		{1024 * 1024, 1},
		{2 * 1024 * 1024, 2},
		{42949672960, 40960},
	}
	for _, c := range cases {
		if got := bytesToMB(c.bytes); got != c.wantMB {
			t.Errorf("bytesToMB(%d) = %d, want %d", c.bytes, got, c.wantMB)
		}
	}
}

func TestParsePartitionNum(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{"/dev/sda1", 1},
		{"/dev/sda3", 3},
		{"/dev/nvme0n1p1", 1},
		{"/dev/nvme0n1p12", 12},
		{"/dev/nvme3n1p12", 12}, // any controller/disk index
		{"/dev/sda21", 21},      // conventional trailing number
	}
	for _, c := range cases {
		if got := parsePartitionNum(c.path); got != c.want {
			t.Errorf("parsePartitionNum(%q) = %d, want %d", c.path, got, c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	validLayout := func() shared.ManualLayout {
		return shared.ManualLayout{
			Disk:       "/dev/sda",
			DiskSizeMB: 100000,
			Partitions: []shared.ManualPartition{
				{Index: 0, Path: "/dev/sda1", SizeMB: 128, Type: shared.TypeESP},
				{Index: 1, Path: "/dev/sda2", SizeMB: 50000, Type: shared.TypeSystem},
				{Index: 2, Path: "/dev/sda3", SizeMB: 0}, // empty role = untouched
			},
		}
	}

	tests := []struct {
		name    string
		layout  shared.ManualLayout
		wantErr bool
	}{
		{
			name:    "valid dualboot layout",
			layout:  validLayout(),
			wantErr: false,
		},
		{
			name: "no boot",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda2", SizeMB: 50000, Type: shared.TypeSystem},
				},
			},
			wantErr: true,
		},
		{
			name: "two boots",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Type: shared.TypeESP},
					{Path: "/dev/sda2", SizeMB: 128, Type: shared.TypeESP},
					{Path: "/dev/sda3", SizeMB: 50000, Type: shared.TypeSystem},
				},
			},
			wantErr: true,
		},
		{
			name: "no root",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Type: shared.TypeESP},
				},
			},
			wantErr: true,
		},
		{
			name: "two roots",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Type: shared.TypeESP},
					{Path: "/dev/sda2", SizeMB: 50000, Type: shared.TypeSystem},
					{Path: "/dev/sda3", SizeMB: 50000, Type: shared.TypeSystem},
				},
			},
			wantErr: true,
		},
		{
			name: "over size",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 60000, Type: shared.TypeESP},
					{Path: "/dev/sda2", SizeMB: 60000, Type: shared.TypeSystem},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple use-all",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 0, Type: shared.TypeESP},
					{Path: "/dev/sda2", SizeMB: 0, Type: shared.TypeSystem},
				},
			},
			wantErr: true,
		},
		{
			name: "unknown role",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Type: shared.PartitionType("swap")},
					{Path: "/dev/sda2", SizeMB: 128, Type: shared.TypeESP},
					{Path: "/dev/sda3", SizeMB: 50000, Type: shared.TypeSystem},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.layout)
			if tc.wantErr && err == nil {
				t.Errorf("Validate(%q) = nil, want error", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Validate(%q) = %v, want nil", tc.name, err)
			}
		})
	}
}

// TestBuildActions_DualbootInvariant asserts the core dualboot safety property:
// Preserve partitions never appear as format actions, while the single Boot
// (vfat/ESP) and Root (ext4/btrfs/LinuxRoot) partitions are correctly keyed by
// role. Existing partitions keep their number; new partitions get the next.
func TestBuildActions_DualbootInvariant(t *testing.T) {
	sectors := []partitionSector{
		{num: 1, start: 2048, end: 1050623},
		{num: 2, start: 1050624, end: 8388609},
	}

	layout := shared.ManualLayout{
		Disk:       "/dev/sda",
		DiskSizeMB: 100000,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/sda1", SizeMB: 512, ExistingFstype: "ext4"}, // empty role = untouched
			{Index: 1, Path: "/dev/sda3", SizeMB: 20000, Type: shared.TypeSystem, ExistingFstype: "ext4"},
			{Index: 2, Path: "", SizeMB: 128, Type: shared.TypeESP},
		},
	}

	actions, err := buildActions("/dev/sda", &layout, shared.FileSystemExt4, sectors)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}

	if len(actions) != 2 {
		t.Fatalf("expected 2 format actions (ESP + System), got %d", len(actions))
	}

	var boot, root *formatAction
	for i := range actions {
		if actions[i].pType == shared.TypeESP {
			boot = &actions[i]
		}
		if actions[i].pType == shared.TypeSystem {
			root = &actions[i]
		}
	}
	if boot == nil {
		t.Fatal("no Boot action produced")
	}
	if root == nil {
		t.Fatal("no Root action produced")
	}

	// Boot: vfat + ESP GUID, and it is a brand-new partition.
	if boot.fstype != shared.FileSystemVfat {
		t.Errorf("boot fstype = %q, want %q", boot.fstype, shared.FileSystemVfat)
	}
	if boot.guid != shared.GPTTypeGUIDs.ESP {
		t.Errorf("boot guid = %q, want %q", boot.guid, shared.GPTTypeGUIDs.ESP)
	}
	if !boot.isNew {
		t.Error("boot action should be a newly created partition")
	}
	if boot.num != 4 {
		t.Errorf("boot num = %d, want 4 (max existing 3, +1)", boot.num)
	}

	// Root: keeps its existing device number, ext4 + LinuxRoot GUID.
	if root.path != "/dev/sda3" {
		t.Errorf("root path = %q, want /dev/sda3", root.path)
	}
	if root.isNew {
		t.Error("root action should reuse the existing partition")
	}
	if root.fstype != shared.FileSystemExt4 {
		t.Errorf("root fstype = %q, want %q", root.fstype, shared.FileSystemExt4)
	}
	if root.guid != shared.GPTTypeGUIDs.LinuxRootX86_64 {
		t.Errorf("root guid = %q, want %q", root.guid, shared.GPTTypeGUIDs.LinuxRootX86_64)
	}
}

// TestBuildActions_BtrfsRoot verifies the root filesystem follows the selected
// mode (btrfs here) while the boot partition stays vfat.
func TestBuildActions_BtrfsRoot(t *testing.T) {
	sectors := []partitionSector{{num: 1, start: 2048, end: 1050623}}
	layout := shared.ManualLayout{
		Disk:       "/dev/sda",
		DiskSizeMB: 100000,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/sda1", SizeMB: 128, Type: shared.TypeESP},
			{Index: 1, Path: "", SizeMB: 0, Type: shared.TypeSystem},
		},
	}

	actions, err := buildActions("/dev/sda", &layout, shared.FileSystemBtrfs, sectors)
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
		t.Fatal("no root action")
	}
	if root.fstype != shared.FileSystemBtrfs {
		t.Errorf("root fstype = %q, want %q", root.fstype, shared.FileSystemBtrfs)
	}
	// Root uses "all remaining", capped so it ends within the disk's usable
	// region (the trailing 256 sectors are reserved for the backup GPT, so
	// sgdisk won't reject a partition extending past the last usable sector).
	// That yields 99486 MiB rather than the uncapped 100000-128 = 99872.
	want := (100000*2048 - gptBackupSectors - 1050624) / 2048
	if root.sizeMB != want {
		t.Errorf("root sizeMB = %d, want %d", root.sizeMB, want)
	}
	// The partition must fit inside the disk after reserving the backup GPT.
	rootEndSector := int64(1050624) + root.sizeMB*2048
	if rootEndSector > 100000*2048-gptBackupSectors {
		t.Errorf("root partition end sector %d exceeds disk usable space", rootEndSector)
	}
}

// TestBuildActions_AllRemainingFitsFullDisk guards the regression where a
// "use all remaining space" partition on a fully allocated disk was sized as
// DiskSizeMB - boot, extending past the last usable sector. sgdisk rejects such
// a partition ("Could not create partition ... past the end of disk"), so the
// computed partition must end within the disk after reserving the backup GPT.
func TestBuildActions_AllRemainingFitsFullDisk(t *testing.T) {
	// Fresh disk: brand-new ESP (512 MiB) + brand-new System (all remaining).
	layout := shared.ManualLayout{
		Disk:       "/dev/vda",
		DiskSizeMB: 40960,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "", SizeMB: 512, Type: shared.TypeESP},
			{Index: 1, Path: "", SizeMB: 0, Type: shared.TypeSystem},
		},
	}

	actions, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, nil)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}

	var system *formatAction
	for i := range actions {
		if actions[i].pType == shared.TypeSystem {
			system = &actions[i]
		}
	}
	if system == nil {
		t.Fatal("no system action")
	}
	if system.sizeMB == 0 {
		t.Fatal("system partition size was not resolved")
	}
	systemEndSector := system.startSector + system.sizeMB*2048
	diskSectors := layout.DiskSizeMB * 2048
	if systemEndSector > diskSectors-gptBackupSectors {
		t.Errorf("system partition end sector %d exceeds disk usable space (disk %d sectors)", systemEndSector, diskSectors)
	}
}

// TestBuildActions_NoOverlapWithExisting asserts the dualboot-safety property at
// the geometry level: every newly created partition must start at or after the
// end of ALL existing partitions (so it never carves into a kept OS/data
// partition) and must stay within the disk. Preserve (untouched) partitions must
// still advance the placement frontier even though they are never formatted.
func TestBuildActions_NoOverlapWithExisting(t *testing.T) {
	// sda1 (kept ESP) ends at 1050623; sda2 (kept Windows) ends at 8388609.
	sectors := []partitionSector{
		{num: 1, start: 2048, end: 1050623},
		{num: 2, start: 1050624, end: 8388609},
	}
	maxEndSector := int64(shared.GPTPartitionStartSector)
	for _, s := range sectors {
		if s.end > maxEndSector {
			maxEndSector = s.end
		}
	}

	layout := shared.ManualLayout{
		Disk:       "/dev/sda",
		DiskSizeMB: 100000,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/sda1", SizeMB: 512, Type: shared.TypeESP}, // kept ESP
			{Index: 1, Path: "/dev/sda2", SizeMB: 40000},                     // Windows, preserve (empty type)
			{Index: 2, Path: "", SizeMB: 20000, Type: shared.TypeSystem},     // NEW Linux root
		},
	}

	actions, err := buildActions("/dev/sda", &layout, shared.FileSystemExt4, sectors)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}

	var newAction *formatAction
	for i := range actions {
		if actions[i].isNew {
			newAction = &actions[i]
		}
	}
	if newAction == nil {
		t.Fatal("expected one brand-new partition action")
	}

	// The new partition must start at or after the end of every existing
	// partition — otherwise it would overwrite a kept partition.
	if newAction.startSector < maxEndSector {
		t.Errorf("new partition starts at sector %d, before existing partitions end at %d — would overlap a kept partition", newAction.startSector, maxEndSector)
	}

	// And it must stay within the disk (1 MiB = 2048 x 512B sectors).
	maxAllowedSectors := layout.DiskSizeMB * 1024
	endSector := newAction.startSector + newAction.sizeMB*1024
	if endSector > maxAllowedSectors {
		t.Errorf("new partition ends at sector %d, beyond disk size %d sectors", endSector, maxAllowedSectors)
	}

	// Preserve partitions must never appear as format actions.
	for i := range actions {
		if actions[i].path == "/dev/sda2" {
			t.Error("preserve partition /dev/sda2 must not be a format action")
		}
	}
}

// TestComputeLayout_FreeSpace checks that ComputeLayout presets every existing
// partition to Preserve and derives free space correctly (total - sum - overhead).
func TestComputeLayout_FreeSpace(t *testing.T) {
	shared.SetFakeDisks(true)
	defer shared.SetFakeDisks(false)

	layout, err := ComputeLayout("/dev/vda")
	if err != nil {
		t.Fatalf("ComputeLayout: %v", err)
	}

	if layout.Disk != "/dev/vda" {
		t.Errorf("layout.Disk = %q, want /dev/vda", layout.Disk)
	}
	if layout.DiskSizeMB != 40960 {
		t.Errorf("layout.DiskSizeMB = %d, want 40960", layout.DiskSizeMB)
	}

	var sumMB int64
	for _, p := range layout.Partitions {
		if p.Type != "" {
			t.Errorf("partition %s role = %q, want empty (untouched)", p.Path, p.Type)
		}
		sumMB += p.SizeMB
	}

	wantFree := layout.DiskSizeMB - sumMB
	if wantFree < 0 {
		wantFree = 0
	}
	if layout.FreeMB != wantFree {
		t.Errorf("layout.FreeMB = %d, want %d", layout.FreeMB, wantFree)
	}
}

// TestDeleteOrder verifies that partitions to delete are returned highest-first
// so deleting them leaves the numbers of the surviving partitions unchanged,
// and that rows without a real device path are ignored.
func TestDeleteOrder(t *testing.T) {
	tests := []struct {
		name    string
		deleted []shared.ManualPartition
		want    []int
	}{
		{
			name:    "empty",
			deleted: nil,
			want:    []int{},
		},
		{
			name:    "single",
			deleted: []shared.ManualPartition{{Path: "/dev/sda2"}},
			want:    []int{2},
		},
		{
			name: "multiple sorted highest-first",
			deleted: []shared.ManualPartition{
				{Path: "/dev/sda3"},
				{Path: "/dev/sda1"},
				{Path: "/dev/sda5"},
			},
			want: []int{5, 3, 1},
		},
		{
			name: "ignores rows without a device path",
			deleted: []shared.ManualPartition{
				{Path: ""},          // brand-new, never existed
				{Path: "/dev/vda4"}, // real
			},
			want: []int{4},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deleteOrder(tc.deleted)
			if len(got) != len(tc.want) {
				t.Fatalf("deleteOrder(%q) len = %d, want %d", tc.name, len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("deleteOrder(%q) = %v, want %v", tc.name, got, tc.want)
				}
			}
		})
	}
}
