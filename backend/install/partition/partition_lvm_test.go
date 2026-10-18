package partition

import (
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// Regression: a previous LVM install leaves the `aelum` volume group active on
// the target disk. Its device-mapper holders keep the System partition busy, so
// mkfs (which opens with O_EXCL) fails with "Device or resource busy". Apply
// must tear the group down before any destructive command.
func TestApply_TearsDownStaleLvmBeforeDestructiveCommands(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, "").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda1"}, "EFI-UUID\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	layout := newDiskLayout()
	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	if err := Apply(ctx, &layout); err != nil {
		t.Fatalf("Apply() = %v", err)
	}

	teardown := firstCall(rec, func(argv []string) bool {
		return len(argv) >= 3 && argv[0] == "vgchange" && argv[1] == "-an" && argv[2] == shared.LvmVolumeGroup
	})
	if teardown < 0 {
		t.Fatal("Apply() did not deactivate the stale aelum VG")
	}

	destructive := firstCall(rec, func(argv []string) bool {
		if len(argv) == 0 {
			return false
		}
		switch argv[0] {
		case "mkfs.ext4", "mkfs.btrfs", "wipefs", "cryptsetup":
			return true
		case "sgdisk":
			return len(argv) > 1 && (argv[1] == "-n" || argv[1] == "-d" || argv[1] == "-t" || argv[1] == "-Z")
		}
		return false
	})
	if destructive >= 0 && destructive < teardown {
		t.Fatalf("destructive command %v ran before the stale VG teardown", rec.Calls()[destructive].Args)
	}
}

// firstCall returns the index of the first recorded command matching match, or
// -1 when none does.
func firstCall(rec *primitivestest.Recorder, match func([]string) bool) int {
	for i, c := range rec.Calls() {
		if match(c.Args) {
			return i
		}
	}
	return -1
}
