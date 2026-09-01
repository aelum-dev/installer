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
				{Index: 0, Path: "/dev/sda1", SizeMB: 128, Role: shared.RoleBoot},
				{Index: 1, Path: "/dev/sda2", SizeMB: 50000, Role: shared.RoleRoot},
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
					{Path: "/dev/sda2", SizeMB: 50000, Role: shared.RoleRoot},
				},
			},
			wantErr: true,
		},
		{
			name: "two boots",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Role: shared.RoleBoot},
					{Path: "/dev/sda2", SizeMB: 128, Role: shared.RoleBoot},
					{Path: "/dev/sda3", SizeMB: 50000, Role: shared.RoleRoot},
				},
			},
			wantErr: true,
		},
		{
			name: "no root",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Role: shared.RoleBoot},
				},
			},
			wantErr: true,
		},
		{
			name: "two roots",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Role: shared.RoleBoot},
					{Path: "/dev/sda2", SizeMB: 50000, Role: shared.RoleRoot},
					{Path: "/dev/sda3", SizeMB: 50000, Role: shared.RoleRoot},
				},
			},
			wantErr: true,
		},
		{
			name: "over size",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 60000, Role: shared.RoleBoot},
					{Path: "/dev/sda2", SizeMB: 60000, Role: shared.RoleRoot},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple use-all",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 0, Role: shared.RoleBoot},
					{Path: "/dev/sda2", SizeMB: 0, Role: shared.RoleRoot},
				},
			},
			wantErr: true,
		},
		{
			name: "unknown role",
			layout: shared.ManualLayout{
				DiskSizeMB: 100000,
				Partitions: []shared.ManualPartition{
					{Path: "/dev/sda1", SizeMB: 128, Role: shared.PartitionRole("swap")},
					{Path: "/dev/sda2", SizeMB: 128, Role: shared.RoleBoot},
					{Path: "/dev/sda3", SizeMB: 50000, Role: shared.RoleRoot},
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
			{Index: 1, Path: "/dev/sda3", SizeMB: 20000, Role: shared.RoleRoot, ExistingFstype: "ext4"},
			{Index: 2, Path: "", SizeMB: 128, Role: shared.RoleBoot},
		},
	}

	actions, err := buildActions("/dev/sda", &layout, shared.FileSystemExt4, sectors)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}

	if len(actions) != 2 {
		t.Fatalf("expected 2 format actions (Boot + Root), got %d", len(actions))
	}

	var boot, root *formatAction
	for i := range actions {
		if actions[i].role == shared.RoleBoot {
			boot = &actions[i]
		}
		if actions[i].role == shared.RoleRoot {
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
			{Index: 0, Path: "/dev/sda1", SizeMB: 128, Role: shared.RoleBoot},
			{Index: 1, Path: "", SizeMB: 0, Role: shared.RoleRoot},
		},
	}

	actions, err := buildActions("/dev/sda", &layout, shared.FileSystemBtrfs, sectors)
	if err != nil {
		t.Fatalf("buildActions: %v", err)
	}

	var root *formatAction
	for i := range actions {
		if actions[i].role == shared.RoleRoot {
			root = &actions[i]
		}
	}
	if root == nil {
		t.Fatal("no root action")
	}
	if root.fstype != shared.FileSystemBtrfs {
		t.Errorf("root fstype = %q, want %q", root.fstype, shared.FileSystemBtrfs)
	}
	// Root uses "all remaining": disk - boot - overhead.
	if root.sizeMB != 100000-128-shared.GPTOverheadMB {
		t.Errorf("root sizeMB = %d, want %d", root.sizeMB, 100000-128-shared.GPTOverheadMB)
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
		if p.Role != "" {
			t.Errorf("partition %s role = %q, want empty (untouched)", p.Path, p.Role)
		}
		sumMB += p.SizeMB
	}

	wantFree := layout.DiskSizeMB - sumMB - shared.GPTOverheadMB
	if wantFree < 0 {
		wantFree = 0
	}
	if layout.FreeMB != wantFree {
		t.Errorf("layout.FreeMB = %d, want %d", layout.FreeMB, wantFree)
	}
}

func TestGPTOverheadBytes(t *testing.T) {
	if GPTOverheadBytes != int64(shared.GPTOverheadMB*1024*1024) {
		t.Errorf("GPTOverheadBytes = %d, want %d", GPTOverheadBytes, shared.GPTOverheadMB*1024*1024)
	}
}
