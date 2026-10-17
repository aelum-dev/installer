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
				{Index: 0, Path: "/dev/sda1", SizeMB: 512, Type: shared.TypeESP},
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
			// Regression: a fully-partitioned disk where existing rows leave fewer
			// free sectors than the trailing GPT backup region must not be rejected
			// by the strict new-partition guard (newSectors == 0).
			name: "existing partitions fill disk, free < gpt backup",
			layout: shared.ManualLayout{
				Disk:          "/dev/nvme0n1",
				DiskSizeMB:    488387,
				DiskSizeBytes: 512110984960,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/nvme0n1p1", SizeMB: 512, Type: shared.TypeESP},
					{Path: "/dev/nvme0n1p2", SizeMB: 487875, Type: shared.TypeSystem},
				},
			},
			wantErr: false,
		},
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
					{Path: "/dev/sda1", SizeMB: 512, Type: shared.TypeESP},
					{Path: "/dev/sda2", SizeMB: 512, Type: shared.TypeESP},
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
					{Path: "/dev/sda1", SizeMB: 512, Type: shared.TypeESP},
				},
			},
			wantErr: true,
		},
		{
			name: "two roots",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 512, Type: shared.TypeESP},
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
					{Path: "/dev/sda1", SizeMB: 512, Type: shared.PartitionType("swap")},
					{Path: "/dev/sda2", SizeMB: 512, Type: shared.TypeESP},
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
			{Index: 2, Path: "", SizeMB: 512, Type: shared.TypeESP},
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
			{Index: 0, Path: "/dev/sda1", SizeMB: 512, Type: shared.TypeESP},
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
	// That yields 99486 MiB rather than the uncapped 100000-512 = 99488.
	want := (100000*2048 - (gptBackupBytes / 512) - 1050624) / 2048
	if root.sizeMB != want {
		t.Errorf("root sizeMB = %d, want %d", root.sizeMB, want)
	}
	// The partition must fit inside the disk after reserving the backup GPT.
	rootEndSector := int64(1050624) + root.sizeMB*2048
	if rootEndSector > 100000*2048-(gptBackupBytes/512) {
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
	if systemEndSector > diskSectors-(gptBackupBytes/512) {
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

func TestValidate_NewPartitionRespectsFreeSectors(t *testing.T) {
	// Free space is reported exactly (FreeSectors). A brand-new explicit root that
	// fits the coarse MiB sum but exceeds the true free sectors must be rejected —
	// this is the truncation bug: counting truncated MB overstates the free space.
	const mib = 1024 * 1024
	layout := shared.ManualLayout{
		Disk:          "/dev/sda",
		DiskSizeMB:    100000,
		DiskSizeBytes: 100000 * mib,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/sda1", SizeMB: 512, SizeBytes: 512 * mib, Type: shared.TypeESP},
			{Index: 1, Path: "/dev/sda2", SizeMB: 90000, SizeBytes: 90000*mib + 500000, Type: shared.TypeSystem},
			{Index: 2, Path: "", SizeMB: 9488, Type: shared.TypeSystem},
		},
	}
	usedBytes := 512*mib + (90000*mib + 500000)
	layout.FreeSectors = int64((100000*mib - usedBytes) / 512)

	// Coarse sum = 512+90000+9488 = 100000 == DiskSizeMB, so it passes that guard.
	if sum := layout.Partitions[0].SizeMB + layout.Partitions[1].SizeMB + layout.Partitions[2].SizeMB; sum > layout.DiskSizeMB {
		t.Fatalf("test setup broke its own coarse guard: sum %d > %d", sum, layout.DiskSizeMB)
	}
	// But the true free space is under 9872 MiB, so this must fail.
	if err := Validate(layout); err == nil {
		t.Errorf("expected overflow error: new partition %d MB exceeds true free space", layout.Partitions[2].SizeMB)
	}
}

// TestValidate_MinPartitionSizes checks the enforced floor for designated
// partitions: an ESP must be at least 512 MiB and the system partition at least
// 8 GiB (the disk floor). A "use all remaining space" row is checked against the
// resolved free space in TestValidate_UseRemainingRespectsFloor.
func TestValidate_MinPartitionSizes(t *testing.T) {
	const mib = 1024 * 1024
	minESP := int64(shared.MinESPSizeBytes / mib)
	minSys := int64(shared.MinSystemPartitionBytes / mib) // 8192
	tooSmallESP := int64(shared.MinESPSizeBytes / mib / 2)

	make := func(esp, sys shared.ManualPartition) shared.ManualLayout {
		return shared.ManualLayout{
			Disk:          "/dev/sda",
			DiskSizeMB:    100000,
			DiskSizeBytes: 100000 * mib,
			Partitions:    []shared.ManualPartition{esp, sys},
		}
	}

	esp := func(sizeMB int64) shared.ManualPartition {
		return shared.ManualPartition{Index: 0, Path: "/dev/sda1", SizeMB: sizeMB, SizeBytes: sizeMB * mib, Type: shared.TypeESP}
	}
	sys := func(sizeMB int64, brandNew bool) shared.ManualPartition {
		part := shared.ManualPartition{Index: 1, SizeMB: sizeMB, Type: shared.TypeSystem}
		if !brandNew {
			part.Path = "/dev/sda2"
			part.SizeBytes = sizeMB * mib
		}
		return part
	}

	tests := []struct {
		name    string
		esp     shared.ManualPartition
		sys     shared.ManualPartition
		wantErr bool
	}{
		{"system exactly 8 GiB", esp(minESP), sys(minSys, false), false},
		{"existing system below 8 GiB", esp(minESP), sys(minSys-1, false), true},
		{"brand-new system below 8 GiB", esp(minESP), sys(4096, true), true},
		{"system use-remaining resolves above floor", esp(minESP), sys(0, true), false},
		{"esp below 512 MiB", esp(tooSmallESP), sys(minSys, false), true},
		{"esp exactly 512 MiB", esp(minESP), sys(minSys, false), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(make(tc.esp, tc.sys)); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestValidate_UseRemainingRespectsFloor guards the resolved-size floor for a
// brand-new "use all remaining space" row: Validate cannot know its final size
// during the per-row pass, so it must check the remaining free space afterwards.
func TestValidate_UseRemainingRespectsFloor(t *testing.T) {
	const mib = int64(1024 * 1024)

	tests := []struct {
		name    string
		layout  shared.ManualLayout
		wantErr bool
	}{
		{
			name: "new ESP use-remaining below 512 MiB",
			layout: shared.ManualLayout{
				Disk: "/dev/vda", DiskSizeMB: 9000, DiskSizeBytes: 9000 * mib,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/vda1", SizeMB: 8700, SizeBytes: 8700 * mib, Type: shared.TypeSystem},
					{Path: "", SizeMB: 0, Type: shared.TypeESP},
				},
			},
			wantErr: true,
		},
		{
			name: "new System use-remaining below 8 GiB",
			layout: shared.ManualLayout{
				Disk: "/dev/vda", DiskSizeMB: 8700, DiskSizeBytes: 8700 * mib,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * mib, Type: shared.TypeESP},
					{Path: "", SizeMB: 0, Type: shared.TypeSystem},
				},
			},
			wantErr: true,
		},
		{
			name: "new ESP use-remaining meets the floor",
			layout: shared.ManualLayout{
				Disk: "/dev/vda", DiskSizeMB: 9000, DiskSizeBytes: 9000 * mib,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/vda1", SizeMB: 8192, SizeBytes: 8192 * mib, Type: shared.TypeSystem},
					{Path: "", SizeMB: 0, Type: shared.TypeESP},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.layout); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// TestBuildActions_UseRemainingRespectsFloor is the last-line defence: even if a
// layout reaches Apply without the coarse check, a use-remaining row that
// resolves below its role floor must not be created.
func TestBuildActions_UseRemainingRespectsFloor(t *testing.T) {
	const mib = int64(1024 * 1024)

	tests := []struct {
		name    string
		layout  shared.ManualLayout
		sectors []partitionSector
	}{
		{
			name: "ESP",
			layout: shared.ManualLayout{
				Disk: "/dev/vda", DiskSizeMB: 300, DiskSizeBytes: 300 * mib,
				Partitions: []shared.ManualPartition{
					{Path: "", SizeMB: 0, Type: shared.TypeESP},
				},
			},
		},
		{
			name: "System",
			layout: shared.ManualLayout{
				Disk: "/dev/vda", DiskSizeMB: 8700, DiskSizeBytes: 8700 * mib,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * mib, Type: shared.TypeESP},
					{Path: "", SizeMB: 0, Type: shared.TypeSystem},
				},
			},
			sectors: []partitionSector{{num: 1, start: 2048, end: 2048 + 512*2048 - 1}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildActions("/dev/vda", &tc.layout, shared.FileSystemExt4, tc.sectors); err == nil {
				t.Error("buildActions accepted a use-remaining row below its role floor")
			}
		})
	}
}

// TestValidate_NewPartitionFreesSpaceFromDeletedRow reproduces a regression
// where Validate used layout.FreeSectors (stamped by ComputeLayout at disk-load
// time and never updated after edits) for its strict new-partition check. When
// the user deletes an existing partition and adds a brand-new one in its place,
// the free space is larger than the stale value reflects, so the old code
// rejected a plan the frontend itself considered valid.
func TestValidate_NewPartitionFreesSpaceFromDeletedRow(t *testing.T) {
	const mib = 1024 * 1024
	diskMB := int64(20480)
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    diskMB,
		DiskSizeBytes: diskMB * mib,
		Partitions: []shared.ManualPartition{
			// Existing ESP kept verbatim.
			{Index: 0, Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * mib, Type: shared.TypeESP},
			// Brand-new System replacing a partition the user deleted (deletedPartitions
			// are excluded from Partitions, so their space is genuinely freed).
			{Index: 1, Path: "", SizeMB: 19960, Type: shared.TypeSystem},
		},
	}
	// Preserve the stale load-time free space (only ~1 MB really free today) to
	// prove the check is recomputed from the live plan, not read from FreeSectors.
	layout.FreeSectors = 2081

	if err := Validate(layout); err != nil {
		t.Fatalf("expected the replacement partition to fit the freed space, got: %v", err)
	}
}

func TestValidate_UseAllOrdering(t *testing.T) {
	// A brand-new "use all remaining space" partition must not be followed by
	// another brand-new partition: the first would consume the disk.
	layout := shared.ManualLayout{
		Disk:       "/dev/sda",
		DiskSizeMB: 100000,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/sda1", SizeMB: 512, Type: shared.TypeESP},
			{Index: 1, Path: "", SizeMB: 0, Type: shared.TypeSystem},
			{Index: 2, Path: "", SizeMB: 500, Type: shared.TypeSystem},
		},
	}
	if err := Validate(layout); err == nil {
		t.Error("expected ordering error: a new partition cannot follow a use-all partition")
	}
}

func TestBuildActions_ExplicitNewPartitionTooLargeErrors(t *testing.T) {
	// A brand-new explicit partition larger than the true free space must fail
	// loudly instead of being silently clamped to a smaller size — clamping would
	// install a different layout than the user asked for (and Validate already
	// rejects this request, so buildActions must agree rather than paper over it).
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    100000,
		DiskSizeBytes: 100000 * 1024 * 1024,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 512, Type: shared.TypeESP},
			{Index: 1, Path: "", SizeMB: 99999, Type: shared.TypeSystem},
		},
	}
	// ESP occupies the first 512 MiB of the disk.
	sectors := []partitionSector{{num: 1, start: 2048, end: 2048 + 512*2048 - 1}}
	if _, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, sectors); err == nil {
		t.Error("expected an error for an explicit partition larger than the free space, got nil (silent clamp)")
	}
}

// TestBuildActions_NewPartitionFillsEarlyGap: a plan whose last existing
// partition reaches the disk end but whose deleted predecessor freed space at
// the front. The new partition must be carved into that front gap, not appended
// past the disk end.
func TestBuildActions_NewPartitionFillsEarlyGap(t *testing.T) {
	// 20 GiB disk. vda2 (system) sits at the very end, leaving a ~2 GiB gap at
	// the front (where a deleted EFI used to be) and only a sliver after it.
	const (
		diskSectors = int64(41943040) // 20 GiB / 512
		lastSector  = diskSectors - (gptBackupBytes / 512)
	)
	sectors := []partitionSector{
		{num: 2, start: 4237056, end: 41940991}, // system, ends near the disk end
	}
	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    20480,
		DiskSizeBytes: 20480 * 1024 * 1024,
		Partitions: []shared.ManualPartition{
			{Index: 1, Path: "/dev/vda2", SizeMB: 18398, Type: shared.TypeSystem},
			{Index: 2, Path: "", SizeMB: 2048, Type: shared.TypeESP}, // new EFI
		},
	}

	actions, err := buildActions("/dev/vda", &layout, shared.FileSystemVfat, sectors)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}

	var esp *formatAction
	for i := range actions {
		if actions[i].isNew && actions[i].pType == shared.TypeESP {
			esp = &actions[i]
		}
	}
	if esp == nil {
		t.Fatal("no new ESP action produced")
	}

	// It must land in the front gap, not after vda2 where there is no room.
	wantStart := int64(2048)
	if esp.startSector != wantStart {
		t.Errorf("ESP start sector = %d, want %d (the freed front gap)", esp.startSector, wantStart)
	}
	if esp.startSector >= sectors[0].start {
		t.Errorf("ESP starts at %d, after the last partition at %d — would run past the disk", esp.startSector, sectors[0].start)
	}

	// It must not overlap the kept system partition, and must stay usable.
	if esp.startSector+esp.sizeMB*2048 > sectors[0].start {
		t.Errorf("ESP end sector %d overlaps system partition starting at %d", esp.startSector+esp.sizeMB*2048, sectors[0].start)
	}
	if esp.startSector+esp.sizeMB*2048-1 > lastSector {
		t.Errorf("ESP end sector %d exceeds last usable sector %d", esp.startSector+esp.sizeMB*2048-1, lastSector)
	}
	// And it keeps the next available GPT number (3), which the kernel exposes
	// as /dev/vda3 regardless of its physical position.
	if esp.num != 3 {
		t.Errorf("ESP num = %d, want 3", esp.num)
	}
}

// TestBuildActions_FragmentedExplicitSizeErrors reproduces the silent-clamp bug:
// the free space is large in total (14399 MiB) but split into two gaps (7000 and
// ~7398 MiB), so no single gap fits a requested 14398 MiB root. The clamped
// result (~7398 MiB) would still meet the 8 GiB floor, so the old code quietly
// created a much smaller root than requested. buildActions must refuse instead.
func TestBuildActions_FragmentedExplicitSizeErrors(t *testing.T) {
	const mib = int64(1024 * 1024)
	diskMB := int64(40000)

	vda1Start := int64(2048)
	vda1Len := int64(5120) * 2048 // 5 GiB
	vda1End := vda1Start + vda1Len - 1
	vda2Start := vda1End + 1 + int64(7000)*2048 // 7 GiB front gap
	vda2Len := int64(20480) * 2048              // 20 GiB
	vda2End := vda2Start + vda2Len - 1

	sectors := []partitionSector{
		{num: 1, start: vda1Start, end: vda1End},
		{num: 2, start: vda2Start, end: vda2End},
	}
	layout := shared.ManualLayout{
		Disk: "/dev/vda", DiskSizeMB: diskMB, DiskSizeBytes: diskMB * mib,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 5120, SizeBytes: 5120 * mib, Type: shared.TypeESP},
			{Index: 1, Path: "/dev/vda2", SizeMB: 20480, SizeBytes: 20480 * mib},
			{Index: 2, Path: "", SizeMB: 14398, Type: shared.TypeSystem}, // fits in total, no single gap
		},
	}

	// Validate must pass: the request fits the total free space, so the bug is
	// invisible to a totals-only check. If this fails the setup is wrong.
	if err := Validate(layout); err != nil {
		t.Fatalf("setup: Validate should pass (total free covers the request): %v", err)
	}
	if _, err := buildActions("/dev/vda", &layout, shared.FileSystemExt4, sectors); err == nil {
		t.Error("buildActions: expected an error (request fits in total but no single gap), got a silent clamp")
	}
}

// TestFeasible_DeletedPartitionFreesSpace confirms the dry-run check models the
// space freed by deleting a partition: the freed 20 GiB region must accept the
// replacement root, while the same layout without the delete is infeasible.
func TestFeasible_DeletedPartitionFreesSpace(t *testing.T) {
	const mib = int64(1024 * 1024)
	diskMB := int64(40000)

	vda1Start := int64(2048)
	vda1Len := int64(5120) * 2048
	vda1End := vda1Start + vda1Len - 1
	vda2Start := vda1End + 1 + int64(7000)*2048
	vda2Len := int64(20480) * 2048
	vda2End := vda2Start + vda2Len - 1

	sectors := []partitionSector{
		{num: 1, start: vda1Start, end: vda1End},
		{num: 2, start: vda2Start, end: vda2End},
	}

	layout := shared.ManualLayout{
		Disk: "/dev/vda", DiskSizeMB: diskMB, DiskSizeBytes: diskMB * mib,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 5120, SizeBytes: 5120 * mib, Type: shared.TypeESP},
			{Index: 1, Path: "", SizeMB: 20000, Type: shared.TypeSystem}, // replaces vda2
		},
		DeletedPartitions: []shared.ManualPartition{{Index: 1, Path: "/dev/vda2"}},
	}

	if err := Validate(layout); err != nil {
		t.Fatalf("setup: Validate should pass: %v", err)
	}
	if err := feasible(layout, sectors); err != nil {
		t.Errorf("feasible: expected the replacement root to fit vda2's freed space, got: %v", err)
	}
}

// TestBuildActions_4KnSectorSize checks the placement math on a 4Kn disk, whose
// logical sector is 4096 bytes: 1 MiB is 256 sectors, not 2048, so alignment and
// sizing must scale with the disk's reported sector size.
func TestBuildActions_4KnSectorSize(t *testing.T) {
	const ss = int64(4096)
	const mib = int64(1024 * 1024)
	spm := mib / ss // 256 sectors per MiB

	diskMB := int64(20480) // 20 GiB
	layout := &shared.ManualLayout{
		Disk:            "/dev/nvme0n1",
		DiskSizeMB:      diskMB,
		DiskSizeBytes:   diskMB * mib,
		SectorSizeBytes: ss,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "", SizeMB: shared.EFIPartitionSizeMB, Type: shared.TypeESP},
			{Index: 1, Path: "", SizeMB: 0, Type: shared.TypeSystem},
		},
	}

	actions, err := buildActions("/dev/nvme0n1", layout, shared.FileSystemExt4, nil)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	esp := actions[0]
	// First usable sector on 4Kn is 1 MiB = 256 sectors.
	if wantStart := mib / ss; esp.startSector != wantStart {
		t.Errorf("4Kn ESP start sector = %d, want %d", esp.startSector, wantStart)
	}
	if esp.startSector%spm != 0 {
		t.Errorf("4Kn ESP start sector %d is not MiB-aligned (multiple of %d)", esp.startSector, spm)
	}

	root := actions[1]
	wantRootStart := esp.startSector + shared.EFIPartitionSizeMB*mib/ss
	if root.startSector != wantRootStart {
		t.Errorf("4Kn root start sector = %d, want %d (contiguous after ESP)", root.startSector, wantRootStart)
	}
	lastSector := layout.DiskSizeBytes/ss - backupSectors(ss)
	if root.startSector+root.sizeMB*spm > lastSector {
		t.Errorf("4Kn root end %d exceeds last usable sector %d", root.startSector+root.sizeMB*spm, lastSector)
	}
}

// TestValidate_RejectsRequestThatOnlyFitsWithGPTPrefix guards the prefix
// accounting fix: a brand-new partition sized to consume the total free space
// computed from sector 0 (i.e. including the unusable 1 MiB GPT prefix plus the
// backup region) must be rejected — the old code, which only reserved the backup
// region, accepted it.
func TestValidate_RejectsRequestThatOnlyFitsWithGPTPrefix(t *testing.T) {
	const mib = int64(1024 * 1024)
	diskMB := int64(100000)
	layout := shared.ManualLayout{
		Disk: "/dev/vda", DiskSizeMB: diskMB, DiskSizeBytes: diskMB * mib,
		Partitions: []shared.ManualPartition{
			{Index: 0, Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * mib, Type: shared.TypeESP},
			// Exactly total-free-minus-1 MiB: the old check allowed this by
			// counting the 1 MiB GPT prefix as allocatable.
			{Index: 1, Path: "", SizeMB: diskMB - 512 - 1, Type: shared.TypeSystem},
		},
	}
	if err := Validate(layout); err == nil {
		t.Error("expected rejection: the request only fits if the unusable GPT prefix is counted as free")
	}
}
