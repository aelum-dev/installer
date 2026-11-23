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

// lsblkShim dispatches on the arguments: a call naming a /dev/ path is a
// per-disk listing (GetPartitions/GetDiskPartitions and the in-use probe),
// anything else is the full tree GetDisks asks for.
func lsblkShim(disksJSON, partsJSON string) string {
	return `for a in "$@"; do
  case "$a" in
    /dev/*)
      cat <<'JSON'
` + partsJSON + `
JSON
      exit 0
      ;;
  esac
done
cat <<'JSON'
` + disksJSON + `
JSON
`
}

// fakeMountinfo points the live-device check's mount table at a fixture, so
// live-detection tests never depend on what the test host has mounted. The
// fixture lines follow mountinfo(8): "ID parent major:minor root mount-point
// options… - fstype source super-options" — the check reads fields 3 and 5.
func fakeMountinfo(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fake mountinfo: %v", err)
	}
	old := procMountsPath
	procMountsPath = path
	t.Cleanup(func() { procMountsPath = old })
}

const (
	disksJSON = `{"blockdevices":[` +
		// vda: only the installer's own mounts (target root + ESP) → not in use.
		`{"name":"vda","type":"disk","size":"21474836480","model":"QEMU HARDDISK","vendor":"","rota":"0","ro":"0","fstype":"","mountpoint":"","fsused":null,"maj:min":"253:0","serial":"QM1","tran":"virtio","children":[` +
		`{"name":"vda1","type":"part","size":"536870912","fstype":"vfat","mountpoint":"/mnt/efi","fsused":1048576,"maj:min":"253:1"},` +
		`{"name":"vda2","type":"part","size":"20000000000","fstype":"btrfs","mountpoint":"/mnt","fsused":null,"maj:min":"253:2"}]},` +
		`{"name":"loop0","type":"disk","size":"1048576","model":"","vendor":"","rota":"0","ro":"0","fstype":"","mountpoint":"","fsused":null,"maj:min":"7:0","serial":"","tran":""},` +
		`{"name":"zram0","type":"disk","size":"8589934592","model":"","vendor":"","rota":"0","ro":"0","fstype":"swap","mountpoint":"[SWAP]","fsused":null,"maj:min":"252:0","serial":"","tran":""},` +
		// sda: the running system's own disk — mounted at / and named by the
		// fixture mount table's root mount.
		`{"name":"sda","type":"disk","size":"1073741824000","model":"","vendor":"WD","rota":"1","ro":"1","fstype":"ext4","mountpoint":"/","fsused":500000000000,"maj:min":"8:16","serial":"S1","tran":"usb"},` +
		// sdb: in use without a visible foreign mount — FSUSED set with no
		// mount point (btrfs mounted where the column cannot show it), plus an
		// active swap area.
		`{"name":"sdb","type":"disk","size":"500107862016","model":"","vendor":"","rota":"1","ro":"0","fstype":"","mountpoint":"","fsused":null,"maj:min":"8:48","serial":"","tran":"sata","children":[` +
		`{"name":"sdb1","type":"part","size":"490000000000","fstype":"btrfs","mountpoint":null,"fsused":4096,"maj:min":"8:49"},` +
		`{"name":"sdb2","type":"part","size":"8589934592","fstype":"swap","mountpoint":"[SWAP]","fsused":null,"maj:min":"8:50"}]},` +
		`{"name":"vda9","type":"part","size":"1024","model":"","vendor":"","rota":"0","ro":"0","fstype":"ext4","mountpoint":"/","fsused":null,"maj:min":"253:9","serial":"","tran":""}]}`

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
	fakeMountinfo(t, "22 1 8:16 / / rw,relatime - ext4 /dev/sda1 rw\n"+
		"34 1 8:33 / /run/archiso/bootmnt rw - iso9660 /dev/sr0 ro\n")

	disks, err := GetDisks()
	if err != nil {
		t.Fatalf("GetDisks() = %v", err)
	}
	if len(disks) != 3 {
		t.Fatalf("GetDisks() returned %d disks, want 3 (loop0, zram0 and the stray part filtered)", len(disks))
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
	// Only /mnt and /mnt/efi are mounted on it — the installer's own target
	// mounts — so it stays selectable.
	if vda.InUse || vda.IsLiveDevice {
		t.Errorf("vda in-use flags wrong: %+v", vda)
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
	// Mounted at / and named by the mount table's root entry: in use AND the
	// live system's disk.
	if !sda.InUse || !sda.IsLiveDevice {
		t.Errorf("sda in-use flags wrong: %+v", sda)
	}

	sdb := disks[2]
	// No foreign mount point is printed for it, but one partition reports
	// FSUSED without a mount and the other is active swap.
	if !sdb.InUse || sdb.IsLiveDevice {
		t.Errorf("sdb in-use flags wrong: %+v", sdb)
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
	fakeMountinfo(t, "22 1 8:16 / / rw,relatime - ext4 /dev/sda1 rw\n")

	disks, err := GetDisks()
	if err != nil {
		t.Fatalf("GetDisks() = %v", err)
	}
	if len(disks) != 3 {
		t.Fatalf("GetDisks() returned %d disks, want 3", len(disks))
	}
	if disks[0].Path != "/dev/vda" || disks[0].PartitionTable != "" {
		t.Errorf("vda partition table = %q, want empty (no table reported)", disks[0].PartitionTable)
	}
	if disks[1].Path != "/dev/sda" || disks[1].PartitionTable != "dos" {
		t.Errorf("sda partition table = %q, want dos", disks[1].PartitionTable)
	}
}

// --- live-device detection ----------------------------------------------------

func TestLiveSystemMajMin(t *testing.T) {
	fakeMountinfo(t, `22 1 8:16 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
24 22 0:44 / /proc rw shared:2 - proc proc rw
34 1 8:33 / /run/archiso/bootmnt rw shared:3 - iso9660 /dev/sr0 ro
36 1 0:41 / /run/archiso/airootfs ro shared:4 - tmpfs tmpfs rw
38 22 8:49 / /home/shared\040disk rw - ext4 /dev/sdb1 rw
`)
	live := liveSystemMajMin()
	if !live["8:16"] || !live["8:33"] {
		t.Fatalf("live set = %v, want 8:16 (/) and 8:33 (bootmnt)", live)
	}
	if len(live) != 2 {
		t.Errorf("live set = %v, want exactly the / and bootmnt devices — tmpfs and unrelated mounts must not appear", live)
	}
}

func TestUnescapeMountPath(t *testing.T) {
	if got := unescapeMountPath(`/home/shared\040disk`); got != "/home/shared disk" {
		t.Errorf("unescapeMountPath() = %q, want the decoded path", got)
	}
	if got := unescapeMountPath(`/plain/path`); got != "/plain/path" {
		t.Errorf("unescapeMountPath() = %q, want the path unchanged", got)
	}
}

func TestSubtreeInUse(t *testing.T) {
	// Every mount point the installer itself creates on a target during a run
	// is its own work, not "the system using the disk".
	ours := []string{"/mnt", "/mnt/efi", "/mnt/boot", "/mnt/home", "/mnt/srv", "/mnt/var/cache", "/mnt/var/tmp", "/mnt/var/log", "/mnt/.snapshots"}
	for _, mp := range ours {
		if subtreeInUse(lsblkEntry{Type: "part", Mountpoint: mp}) {
			t.Errorf("subtreeInUse: installer target mount %q must not count as in use", mp)
		}
	}
	foreign := []string{"/", "/home", "/mnt/data", "/run/archiso/bootmnt", "[SWAP]", "/media/usb"}
	for _, mp := range foreign {
		if !subtreeInUse(lsblkEntry{Type: "part", Mountpoint: mp}) {
			t.Errorf("subtreeInUse: %q must count as in use", mp)
		}
	}
	// FSUSED without a printed mount point is a hidden mount (lsblk shows only
	// one mount point per device); FSUSED beside an installer mount is not.
	if !subtreeInUse(lsblkEntry{Type: "part", Fsused: jsonSize{Value: "4096"}}) {
		t.Error("subtreeInUse: FSUSED without a mount point must count as in use")
	}
	if subtreeInUse(lsblkEntry{Type: "part", Mountpoint: "/mnt", Fsused: jsonSize{Value: "4096"}}) {
		t.Error("subtreeInUse: FSUSED on the installer's own target root must not count as in use")
	}
	// A holder two levels down (partition → mapper → fs) still marks the disk.
	nested := lsblkEntry{Type: "disk", Children: []lsblkEntry{
		{Type: "part", Children: []lsblkEntry{{Type: "crypt", Mountpoint: "/home"}}},
	}}
	if !subtreeInUse(nested) {
		t.Error("subtreeInUse: a holder's mount must mark the whole disk in use")
	}
}

func TestSubtreeIsLive(t *testing.T) {
	live := map[string]bool{"254:0": true}
	tree := lsblkEntry{Type: "disk", MajMin: "8:0", Children: []lsblkEntry{
		{Type: "part", MajMin: "8:1", Children: []lsblkEntry{{Type: "crypt", MajMin: "254:0", Mountpoint: "/"}}},
	}}
	if !subtreeIsLive(tree, live) {
		t.Error("subtreeIsLive: the mapper backing / inside the subtree must mark the disk live")
	}
	other := lsblkEntry{Type: "disk", MajMin: "8:16", Children: []lsblkEntry{{Type: "part", MajMin: "8:17"}}}
	if subtreeIsLive(other, live) {
		t.Error("subtreeIsLive: an unrelated disk must not be live")
	}
}

// --- ValidateDiskNotInUse ------------------------------------------------------

func TestValidateDiskNotInUse(t *testing.T) {
	probe := func(tree string) string {
		return `cat <<'JSON'
` + tree + `
JSON
`
	}
	jsonOf := func(children string) string {
		return `{"blockdevices":[{"name":"vda","type":"disk","maj:min":"253:0","children":[` + children + `]}]}`
	}
	tests := []struct {
		name      string
		tree      string
		mountinfo string
		wantErr   string
	}{
		{
			name: "nothing mounted",
			tree: jsonOf(`{"name":"vda1","type":"part","maj:min":"253:1","mountpoint":null,"fsused":null}`),
		},
		{
			name: "installer's own target mounts (per-step run resumed)",
			tree: jsonOf(`{"name":"vda2","type":"part","maj:min":"253:2","mountpoint":"/mnt","fsused":12345}`),
		},
		{
			name:    "foreign mount",
			tree:    jsonOf(`{"name":"vda2","type":"part","maj:min":"253:2","mountpoint":"/home","fsused":12345}`),
			wantErr: "in use",
		},
		{
			name:    "mount under /mnt that is not the installer's",
			tree:    jsonOf(`{"name":"vda2","type":"part","maj:min":"253:2","mountpoint":"/mnt/data","fsused":12345}`),
			wantErr: "in use",
		},
		{
			name:    "active swap",
			tree:    jsonOf(`{"name":"vda3","type":"part","maj:min":"253:3","fstype":"swap","mountpoint":"[SWAP]","fsused":null}`),
			wantErr: "in use",
		},
		{
			name:    "hidden btrfs mount: FSUSED without a printed mount point",
			tree:    jsonOf(`{"name":"vda2","type":"part","maj:min":"253:2","mountpoint":null,"fsused":4096}`),
			wantErr: "in use",
		},
		{
			name:      "disk the ISO booted from: its partition is the archiso boot device",
			tree:      jsonOf(`{"name":"vda1","type":"part","maj:min":"8:33","mountpoint":"/run/archiso/bootmnt","fsused":null}`),
			mountinfo: "34 1 8:33 / /run/archiso/bootmnt rw - iso9660 /dev/sr0 ro\n",
			wantErr:   "holds the running system",
		},
		{
			name:      "LUKS system disk: the live mapper is nested in the subtree",
			tree:      jsonOf(`{"name":"vda2","type":"part","maj:min":"253:2","mountpoint":null,"fsused":null,"children":[{"name":"cryptroot","type":"crypt","maj:min":"254:0","mountpoint":"/"}]}`),
			mountinfo: "22 1 254:0 / / rw - ext4 /dev/mapper/cryptroot rw\n",
			wantErr:   "holds the running system",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeTools(t, map[string]string{"lsblk": probe(tt.tree)})
			fakeMountinfo(t, tt.mountinfo)
			err := ValidateDiskNotInUse("/dev/vda")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDiskNotInUse() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateDiskNotInUse() error = %v, want %q", err, tt.wantErr)
			}
		})
	}

	t.Run("unknown device is accepted", func(t *testing.T) {
		fakeTools(t, map[string]string{"lsblk": "exit 1\n"})
		fakeMountinfo(t, "")
		if err := ValidateDiskNotInUse("/dev/aelum-nonexistent-device"); err != nil {
			t.Fatalf("ValidateDiskNotInUse() = %v, want nil (the size check names a missing device)", err)
		}
	})

	t.Run("fake disk mode skips the probe", func(t *testing.T) {
		old := fakeDisksEnabled
		t.Cleanup(func() { SetFakeDisks(old) })
		SetFakeDisks(true)
		// No lsblk on PATH at all: only a skipped probe can pass.
		fakeTools(t, map[string]string{})
		fakeMountinfo(t, "")
		if err := ValidateDiskNotInUse("/dev/sda"); err != nil {
			t.Fatalf("ValidateDiskNotInUse() in fake mode = %v, want nil", err)
		}
	})
}

// --- ValidateDiskPath (whole-disk guard) ---------------------------------------

func TestValidateDiskPath_WholeDisk(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"whole disk", "echo disk\n", ""},
		{"partition is refused", "echo part\n", "not a whole disk"},
		{"mapper device is refused", "echo crypt\n", "not a whole disk"},
		{"lvm volume is refused", "echo lvm\n", "not a whole disk"},
		{"unknown device passes the path check", "exit 1\n", ""},
		{"empty output passes the path check", "true\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeTools(t, map[string]string{"lsblk": tt.script})
			err := ValidateDiskPath("/dev/vda1")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDiskPath() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateDiskPath() error = %v, want %q", err, tt.wantErr)
			}
		})
	}

	t.Run("fake disk mode skips the probe", func(t *testing.T) {
		old := fakeDisksEnabled
		t.Cleanup(func() { SetFakeDisks(old) })
		SetFakeDisks(true)
		fakeTools(t, map[string]string{})
		if err := ValidateDiskPath("/dev/nvme4n1"); err != nil {
			t.Fatalf("ValidateDiskPath() in fake mode = %v, want nil", err)
		}
	})
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

// An lsblk that exits 0 without naming the device would otherwise make the
// Blockdevices[0] index panic the whole GUI: this runs for every disk on
// page load. The twin GetPartitions already refused this; both must.
func TestGetDiskPartitions_NoDevices(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": lsblkShim(disksJSON, `{"blockdevices":[]}`)})

	if _, err := GetDiskPartitions("/dev/vda"); err == nil || !strings.Contains(err.Error(), "no block devices found") {
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
