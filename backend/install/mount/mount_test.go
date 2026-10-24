package mount

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// baseCtx is a fully-populated context that would pass every precondition, so
// each test can knock out exactly one field and assert the corresponding error.
func baseCtx() *shared.InstallContext {
	return &shared.InstallContext{
		DiskID:     "/dev/vda",
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{
			Disk:        "/dev/vda",
			EFIPart:     "/dev/vda1",
			RootPart:    "/dev/vda2",
			RootUUID:    "ROOT-UUID",
			MountPoints: map[string]string{},
		},
	}
}

func TestRun_RequiresDiskID(t *testing.T) {
	ctx := baseCtx()
	ctx.DiskID = ""
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for a missing disk ID")
	}
}

func TestRun_RequiresPartitions(t *testing.T) {
	ctx := baseCtx()
	ctx.Partitions = nil
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for nil partitions")
	}
}

func TestRun_RequiresFileSystem(t *testing.T) {
	ctx := baseCtx()
	ctx.FileSystem = ""
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for a missing filesystem")
	}
}

func TestRun_RequiresPartitionPaths(t *testing.T) {
	ctx := baseCtx()
	ctx.Partitions.EFIPart = ""
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error for missing partition paths")
	}
}

func TestRun_LuksRequiresMapper(t *testing.T) {
	ctx := baseCtx()
	ctx.Luks = true
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error when LUKS is enabled without a root mapper path")
	}
}

// TestRun_LuksRequiresPassphrase checks that mount refuses to proceed when LUKS
// is enabled but no passphrase was supplied. The exact error depends on the
// host (whether the mapper already exists), so only the failure is asserted.
func TestRun_LuksRequiresPassphrase(t *testing.T) {
	ctx := baseCtx()
	ctx.Luks = true
	ctx.Partitions.RootMapper = shared.LuksMapperPath
	// No LuksPassphrase set.
	if err := Run(ctx); err == nil {
		t.Fatal("expected an error when LUKS is enabled without a passphrase")
	}
}

func TestVerifyMount_NotMounted(t *testing.T) {
	err := verifyMount("/aelum-nonexistent-mountpoint", "/dev/vda2")
	if err == nil {
		t.Fatal("expected an error for a mount point that is not mounted")
	}
	if !strings.Contains(err.Error(), "not mounted") {
		t.Errorf("error = %v, want a 'not mounted' error", err)
	}
}

// TestVerifyMount_LvmMapperAliasAccepted reproduces what findmnt really reports
// for a logical volume: the mount was made through /dev/<vg>/<lv>, but the
// device-mapper node it reached is named with every hyphen of the volume group
// doubled. Both names are the same device, so verification must accept it.
func TestVerifyMount_LvmMapperAliasAccepted(t *testing.T) {
	const lv = "/dev/aelum-1a2b3c4d/root"
	const mapper = "/dev/mapper/aelum--1a2b3c4d-root"
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"findmnt", "-n", "-o", "SOURCE", "/mnt"}, mapper+"[/@]\n").
		OnArgsOutput([]string{"readlink", "-f", mapper}, "/dev/dm-0\n").
		OnArgsOutput([]string{"readlink", "-f", lv}, "/dev/dm-0\n")
	defer primitives.SetCommandRunner(rec)()

	if err := verifyMount("/mnt", lv); err != nil {
		t.Fatalf("verifyMount() = %v, want the device-mapper alias accepted", err)
	}
	if !rec.Called("readlink") {
		t.Error("the alias check should only resolve paths once the names differ")
	}
}

// TestVerifyMount_DifferentDeviceNodesRejected guards the alias check: two paths
// that resolve to different nodes are still the wrong mount, and a path that
// resolves to nothing never counts as a match.
func TestVerifyMount_DifferentDeviceNodesRejected(t *testing.T) {
	const lv = "/dev/aelum-1a2b3c4d/root"
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"findmnt", "-n", "-o", "SOURCE", "/mnt"}, "/dev/mapper/aelum--1a2b3c4d-home\n").
		OnArgsOutput([]string{"readlink", "-f", "/dev/mapper/aelum--1a2b3c4d-home"}, "/dev/dm-1\n").
		OnArgsOutput([]string{"readlink", "-f", lv}, "/dev/dm-0\n")
	defer primitives.SetCommandRunner(rec)()

	err := verifyMount("/mnt", lv)
	if err == nil || !strings.Contains(err.Error(), "expected "+lv) {
		t.Fatalf("verifyMount() error = %v, want a mismatch against %s", err, lv)
	}
}

// TestVerifyMount_UnresolvablePathRejected keeps verification strict when the
// alias lookup cannot answer: an expected device that resolves to nothing must
// not be matched to an unrelated mount.
func TestVerifyMount_UnresolvablePathRejected(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"findmnt", "-n", "-o", "SOURCE", "/mnt"}, "/dev/mapper/aelum--1a2b3c4d-root\n").
		OnArgsError([]string{"readlink", "-f"}, errors.New("readlink: No such file or directory"))
	defer primitives.SetCommandRunner(rec)()

	err := verifyMount("/mnt", "/dev/aelum-1a2b3c4d/root")
	if err == nil || !strings.Contains(err.Error(), "is mounted on") {
		t.Fatalf("verifyMount() error = %v, want a mismatch error", err)
	}
}

// TestBuildMountOptsAlias guards the exported alias used by callers that only
// import mount.
func TestBuildMountOptsAlias(t *testing.T) {
	sv := shared.BtrfsSubvolume{Name: "@", Compression: "zstd"}
	if got, want := BuildMountOpts(&sv), "subvol=@,compress=zstd"; got != want {
		t.Errorf("BuildMountOpts = %q, want %q", got, want)
	}
}

// --- LVM ---------------------------------------------------------------------

// lvmMountRecorder scripts the mount step's verification commands so a logical
// volume install can run end-to-end without touching real devices.
//
// findmnt is scripted the way a real install reports it: the mount is named by
// the device-mapper node it reached, where LVM doubles every hyphen of the
// volume group name (aelum-1a2b3c4d/root → aelum--1a2b3c4d-root). readlink
// resolves both spellings to the same node, so these tests cover the alias the
// mount step has to tolerate instead of assuming the /dev/<vg>/<lv> spelling.
func lvmMountRecorder(t *testing.T) *primitivestest.Recorder {
	t.Helper()
	const (
		lvRoot   = "/dev/aelum-1a2b3c4d/root"
		lvHome   = "/dev/aelum-1a2b3c4d/home"
		dmRoot   = "/dev/mapper/aelum--1a2b3c4d-root"
		dmHome   = "/dev/mapper/aelum--1a2b3c4d-home"
		nodeRoot = "/dev/dm-0"
		nodeHome = "/dev/dm-1"
	)
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"findmnt", "-n", "-o", "SOURCE", shared.MountRoot}, dmRoot+"\n").
		OnArgsOutput([]string{"findmnt", "-n", "-o", "SOURCE", shared.MountRoot + "/home"}, dmHome+"\n").
		OnArgsOutput([]string{"findmnt", "-n", "-o", "SOURCE", shared.ESPMountPoint}, "/dev/vda1\n").
		OnArgsOutput([]string{"readlink", "-f", dmRoot}, nodeRoot+"\n").
		OnArgsOutput([]string{"readlink", "-f", lvRoot}, nodeRoot+"\n").
		OnArgsOutput([]string{"readlink", "-f", dmHome}, nodeHome+"\n").
		OnArgsOutput([]string{"readlink", "-f", lvHome}, nodeHome+"\n").
		OnArgsOutput([]string{"stat", "-c", "%d:%i"}, "1:2\n")
	t.Cleanup(primitives.SetCommandRunner(rec))
	return rec
}

func TestRun_LvmMountsRootAndHomeVolumes(t *testing.T) {
	rec := lvmMountRecorder(t)

	ctx := baseCtx()
	ctx.Lvm = true
	ctx.Partitions.VGName = "aelum-1a2b3c4d"
	ctx.Partitions.HomeDevice = "/dev/aelum-1a2b3c4d/home"
	ctx.Partitions.HomeUUID = "HOME-UUID"

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	activate := -1
	mountRoot := -1
	mountHome := -1
	for i, c := range rec.Calls() {
		joined := strings.Join(c.Args, " ")
		if joined == "vgchange -ay aelum-1a2b3c4d" {
			activate = i
		}
		if strings.HasPrefix(joined, "mount ") && strings.Contains(joined, "/dev/aelum-1a2b3c4d/root "+shared.MountRoot) {
			mountRoot = i
		}
		if strings.HasPrefix(joined, "mount ") && strings.Contains(joined, "/dev/aelum-1a2b3c4d/home "+shared.MountRoot+"/home") {
			mountHome = i
		}
	}
	if activate < 0 {
		t.Fatalf("expected the volume group to be activated, got %v", rec.Calls())
	}
	if mountRoot < 0 || mountHome < 0 {
		t.Fatalf("expected root and home LV mounts, got %v", rec.Calls())
	}
	if activate > mountRoot {
		t.Error("the volume group must be activated before the root LV is mounted")
	}
	if mountHome < mountRoot {
		t.Error("root must be mounted before the home volume")
	}
}

func TestRun_LvmBtrfsMountsSubvolumesFromRootLV(t *testing.T) {
	rec := lvmMountRecorder(t)

	ctx := baseCtx()
	ctx.FileSystem = shared.FileSystemBtrfs
	ctx.Lvm = true
	ctx.Partitions.VGName = "aelum-1a2b3c4d"
	for _, sv := range shared.GetBtrfsSubvolumes() {
		if sv.MountPoint != "/" {
			ctx.Partitions.MountPoints[sv.MountPoint] = "/dev/aelum-1a2b3c4d/root"
		}
	}

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	// Every subvolume mount must point at the root LV, never the raw partition.
	for _, c := range rec.CallsFor("mount") {
		if strings.Contains(strings.Join(c, " "), "/dev/vda2") {
			t.Errorf("btrfs subvolumes must mount from the root LV, got %v", c)
		}
	}
	if rec.Called("mkfs.ext4") {
		t.Error("no ext4 formatting should happen on the btrfs path")
	}
}

// With LUKS+LVM the mapper must be opened before the group is activated: the
// PV only exists once the decrypted device does.
func TestRun_LuksLvmOpensMapperBeforeActivating(t *testing.T) {
	rec := lvmMountRecorder(t)
	// cryptsetup status fails → the mapper is reopened.
	rec.OnArgsError([]string{"cryptsetup", "status"}, errors.New("not active"))

	ctx := baseCtx()
	ctx.Luks = true
	ctx.LuksPassphrase = []byte("secret")
	ctx.Lvm = true
	ctx.Partitions.RootMapper = shared.LuksMapperPath
	ctx.Partitions.VGName = "aelum-1a2b3c4d"

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	open := -1
	activate := -1
	for i, c := range rec.Calls() {
		joined := strings.Join(c.Args, " ")
		if strings.HasPrefix(joined, "cryptsetup open ") {
			open = i
		}
		if joined == "vgchange -ay aelum-1a2b3c4d" {
			activate = i
		}
	}
	if open < 0 || activate < 0 {
		t.Fatalf("expected cryptsetup open and vgchange -ay, got %v", rec.Calls())
	}
	if open > activate {
		t.Error("the LUKS mapper must be opened before the volume group is activated")
	}
}
