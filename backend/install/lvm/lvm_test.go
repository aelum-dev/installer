package lvm

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestPath verifies logical volumes resolve to their device-mapper paths.
func TestPath(t *testing.T) {
	if got := Path("aelum", "root"); got != "/dev/mapper/aelum-root" {
		t.Errorf("Path = %q, want /dev/mapper/aelum-root", got)
	}
	if got := Path(shared.LvmVolumeGroup, shared.LvmHomeVolume); got != shared.LvmHomeLvPath {
		t.Errorf("Path = %q, want %q", got, shared.LvmHomeLvPath)
	}
}

// TestPvCreateArgs verifies pvcreate force-creates the volume without prompting.
func TestPvCreateArgs(t *testing.T) {
	got := pvCreateArgs("/dev/vda2")
	want := []string{"pvcreate", "-ff", "-y", "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pvCreateArgs() = %v, want %v", got, want)
	}
}

// TestVgCreateArgs verifies the volume group is named and the PV is the final
// argument.
func TestVgCreateArgs(t *testing.T) {
	got := vgCreateArgs(shared.LvmVolumeGroup, "/dev/vda2")
	want := []string{"vgcreate", shared.LvmVolumeGroup, "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("vgCreateArgs() = %v, want %v", got, want)
	}
}

// TestLvCreateSizeArgs verifies the root LV is created at a fixed size in MiB,
// with signature wiping forced so a reused LV name never triggers an
// interactive "Wipe it? [y/n]" prompt (which defaults to "n" without a TTY).
func TestLvCreateSizeArgs(t *testing.T) {
	got := lvCreateSizeArgs(8192, shared.LvmRootVolume, shared.LvmVolumeGroup)
	want := []string{"lvcreate", "-y", "-W", "y", "-L", "8192M", "-n", shared.LvmRootVolume, shared.LvmVolumeGroup}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lvCreateSizeArgs() = %v, want %v", got, want)
	}
}

// TestLvCreateRestArgs verifies the home LV consumes all remaining free space,
// with signature wiping forced (see TestLvCreateSizeArgs).
func TestLvCreateRestArgs(t *testing.T) {
	got := lvCreateRestArgs(shared.LvmHomeVolume, shared.LvmVolumeGroup)
	want := []string{"lvcreate", "-y", "-W", "y", "-l", "100%FREE", "-n", shared.LvmHomeVolume, shared.LvmVolumeGroup}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lvCreateRestArgs() = %v, want %v", got, want)
	}
}

// TestVgChangeArgs verifies activation and deactivation use -ay / -an.
func TestVgChangeArgs(t *testing.T) {
	if got := vgChangeArgs(true, shared.LvmVolumeGroup); !reflect.DeepEqual(got, []string{"vgchange", "-ay", shared.LvmVolumeGroup}) {
		t.Errorf("vgChangeArgs(true) = %v", got)
	}
	if got := vgChangeArgs(false, shared.LvmVolumeGroup); !reflect.DeepEqual(got, []string{"vgchange", "-an", shared.LvmVolumeGroup}) {
		t.Errorf("vgChangeArgs(false) = %v", got)
	}
}

// TestRemoveArgs verifies vgremove/pvremove force-remove without prompting.
func TestRemoveArgs(t *testing.T) {
	if got := vgRemoveArgs(shared.LvmVolumeGroup); !reflect.DeepEqual(got, []string{"vgremove", "-ff", shared.LvmVolumeGroup}) {
		t.Errorf("vgRemoveArgs() = %v", got)
	}
	if got := pvRemoveArgs("/dev/vda2"); !reflect.DeepEqual(got, []string{"pvremove", "-ff", "-y", "/dev/vda2"}) {
		t.Errorf("pvRemoveArgs() = %v", got)
	}
}

// TestComputeRootMiB verifies the fixed 30% / 8 GiB-floor split, including the
// clamp that keeps the home LV at or above the 8 GiB floor.
func TestComputeRootMiB(t *testing.T) {
	const minMiB = 8 * 1024 // 8 GiB in MiB

	tests := []struct {
		name    string
		vgMiB   int64
		want    int64
		wantErr bool
	}{
		{"large disk uses 30%", 100 * 1024, 30 * 1024, false},
		{"small disk clamps root down to keep home floor", 16000, 16000 - minMiB, false},
		{"exactly 16 GiB splits evenly", 16 * 1024, minMiB, false},
		{"just above floor", 20000, minMiB, false},
		{"too small to hold both", 8192, 0, true},
		{"zero", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := computeRootMiB(tt.vgMiB)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("computeRootMiB(%d) = %d, want error", tt.vgMiB, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("computeRootMiB(%d) unexpected error: %v", tt.vgMiB, err)
			}
			if got != tt.want {
				t.Errorf("computeRootMiB(%d) = %d, want %d", tt.vgMiB, got, tt.want)
			}
			// The home LV takes the rest; it must stay at or above the floor.
			if home := tt.vgMiB - got; home < minMiB {
				t.Errorf("home LV size %d MiB is below the %d MiB floor", home, minMiB)
			}
		})
	}
}

// TestComputeRootMiB_MatchesConstants guards the sizing rule against the shared
// constants it is derived from.
func TestComputeRootMiB_MatchesConstants(t *testing.T) {
	if shared.LvmRootPercent != 30 {
		t.Fatalf("LvmRootPercent changed to %d; update the split tests", shared.LvmRootPercent)
	}
	if shared.LvmRootMinBytes>>20 != 8192 {
		t.Fatalf("LvmRootMinBytes changed to %d; update the split tests", shared.LvmRootMinBytes)
	}
}

// --- functional tests via the injected CommandRunner -------------------------

// TestVGActivate verifies the volume group is activated (-ay) and deactivated
// (-an), and that failures are surfaced.
func TestVGActivate(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := VGActivate(shared.LvmVolumeGroup); err != nil {
		t.Fatalf("VGActivate() = %v, want nil", err)
	}
	calls := rec.CallsFor("vgchange")
	want := []string{"vgchange", "-ay", shared.LvmVolumeGroup}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("vgchange argv = %v, want %v", calls, want)
	}

	rec.Reset()
	rec.OnError("vgchange", errors.New("VG not found"))
	if err := VGActivate(shared.LvmVolumeGroup); err == nil {
		t.Fatal("VGActivate() = nil, want error")
	}
}

// TestVGDeactivate verifies deactivation uses -an.
func TestVGDeactivate(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := VGDeactivate(shared.LvmVolumeGroup); err != nil {
		t.Fatalf("VGDeactivate() = %v, want nil", err)
	}
	calls := rec.CallsFor("vgchange")
	want := []string{"vgchange", "-an", shared.LvmVolumeGroup}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("vgchange argv = %v, want %v", calls, want)
	}
}

// TestVGSizeMiB verifies the vgs output is parsed to MiB.
func TestVGSizeMiB(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("vgs", " 16384.50\n")
	defer primitives.SetCommandRunner(rec)()

	got, err := VGSizeMiB(shared.LvmVolumeGroup)
	if err != nil {
		t.Fatalf("VGSizeMiB() = %v, want nil", err)
	}
	if got != 16384 {
		t.Errorf("VGSizeMiB() = %d, want 16384", got)
	}

	rec.Reset()
	rec.OnOutput("vgs", "not-a-number")
	if _, err := VGSizeMiB(shared.LvmVolumeGroup); err == nil {
		t.Fatal("VGSizeMiB() = nil error, want parse error")
	}

	rec.Reset()
	rec.OnError("vgs", errors.New("exit status 5"))
	if _, err := VGSizeMiB(shared.LvmVolumeGroup); err == nil {
		t.Fatal("VGSizeMiB() = nil error, want command error")
	}
}

// TestDeviceSizeBytes verifies blockdev output is parsed to bytes.
func TestDeviceSizeBytes(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("blockdev", "34359738368\n")
	defer primitives.SetCommandRunner(rec)()

	got, err := DeviceSizeBytes("/dev/vda2")
	if err != nil {
		t.Fatalf("DeviceSizeBytes() = %v, want nil", err)
	}
	if got != 34359738368 {
		t.Errorf("DeviceSizeBytes() = %d, want 34359738368", got)
	}

	rec.Reset()
	rec.OnError("blockdev", errors.New("no such device"))
	if _, err := DeviceSizeBytes("/dev/vda2"); err == nil {
		t.Fatal("DeviceSizeBytes() = nil error, want command error")
	}
}

// TestTeardown verifies the idempotent cleanup issues vgchange -an, vgremove and
// pvremove for each non-empty device, skipping empty entries.
func TestTeardown(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	Teardown(shared.LvmVolumeGroup, []string{"/dev/vda2", "", "/dev/vda3"})

	if calls := rec.CallsFor("vgchange"); len(calls) != 1 || calls[0][1] != "-an" {
		t.Errorf("vgchange calls = %v, want one -an", calls)
	}
	if calls := rec.CallsFor("vgremove"); len(calls) != 1 {
		t.Errorf("vgremove calls = %v, want one", calls)
	}
	pv := rec.CallsFor("pvremove")
	if len(pv) != 2 {
		t.Errorf("pvremove calls = %v, want two (empty device skipped)", pv)
	}
	for _, c := range pv {
		if c[len(c)-1] == "" {
			t.Errorf("pvremove issued for an empty device: %v", c)
		}
	}
}

// largeDevice / largeVG script a 32 GiB device holding a 32 GiB (32768 MiB) VG.
const (
	blockdev32GiB = "34359738368"
	vgs32768MiB   = "32768"
)

// TestApply_Success drives the full LVM creation sequence with a scripted
// runner and a stub formatter, verifying the LVs and command sequence.
func TestApply_Success(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("blockdev", blockdev32GiB).
		OnOutput("vgs", vgs32768MiB)
	defer primitives.SetCommandRunner(rec)()

	type formatted struct {
		device string
		fs     string
		subvol int
	}
	var calls []formatted
	format := func(device, fsType string, subs []shared.BtrfsSubvolume) error {
		calls = append(calls, formatted{device: device, fs: fsType, subvol: len(subs)})
		return nil
	}

	lvs, err := Apply("/dev/vda2", Params{FileSystem: shared.FileSystemBtrfs}, format)
	if err != nil {
		t.Fatalf("Apply() = %v, want nil", err)
	}

	if len(lvs) != 2 {
		t.Fatalf("Apply() returned %d LVs, want 2", len(lvs))
	}
	if lvs[0].Path != shared.LvmRootLvPath || lvs[0].MountPoint != "/" {
		t.Errorf("root LV = %+v, want path %s mount /", lvs[0], shared.LvmRootLvPath)
	}
	if lvs[1].Path != shared.LvmHomeLvPath || lvs[1].MountPoint != "/home" {
		t.Errorf("home LV = %+v, want path %s mount /home", lvs[1], shared.LvmHomeLvPath)
	}
	if lvs[0].FileSystem != shared.FileSystemBtrfs || lvs[1].FileSystem != shared.FileSystemBtrfs {
		t.Errorf("LV filesystems = %q/%q, want btrfs", lvs[0].FileSystem, lvs[1].FileSystem)
	}

	// The command sequence: pvcreate, vgcreate, then two lvcreate calls.
	if calls := rec.CallsFor("pvcreate"); len(calls) != 1 {
		t.Errorf("pvcreate calls = %v, want 1", calls)
	}
	if calls := rec.CallsFor("vgcreate"); len(calls) != 1 {
		t.Errorf("vgcreate calls = %v, want 1", calls)
	}
	if calls := rec.CallsFor("lvcreate"); len(calls) != 2 {
		t.Errorf("lvcreate calls = %v, want 2", calls)
	}

	// Both LVs are formatted with btrfs; the root LV carries the LVM btrfs
	// subvolume scheme.
	if len(calls) != 2 {
		t.Fatalf("formatter called %d times, want 2", len(calls))
	}
	if calls[0].device != shared.LvmRootLvPath || calls[0].fs != shared.FileSystemBtrfs || calls[0].subvol == 0 {
		t.Errorf("root format = %+v, want btrfs with subvolumes", calls[0])
	}
	if calls[1].device != shared.LvmHomeLvPath {
		t.Errorf("home format device = %q, want %q", calls[1].device, shared.LvmHomeLvPath)
	}
}

// TestApply_Ext4PassesNoSubvolumes verifies ext4 LVs are formatted without the
// btrfs subvolume scheme.
func TestApply_Ext4PassesNoSubvolumes(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("blockdev", blockdev32GiB).
		OnOutput("vgs", vgs32768MiB)
	defer primitives.SetCommandRunner(rec)()

	gotSubvol := -1
	format := func(_ string, _ string, subs []shared.BtrfsSubvolume) error {
		gotSubvol = len(subs)
		return nil
	}

	lvs, err := Apply("/dev/vda2", Params{FileSystem: shared.FileSystemExt4}, format)
	if err != nil {
		t.Fatalf("Apply() = %v, want nil", err)
	}
	if gotSubvol != 0 {
		t.Errorf("ext4 formatter received %d subvolumes, want 0", gotSubvol)
	}
	if lvs[0].FileSystem != shared.FileSystemExt4 {
		t.Errorf("LV filesystem = %q, want ext4", lvs[0].FileSystem)
	}
}

// TestApply_InputValidation verifies empty device and nil formatter are rejected
// before any command runs.
func TestApply_InputValidation(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if _, err := Apply("", Params{}, noopFormatter); err == nil {
		t.Error("Apply(\"\") = nil, want error")
	}
	if _, err := Apply("/dev/vda2", Params{}, nil); err == nil {
		t.Error("Apply(nil formatter) = nil, want error")
	}
	if len(rec.Calls()) != 0 {
		t.Errorf("commands ran during input validation: %v", rec.Calls())
	}
}

// TestApply_DeviceTooSmall verifies the 16 GiB floor is enforced.
func TestApply_DeviceTooSmall(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("blockdev", "1073741824") // 1 GiB
	defer primitives.SetCommandRunner(rec)()

	_, err := Apply("/dev/vda2", Params{}, noopFormatter)
	if err == nil {
		t.Fatal("Apply() = nil, want too-small error")
	}
	if !strings.Contains(err.Error(), "at least") {
		t.Errorf("error = %q, want it to mention the minimum size", err)
	}
}

// TestApply_PvCreateFails verifies a failing pvcreate aborts with context.
func TestApply_PvCreateFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("blockdev", blockdev32GiB).
		OnOutput("vgs", vgs32768MiB).
		OnArgsError([]string{"pvcreate"}, errors.New("exit status 5"))
	defer primitives.SetCommandRunner(rec)()

	_, err := Apply("/dev/vda2", Params{}, noopFormatter)
	if err == nil || !strings.Contains(err.Error(), "pvcreate") {
		t.Fatalf("Apply() error = %v, want pvcreate failure", err)
	}
}

// TestApply_FormatFails verifies a formatter error is wrapped with LV context.
func TestApply_FormatFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("blockdev", blockdev32GiB).
		OnOutput("vgs", vgs32768MiB)
	defer primitives.SetCommandRunner(rec)()

	fail := func(string, string, []shared.BtrfsSubvolume) error {
		return errors.New("mkfs failed")
	}

	_, err := Apply("/dev/vda2", Params{}, fail)
	if err == nil || !strings.Contains(err.Error(), "format root LV") {
		t.Fatalf("Apply() error = %v, want format root LV failure", err)
	}
}

func noopFormatter(string, string, []shared.BtrfsSubvolume) error { return nil }
