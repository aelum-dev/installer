package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTools writes executable shims onto PATH so the lsblk/blockdev/findmnt
// shell-outs in disks.go see scripted output.
func fakeTools(t *testing.T, scripts map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// lsblkShim dispatches on the "-d" flag: the disk listing when present, the
// partition listing otherwise.
func lsblkShim(disksJSON, partsJSON string) string {
	return `for a in "$@"; do
  if [ "$a" = "-d" ]; then
    cat <<'JSON'
` + disksJSON + `
JSON
    exit 0
  fi
done
cat <<'JSON'
` + partsJSON + `
JSON
`
}

const (
	disksJSON = `{"blockdevices":[` +
		`{"name":"vda","type":"disk","size":"21474836480","model":"QEMU HARDDISK","vendor":"","rota":"0","ro":"0","fstype":"","mountpoint":"","serial":"QM1","tran":"virtio","children":[` +
		`{"name":"vda1","type":"part","size":"536870912","fstype":"vfat","mountpoint":"/boot"},` +
		`{"name":"vda2","type":"part","size":"20000000000","fstype":"btrfs","mountpoint":""}]},` +
		`{"name":"loop0","type":"disk","size":"1048576","model":"","vendor":"","rota":"0","ro":"0","fstype":"","mountpoint":"","serial":"","tran":""},` +
		`{"name":"sda","type":"disk","size":"1073741824000","model":"","vendor":"WD","rota":"1","ro":"1","fstype":"ext4","mountpoint":"/","serial":"S1","tran":"usb"},` +
		`{"name":"vda9","type":"part","size":"1024","model":"","vendor":"","rota":"0","ro":"0","fstype":"ext4","mountpoint":"/","serial":"","tran":""}]}`

	partsJSON = `{"blockdevices":[{"children":[` +
		`{"name":"vda1","type":"part","size":"536870912","fstype":"vfat","uuid":"EFI-UUID","mountpoint":"/boot"},` +
		`{"name":"vda2","type":"part","size":"20000000000","fstype":"ext4","uuid":"ROOT-UUID","mountpoint":""},` +
		`{"name":"vda3","type":"rom","size":"1024"}]}]}`

	btrfsPartsJSON = `{"blockdevices":[{"children":[` +
		`{"name":"vda1","type":"part","size":"536870912","fstype":"vfat","uuid":"EFI-UUID","mountpoint":"/boot"},` +
		`{"name":"vda2","type":"part","size":"20000000000","fstype":"btrfs","uuid":"ROOT-UUID","mountpoint":""}]}]}`
)

// --- GetDisks ----------------------------------------------------------------

func TestGetDisks_ParsesAndFilters(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": lsblkShim(disksJSON, partsJSON)})

	disks, err := GetDisks()
	if err != nil {
		t.Fatalf("GetDisks() = %v", err)
	}
	if len(disks) != 2 {
		t.Fatalf("GetDisks() returned %d disks, want 2 (loop0 and part filtered)", len(disks))
	}

	vda := disks[0]
	if vda.Path != "/dev/vda" || vda.Size != 21474836480 || vda.SizeFormatted != "20.0 GB" {
		t.Errorf("vda fields wrong: %+v", vda)
	}
	if vda.Fstype != "btrfs" {
		t.Errorf("vda fstype = %q, want btrfs (derived from children)", vda.Fstype)
	}
	if vda.Rotational || vda.Readonly || vda.IsRemovable {
		t.Errorf("vda flags wrong: %+v", vda)
	}
	if vda.Model != "QEMU HARDDISK" {
		t.Errorf("vda model = %q", vda.Model)
	}

	sda := disks[1]
	if sda.Model != "WD" {
		t.Errorf("sda model = %q, want WD (vendor fallback)", sda.Model)
	}
	if !sda.Rotational || !sda.Readonly || !sda.IsRemovable {
		t.Errorf("sda flags wrong: %+v", sda)
	}
	if sda.Fstype != "ext4" {
		t.Errorf("sda fstype = %q", sda.Fstype)
	}
}

// --- PartitionTableType ------------------------------------------------------

func TestPartitionTableType(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"GPT disk", "echo gpt\n", "gpt"},
		{"MBR disk (util-linux names it dos)", "echo dos\n", "dos"},
		{"no partition table (blkid exits 2)", "exit 2\n", ""},
		{"unreadable device", "echo boom >&2\nexit 1\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeTools(t, map[string]string{"blkid": tt.script})
			if got := PartitionTableType("/dev/vda"); got != tt.want {
				t.Errorf("PartitionTableType() = %q, want %q", got, tt.want)
			}
		})
	}
}

// GetDisks carries the table type to the UI so manual mode can refuse a disk
// before the user designs a plan sgdisk will not write.
func TestGetDisks_ReportsPartitionTable(t *testing.T) {
	fakeTools(t, map[string]string{
		"lsblk": lsblkShim(disksJSON, partsJSON),
		"blkid": `for a in "$@"; do
  if [ "$a" = "/dev/sda" ]; then echo dos; exit 0; fi
done
exit 2
`,
	})

	disks, err := GetDisks()
	if err != nil {
		t.Fatalf("GetDisks() = %v", err)
	}
	if len(disks) != 2 {
		t.Fatalf("GetDisks() returned %d disks, want 2", len(disks))
	}
	if disks[0].Path != "/dev/vda" || disks[0].PartitionTable != "" {
		t.Errorf("vda partition table = %q, want empty (no table reported)", disks[0].PartitionTable)
	}
	if disks[1].Path != "/dev/sda" || disks[1].PartitionTable != "dos" {
		t.Errorf("sda partition table = %q, want dos", disks[1].PartitionTable)
	}
}

func TestGetDisks_Error(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": "echo 'lsblk fail' >&2\nexit 1\n"})

	if _, err := GetDisks(); err == nil || !strings.Contains(err.Error(), "failed to run lsblk") {
		t.Fatalf("GetDisks() error = %v, want lsblk failure", err)
	}
}

func TestSetFakeDisks(t *testing.T) {
	old := fakeDisksEnabled
	t.Cleanup(func() { SetFakeDisks(old) })

	SetFakeDisks(true)
	disks, err := GetDisks()
	if err != nil {
		t.Fatalf("GetDisks() = %v", err)
	}
	if len(disks) != len(fakeDisks()) {
		t.Fatalf("GetDisks() returned %d fake disks, want %d", len(disks), len(fakeDisks()))
	}
	// Fake disks without a filesystem get one derived from their partitions.
	var sda *DiskInfo
	for i := range disks {
		if disks[i].Path == "/dev/sda" {
			sda = &disks[i]
		}
	}
	if sda == nil || sda.Fstype != "ext4" {
		t.Errorf("fake /dev/sda fstype = %v, want ext4", sda)
	}

	SetFakeDisks(false)
}

// --- GetPartitions -----------------------------------------------------------

func TestGetPartitions(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": lsblkShim(disksJSON, partsJSON)})

	parts, err := GetPartitions("/dev/vda")
	if err != nil {
		t.Fatalf("GetPartitions() = %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("GetPartitions() = %d entries, want 2 (rom skipped)", len(parts))
	}
	if parts[0].Path != "/dev/vda1" || parts[0].UUID != "EFI-UUID" || parts[0].Size != 536870912 {
		t.Errorf("parts[0] = %+v", parts[0])
	}
	if parts[1].Path != "/dev/vda2" || parts[1].Fstype != "ext4" {
		t.Errorf("parts[1] = %+v", parts[1])
	}
}

func TestGetPartitions_Error(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": "echo boom >&2\nexit 1\n"})

	if _, err := GetPartitions("/dev/vda"); err == nil || !strings.Contains(err.Error(), "failed to run lsblk") {
		t.Fatalf("GetPartitions() error = %v", err)
	}
}

func TestGetPartitions_NoDevices(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": lsblkShim(disksJSON, `{"blockdevices":[]}`)})

	if _, err := GetPartitions("/dev/vda"); err == nil || !strings.Contains(err.Error(), "no block devices found") {
		t.Fatalf("GetPartitions() error = %v", err)
	}
}

// --- GetDiskPartitions -------------------------------------------------------

func TestGetDiskPartitions_Plain(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": lsblkShim(disksJSON, partsJSON)})

	parts, err := GetDiskPartitions("/dev/vda")
	if err != nil {
		t.Fatalf("GetDiskPartitions() = %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("GetDiskPartitions() = %d entries, want 2", len(parts))
	}
	if len(parts[0].Mountpoints) != 1 || parts[0].Mountpoints[0] != "/boot" {
		t.Errorf("parts[0].Mountpoints = %v, want [/boot]", parts[0].Mountpoints)
	}
}

func TestGetDiskPartitions_BtrfsExpanded(t *testing.T) {
	findmnt := `cat <<'JSON'
{"filesystems":[{"target":"/"},{"target":"/home"},{"target":""}]}
JSON
`
	fakeTools(t, map[string]string{
		"lsblk":   lsblkShim(disksJSON, btrfsPartsJSON),
		"findmnt": findmnt,
	})

	parts, err := GetDiskPartitions("/dev/vda")
	if err != nil {
		t.Fatalf("GetDiskPartitions() = %v", err)
	}
	var root *PartitionEntry
	for i := range parts {
		if parts[i].Fstype == "btrfs" {
			root = &parts[i]
		}
	}
	if root == nil {
		t.Fatal("btrfs partition not found")
	}
	if len(root.Mountpoints) != 2 || root.Mountpoints[0] != "/" || root.Mountpoints[1] != "/home" {
		t.Errorf("btrfs Mountpoints = %v, want [/, /home]", root.Mountpoints)
	}
}

func TestGetDiskPartitions_BtrfsFindmntFailsFallsBack(t *testing.T) {
	fakeTools(t, map[string]string{
		"lsblk":   lsblkShim(disksJSON, btrfsPartsJSON),
		"findmnt": "exit 1\n",
	})

	parts, err := GetDiskPartitions("/dev/vda")
	if err != nil {
		t.Fatalf("GetDiskPartitions() = %v", err)
	}
	var root *PartitionEntry
	for i := range parts {
		if parts[i].Fstype == "btrfs" {
			root = &parts[i]
		}
	}
	if root == nil {
		t.Fatal("btrfs partition not found")
	}
	if len(root.Mountpoints) != 1 || root.Mountpoints[0] != "" {
		t.Errorf("Mountpoints = %v, want single empty fallback", root.Mountpoints)
	}
}

func TestGetDiskPartitions_Error(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": "echo boom >&2\nexit 1\n"})

	if _, err := GetDiskPartitions("/dev/vda"); err == nil || !strings.Contains(err.Error(), "lsblk failed") {
		t.Fatalf("GetDiskPartitions() error = %v", err)
	}
}

// --- LogicalSectorSize -------------------------------------------------------

func TestLogicalSectorSize(t *testing.T) {
	fakeTools(t, map[string]string{"blockdev": "echo 4096\n"})
	if got := LogicalSectorSize("/dev/nvme0n1"); got != 4096 {
		t.Errorf("LogicalSectorSize() = %d, want 4096", got)
	}
}

func TestLogicalSectorSize_Fallback(t *testing.T) {
	fakeTools(t, map[string]string{"blockdev": "exit 1\n"})
	if got := LogicalSectorSize("/dev/vda"); got != 512 {
		t.Errorf("LogicalSectorSize() = %d, want 512 fallback", got)
	}
}

// --- jsonSize.Int64 ----------------------------------------------------------

func TestJsonSize_Int64(t *testing.T) {
	tests := []struct {
		value string
		want  int64
	}{
		{"21474836480", 21474836480},
		{"0", 0},
		{"", 0},
		{"not-a-number", 0},
	}
	for _, tt := range tests {
		s := jsonSize{Value: tt.value}
		if got := s.Int64(); got != tt.want {
			t.Errorf("jsonSize{%q}.Int64() = %d, want %d", tt.value, got, tt.want)
		}
	}
}

// --- ValidateDiskSize --------------------------------------------------------

func TestValidateDiskSize(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"large enough", "echo 21474836480\n", ""},
		{"too small", "echo 1048576\n", "too small"},
		{"lsblk fails", "exit 1\n", "failed to get disk size"},
		{"unparseable", "echo nonsense\n", "failed to parse disk size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeTools(t, map[string]string{"lsblk": tt.script})
			err := ValidateDiskSize("/dev/vda")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDiskSize() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateDiskSize() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
