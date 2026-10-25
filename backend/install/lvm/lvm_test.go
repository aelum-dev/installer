package lvm

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// useRecorder installs a scripted command runner for the duration of a test.
func useRecorder(t *testing.T) *primitivestest.Recorder {
	t.Helper()
	rec := primitivestest.NewRecorder()
	t.Cleanup(primitives.SetCommandRunner(rec))
	return rec
}

// callIndex returns the position of the first call whose argv matches every
// given token, or -1.
func callIndex(rec *primitivestest.Recorder, tokens ...string) int {
	for i, c := range rec.Calls() {
		joined := strings.Join(c.Args, " ")
		matches := true
		for _, tok := range tokens {
			if !strings.Contains(joined, tok) {
				matches = false
				break
			}
		}
		if matches {
			return i
		}
	}
	return -1
}

func TestVolumeGroupName(t *testing.T) {
	// The volume group is the install id alone: no prefix, no hyphen.
	tests := []struct {
		name     string
		partUUID string
		want     string
	}{
		{"typical partuuid", "1a2b3c4d-0000-4000-8000-000000000000", "1a2b3c4d"},
		{"uppercase is lowered", "AABBCCDD-1111-2222-3333-444455556666", "aabbccdd"},
		{"separators are dropped", "abcd-ef01", "abcdef01"},
		{"digit-leading id", "12345678-0000-4000-8000-000000000000", "12345678"},
	}
	for _, tt := range tests {
		if got := VolumeGroupName(tt.partUUID); got != tt.want {
			t.Errorf("%s: VolumeGroupName(%q) = %q, want %q", tt.name, tt.partUUID, got, tt.want)
		}
	}
}

func TestVGNameForDevice(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value"}, "1a2b3c4d-aaaa\n")

	got, err := VGNameForDevice("/dev/vda2")
	if err != nil {
		t.Fatalf("VGNameForDevice: %v", err)
	}
	if want := "1a2b3c4d"; got != want {
		t.Errorf("VGNameForDevice = %q, want %q", got, want)
	}
	if !rec.Called("blkid") {
		t.Error("expected blkid to be called")
	}
}

func TestVGNameForDeviceRequiresPARTUUID(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value"}, "\n")

	if _, err := VGNameForDevice("/dev/vda2"); err == nil {
		t.Fatal("expected an error when the device has no PARTUUID, got nil")
	}
}

func TestPathUsesVGSymlinkNotMapper(t *testing.T) {
	got := Path("1a2b3c4d", shared.LvmRootVolumeName)
	if want := "/dev/1a2b3c4d/root"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	if strings.Contains(got, "/dev/mapper/") {
		t.Errorf("Path must not build a /dev/mapper path (hyphens are escaped there): %q", got)
	}
}

func TestMapperNameEscapesHyphens(t *testing.T) {
	// The helper is tested for any name, not just this installer's hyphen-free
	// group names: dmsetup must be given the node LVM really created.
	tests := []struct {
		name         string
		vg, lv, want string
	}{
		{"hyphen-free group (what VolumeGroupName builds)", "1a2b3c4d", "root", "1a2b3c4d-root"},
		{"hyphenated group doubles every hyphen", "aelum-1a2b3c4d", "root", "aelum--1a2b3c4d-root"},
		{"no hyphen anywhere", "plainvg", "home", "plainvg-home"},
	}
	for _, tt := range tests {
		if got := mapperName(tt.vg, tt.lv); got != tt.want {
			t.Errorf("%s: mapperName(%q, %q) = %q, want %q", tt.name, tt.vg, tt.lv, got, tt.want)
		}
	}
}

func TestComputeRootMiB(t *testing.T) {
	minMiB := int64(shared.MinLvmRootVolumeBytes >> 20)

	tests := []struct {
		name  string
		vgMiB int64
		want  int64
	}{
		{"minimum group uses the root floor", int64(shared.MinLvmVolumeGroupBytes >> 20), minMiB},
		{"30 percent inside the range", 200 * 1024, 200 * 1024 * int64(shared.LvmRootPercent) / 100},
		{"exactly two floors", 2 * minMiB, minMiB},
		{"group smaller than two floors still yields a positive root", minMiB + 100, 100},
	}
	for _, tt := range tests {
		got, err := computeRootMiB(tt.vgMiB)
		if err != nil {
			t.Fatalf("%s: computeRootMiB(%d): %v", tt.name, tt.vgMiB, err)
		}
		if got != tt.want {
			t.Errorf("%s: computeRootMiB(%d) = %d, want %d", tt.name, tt.vgMiB, got, tt.want)
		}
	}
}

func TestCreateBuildsPVThenVG(t *testing.T) {
	rec := useRecorder(t)
	rec.OnOutput("blockdev", "17179869184") // 16 GiB
	rec.OnArgsOutput([]string{"vgs", "--noheadings", "--nosuffix", "--units", "m", "-o", "vg_size"}, "16384\n")

	if err := Create("/dev/vda2", "1a2b3c4d"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	pv := callIndex(rec, "pvcreate", "-ff", "-y", "/dev/vda2")
	vg := callIndex(rec, "vgcreate", "1a2b3c4d", "/dev/vda2")
	if pv < 0 || vg < 0 {
		t.Fatalf("expected pvcreate and vgcreate, got %v", rec.CallsFor("pvcreate"))
	}
	if pv > vg {
		t.Error("pvcreate must run before vgcreate")
	}
}

func TestCreateRefusesSmallDevice(t *testing.T) {
	rec := useRecorder(t)
	rec.OnOutput("blockdev", "8589934592") // 8 GiB

	err := Create("/dev/vda2", "1a2b3c4d")
	if err == nil {
		t.Fatal("expected an error for a device below the LVM floor, got nil")
	}
	if rec.Called("pvcreate") {
		t.Error("pvcreate must not run on a device that is too small")
	}
}

func TestCreateRefusesMultiPVGroup(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"pvs", "--noheadings", "-o", "vg_name"}, "othervg\n")
	rec.OnArgsOutput([]string{"vgs", "--noheadings", "-o", "pv_count"}, "2\n")

	err := Create("/dev/vda2", "1a2b3c4d")
	if err == nil {
		t.Fatal("expected an error for a multi-PV group, got nil")
	}
	if !strings.Contains(err.Error(), "spans 2 physical volumes") {
		t.Errorf("error = %v, want it to explain the multi-PV refusal", err)
	}
	if rec.Called("vgremove") {
		t.Error("vgremove must not run for a group this install does not own")
	}
}

func TestReleaseDeviceSinglePV(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"pvs", "--noheadings", "-o", "vg_name"}, "1a2b3c4d\n")
	rec.OnArgsOutput([]string{"vgs", "--noheadings", "-o", "pv_count"}, "1\n")

	if err := ReleaseDevice("/dev/vda2"); err != nil {
		t.Fatalf("ReleaseDevice: %v", err)
	}

	deactivate := callIndex(rec, "vgchange", "-an", "1a2b3c4d")
	remove := callIndex(rec, "vgremove", "-ff", "1a2b3c4d")
	pvremove := callIndex(rec, "pvremove", "-ff", "-y", "/dev/vda2")
	if deactivate < 0 || remove < 0 || pvremove < 0 {
		t.Fatalf("missing teardown command: %v", rec.Calls())
	}
	if deactivate >= remove || remove >= pvremove {
		t.Error("teardown order must be vgchange -an, vgremove, pvremove")
	}
	if idx := callIndex(rec, "dmsetup", "remove", "1a2b3c4d-root"); idx < 0 {
		t.Error("expected the escaped root LV mapper node to be removed")
	}
}

func TestReleaseDeviceWithoutPVLabel(t *testing.T) {
	rec := useRecorder(t)
	// A device with no PV label makes pvs fail; that is the common case.
	rec.OnArgsError([]string{"pvs"}, errors.New("no PV label"))

	if err := ReleaseDevice("/dev/vda2"); err != nil {
		t.Fatalf("ReleaseDevice on a non-PV device: %v", err)
	}
	if rec.Called("vgremove") {
		t.Error("vgremove must not run when the device is not a PV")
	}
}

func TestReleaseForDiskIncludesCryptoAndLVMDevices(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"lsblk"}, "vda2 part\ndm-0 crypt\ndm-1 lvm\n")

	ReleaseForDisk("/dev/vda", nil)

	// Every derived device is probed for a PV; the LUKS mapper is what carries
	// the PV when LVM sits on encryption.
	for _, dev := range []string{"/dev/vda2", "/dev/dm-0", "/dev/dm-1"} {
		if callIndex(rec, "pvs", dev) < 0 {
			t.Errorf("expected pvs lookup for %s, got %v", dev, rec.CallsFor("pvs"))
		}
	}
}

func TestDeactivateBeforeMapperCloseIsCallerResponsibility(t *testing.T) {
	rec := useRecorder(t)

	if err := Deactivate("1a2b3c4d"); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if idx := callIndex(rec, "vgchange", "-an", "1a2b3c4d"); idx < 0 {
		t.Errorf("expected vgchange -an, got %v", rec.Calls())
	}
}

func TestDeactivateForDisk_KeepsTheInstalledGroup(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"lsblk"}, "vda2 part\ndm-1 lvm\n")
	rec.OnArgsOutput([]string{"pvs", "--noheadings", "-o", "vg_name", "/dev/vda2"},
		"  1a2b3c4d")

	DeactivateForDisk("/dev/vda", nil)

	if callIndex(rec, "vgchange", "-an", "1a2b3c4d") < 0 {
		t.Errorf("expected vgchange -an, got %v", rec.Calls())
	}
	// The installed system's root *is* a logical volume: end-of-install cleanup
	// must never remove the group or its physical volume label.
	for _, forbidden := range []string{"vgremove", "pvremove", "lvremove"} {
		if rec.Called(forbidden) {
			t.Errorf("cleanup ran %s; the installed volume group must survive", forbidden)
		}
	}
}

func TestDeactivateForDisk_SkipsDevicesWithoutAGroup(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"lsblk"}, "vda1 part\nvda2 part\n")

	DeactivateForDisk("/dev/vda", nil)

	if rec.Called("vgchange") {
		t.Errorf("a device with no volume group must not be deactivated: %v", rec.Calls())
	}
}

func TestDeactivateForDisk_EmptyIDIsNoOp(t *testing.T) {
	rec := useRecorder(t)

	DeactivateForDisk("", nil)

	if len(rec.Calls()) != 0 {
		t.Fatalf("empty disk ID ran %d commands, want none", len(rec.Calls()))
	}
}

func TestCreateHomeLVConsumesRest(t *testing.T) {
	rec := useRecorder(t)

	path, err := CreateHomeLV("1a2b3c4d")
	if err != nil {
		t.Fatalf("CreateHomeLV: %v", err)
	}
	if want := "/dev/1a2b3c4d/home"; path != want {
		t.Errorf("CreateHomeLV path = %q, want %q", path, want)
	}
	if idx := callIndex(rec, "lvcreate", "-l", "100%FREE", "-n", "home", "1a2b3c4d"); idx < 0 {
		t.Errorf("expected lvcreate -l 100%%FREE, got %v", rec.CallsFor("lvcreate"))
	}
}

func TestCreateRootLVBtrfsUsesWholeGroup(t *testing.T) {
	rec := useRecorder(t)

	path, err := CreateRootLV("1a2b3c4d", shared.FileSystemBtrfs)
	if err != nil {
		t.Fatalf("CreateRootLV: %v", err)
	}
	if want := "/dev/1a2b3c4d/root"; path != want {
		t.Errorf("CreateRootLV path = %q, want %q", path, want)
	}
	if idx := callIndex(rec, "lvcreate", "-l", "100%FREE", "-n", "root", "1a2b3c4d"); idx < 0 {
		t.Errorf("expected the btrfs root LV to take the whole group, got %v", rec.CallsFor("lvcreate"))
	}
}

func TestCreateRootLVExt4IsSized(t *testing.T) {
	rec := useRecorder(t)
	rec.OnArgsOutput([]string{"vgs", "--noheadings", "--nosuffix", "--units", "m", "-o", "vg_size"}, "204800\n")

	if _, err := CreateRootLV("1a2b3c4d", shared.FileSystemExt4); err != nil {
		t.Fatalf("CreateRootLV: %v", err)
	}
	// 30% of 200 GiB = 61440 MiB.
	if idx := callIndex(rec, "lvcreate", "-L", "61440M", "-n", "root", "1a2b3c4d"); idx < 0 {
		t.Errorf("expected a 61440M root LV, got %v", rec.CallsFor("lvcreate"))
	}
}
