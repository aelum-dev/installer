package shared

import (
	"strings"
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{500107862016, "465.8 GB"},
	}
	for _, tt := range tests {
		if got := FormatSize(tt.bytes); got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

// TestDerivePartitionPath pins the disk-number separator rule: disks whose name
// ends in a digit get a "p" before the partition number (nvme0n1 -> nvme0n1p1),
// everything else appends directly (sda -> sda1).
func TestDerivePartitionPath(t *testing.T) {
	tests := []struct {
		disk string
		num  int
		want string
	}{
		{"/dev/sda", 1, "/dev/sda1"},
		{"/dev/vda", 2, "/dev/vda2"},
		{"/dev/nvme0n1", 1, "/dev/nvme0n1p1"},
		{"/dev/mmcblk0", 3, "/dev/mmcblk0p3"},
		{"/dev/loop0", 1, "/dev/loop0p1"},
		{"/dev/sdcard", 1, "/dev/sdcard1"},
	}
	for _, tt := range tests {
		if got := DerivePartitionPath(tt.disk, tt.num); got != tt.want {
			t.Errorf("DerivePartitionPath(%q, %d) = %q, want %q", tt.disk, tt.num, got, tt.want)
		}
	}
}

func TestIsVirtualDevice(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"loop0", true},
		{"/dev/loop0", true},
		{"ram1", true},
		{"dm-0", true},
		{"md0", true},
		{"sda", false},
		{"nvme0n1", false},
		{"vda", false},
	}
	for _, tt := range tests {
		if got := isVirtualDevice(tt.name); got != tt.want {
			t.Errorf("isVirtualDevice(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsRemovable(t *testing.T) {
	tests := []struct {
		trans string
		want  bool
	}{
		{"usb", true},
		{"sd", true},
		{"mmc", true},
		{"sata", false},
		{"nvme", false},
		{"virtio", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isRemovable(tt.trans); got != tt.want {
			t.Errorf("isRemovable(%q) = %v, want %v", tt.trans, got, tt.want)
		}
	}
}

// TestDeriveFstypeFromChildren covers the priority used to tag a disk with an
// "existing filesystem" before the user clicks it: a partition mounted at / wins,
// then the first btrfs/ext4, then the first typed partition.
func TestDeriveFstypeFromChildren(t *testing.T) {
	part := func(fstype, mountpoint string) lsblkEntry {
		return lsblkEntry{Type: "part", Fstype: fstype, Mountpoint: mountpoint}
	}
	tests := []struct {
		name     string
		children []lsblkEntry
		want     string
	}{
		{"root mount wins over earlier btrfs", []lsblkEntry{part("btrfs", ""), part("ext4", "/")}, "ext4"},
		{"first ext4/btrfs when no root mount", []lsblkEntry{part("ntfs", ""), part("btrfs", "")}, "btrfs"},
		{"falls back to first typed partition", []lsblkEntry{part("", ""), part("ntfs", "")}, "ntfs"},
		{"ignores non-partition children", []lsblkEntry{{Type: "disk", Fstype: "ext4"}, part("vfat", "")}, "vfat"},
		{"no typed partitions", []lsblkEntry{part("", "")}, ""},
		{"empty", nil, ""},
	}
	for _, tt := range tests {
		if got := deriveFstypeFromChildren(tt.children); got != tt.want {
			t.Errorf("%s: deriveFstypeFromChildren(...) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestContains(t *testing.T) {
	if !contains([]string{"a", "b", "/"}, "/") {
		t.Error("contains should find a present value")
	}
	if contains([]string{"a", "b"}, "/") {
		t.Error("contains should not find an absent value")
	}
	if contains(nil, "x") {
		t.Error("contains(nil) should be false")
	}
}

func TestLogicalSectorSize_FakeMode(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")
	if got := LogicalSectorSize("/dev/vda"); got != 512 {
		t.Errorf("LogicalSectorSize in fake mode = %d, want 512", got)
	}
}

// Fake disks report a GPT so the manual-mode partition-table check never fires in
// UI/dry-run mode, where no real device is being read.
func TestPartitionTableType_FakeMode(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")
	if got := PartitionTableType("/dev/vda"); got != PartitionTableGPT {
		t.Errorf("PartitionTableType in fake mode = %q, want %q", got, PartitionTableGPT)
	}
}

func TestFakeDisks(t *testing.T) {
	disks := fakeDisks()
	if len(disks) == 0 {
		t.Fatal("fakeDisks returned no disks")
	}
	for _, d := range disks {
		if !strings.HasPrefix(d.Path, "/dev/") {
			t.Errorf("disk %q path %q is not under /dev", d.Name, d.Path)
		}
		if d.Size <= 0 {
			t.Errorf("disk %q has non-positive size %d", d.Name, d.Size)
		}
		if d.SizeFormatted == "" {
			t.Errorf("disk %q has empty SizeFormatted", d.Name)
		}
	}
}

func TestFakePartitions(t *testing.T) {
	parts := fakePartitions("/dev/vda")
	if len(parts) != 2 {
		t.Fatalf("fakePartitions(/dev/vda) returned %d partitions, want 2", len(parts))
	}
	if parts[0].Fstype != "fat32" || parts[1].Fstype != "ext4" {
		t.Errorf("unexpected fake partition types: %+v", parts)
	}
	if got := fakePartitions("/dev/does-not-exist"); len(got) != 0 {
		t.Errorf("unknown disk should have no fake partitions, got %d", len(got))
	}
}

func TestDeriveFstypeFromFakePartitions(t *testing.T) {
	tests := []struct {
		disk string
		want string
	}{
		{"/dev/sda", "ext4"},      // root mount wins
		{"/dev/nvme1n1", "btrfs"}, // no root mount; btrfs preferred
		{"/dev/sdb", "ntfs"},      // only ntfs present
		{"/dev/sdc", "vfat"},      // vfat fallback
		{"/dev/unknown", ""},      // no fake partitions
	}
	for _, tt := range tests {
		if got := deriveFstypeFromFakePartitions(tt.disk); got != tt.want {
			t.Errorf("deriveFstypeFromFakePartitions(%q) = %q, want %q", tt.disk, got, tt.want)
		}
	}
}

// TestGetDisks_FakeMode checks the FAKE_DISKS=1 path and that disks without an
// explicit fstype inherit one from their fake partitions.
func TestGetDisks_FakeMode(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")

	disks, err := GetDisks()
	if err != nil {
		t.Fatalf("GetDisks: %v", err)
	}
	if len(disks) == 0 {
		t.Fatal("GetDisks returned no fake disks")
	}

	byName := make(map[string]DiskInfo, len(disks))
	for _, d := range disks {
		byName[d.Name] = d
	}

	// vda ships without a top-level fstype, so it must be derived from its
	// fake partitions (ext4 root).
	vda, ok := byName["vda"]
	if !ok {
		t.Fatal("fake vda disk missing")
	}
	if vda.Fstype != "ext4" {
		t.Errorf("vda Fstype = %q, want ext4 (derived from fake partitions)", vda.Fstype)
	}
}
