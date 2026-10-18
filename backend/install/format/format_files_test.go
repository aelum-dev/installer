package format

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// fakeLsblk writes an executable "lsblk" shim onto PATH returning the given JSON
// so shared.GetPartitions (which shells out for real) sees a scripted disk.
func fakeLsblk(t *testing.T, json string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "lsblk")
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat <<'JSON'\n"+json+"\nJSON\n"), 0o755); err != nil {
		t.Fatalf("write fake lsblk: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const (
	lsblkExt4 = `{"blockdevices":[{"name":"vda","type":"disk","size":"21474836480","children":[` +
		`{"name":"vda1","type":"part","size":"134217728","fstype":"vfat","uuid":"EFI-UUID"},` +
		`{"name":"vda2","type":"part","size":"21340618752","fstype":"ext4","uuid":"ROOT-UUID"}]}]}`
	lsblkNoEFI    = `{"blockdevices":[{"name":"vda","type":"disk","size":"21474836480","children":[]}]}`
	lsblkNoFstype = `{"blockdevices":[{"name":"vda","type":"disk","size":"21474836480","children":[` +
		`{"name":"vda1","type":"part","size":"134217728"},` +
		`{"name":"vda2","type":"part","size":"21340618752"}]}]}`
)

// --- ReadPartitionUUID -------------------------------------------------------

func TestReadPartitionUUID(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "  ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	got, err := ReadPartitionUUID("/dev/vda2")
	if err != nil {
		t.Fatalf("ReadPartitionUUID() = %v", err)
	}
	if got != "ROOT-UUID" {
		t.Errorf("ReadPartitionUUID() = %q, want ROOT-UUID", got)
	}
}

func TestReadPartitionUUID_Error(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("blkid", errors.New("exit status 2"))
	defer primitives.SetCommandRunner(rec)()

	if _, err := ReadPartitionUUID("/dev/vda2"); err == nil || !strings.Contains(err.Error(), "blkid") {
		t.Fatalf("ReadPartitionUUID() error = %v, want blkid failure", err)
	}
}

func TestReadPartitionUUID_Empty(t *testing.T) {
	rec := primitivestest.NewRecorder().OnOutput("blkid", "  \n")
	defer primitives.SetCommandRunner(rec)()

	if _, err := ReadPartitionUUID("/dev/vda2"); err == nil || !strings.Contains(err.Error(), "no filesystem UUID") {
		t.Fatalf("ReadPartitionUUID() error = %v, want an empty-UUID failure", err)
	}
}

// --- NewPartitionInfo / applyRootSetup ---------------------------------------

func TestNewPartitionInfo(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda1"}, "EFI-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	rootLV := shared.LvmRootLvPath
	homeLV := shared.LvmHomeLvPath

	tests := []struct {
		name        string
		ctx         *shared.InstallContext
		setup       RootSetup
		fsType      string
		wantMapper  bool
		wantLuks    string
		wantRootLV  string
		wantHomeLV  string
		wantSubvols map[string]string // mount point -> device (only entries that must be present)
	}{
		{
			name:   "plain ext4",
			ctx:    &shared.InstallContext{},
			setup:  RootSetup{RootDevice: "/dev/vda2", RootUUID: "ROOT-UUID"},
			fsType: shared.FileSystemExt4,
		},
		{
			name:   "plain btrfs has subvolume map",
			ctx:    &shared.InstallContext{},
			setup:  RootSetup{RootDevice: "/dev/vda2", RootUUID: "ROOT-UUID"},
			fsType: shared.FileSystemBtrfs,
			wantSubvols: map[string]string{
				"/home":      "/dev/vda2",
				"/var/cache": "/dev/vda2",
			},
		},
		{
			name:       "luks ext4 records mapper and luks uuid",
			ctx:        &shared.InstallContext{Luks: true},
			setup:      RootSetup{RootDevice: shared.LuksMapperPath, RootUUID: "ROOT-UUID", LuksUUID: "LUKS-UUID"},
			fsType:     shared.FileSystemExt4,
			wantMapper: true,
			wantLuks:   "LUKS-UUID",
		},
		{
			name:       "lvm ext4 records lvs without subvolumes",
			ctx:        &shared.InstallContext{Lvm: true},
			setup:      RootSetup{RootDevice: rootLV, RootUUID: "ROOT-UUID", HomeLV: homeLV, HomeUUID: "HOME-UUID"},
			fsType:     shared.FileSystemExt4,
			wantRootLV: rootLV,
			wantHomeLV: homeLV,
		},
		{
			name:       "lvm btrfs splits root and home subvolumes",
			ctx:        &shared.InstallContext{Lvm: true},
			setup:      RootSetup{RootDevice: rootLV, RootUUID: "ROOT-UUID", HomeLV: homeLV, HomeUUID: "HOME-UUID"},
			fsType:     shared.FileSystemBtrfs,
			wantRootLV: rootLV,
			wantHomeLV: homeLV,
			wantSubvols: map[string]string{
				"/home":      homeLV,
				"/var/cache": rootLV,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := NewPartitionInfo(tt.ctx, "/dev/vda", "/dev/vda1", "/dev/vda2", tt.setup, tt.fsType)
			if err != nil {
				t.Fatalf("NewPartitionInfo() = %v", err)
			}
			if info.Disk != "/dev/vda" || info.EFIPart != "/dev/vda1" || info.RootPart != "/dev/vda2" {
				t.Errorf("identity fields wrong: %+v", info)
			}
			if info.EFIUUID != "EFI-UUID" || info.RootUUID != "ROOT-UUID" {
				t.Errorf("UUID fields wrong: efi=%q root=%q", info.EFIUUID, info.RootUUID)
			}
			if tt.wantMapper && info.RootMapper != shared.LuksMapperPath {
				t.Errorf("RootMapper = %q, want %q", info.RootMapper, shared.LuksMapperPath)
			}
			if info.LuksUUID != tt.wantLuks {
				t.Errorf("LuksUUID = %q, want %q", info.LuksUUID, tt.wantLuks)
			}
			if info.RootLV != tt.wantRootLV {
				t.Errorf("RootLV = %q, want %q", info.RootLV, tt.wantRootLV)
			}
			if info.HomeLV != tt.wantHomeLV {
				t.Errorf("HomeLV = %q, want %q", info.HomeLV, tt.wantHomeLV)
			}
			for mp, dev := range tt.wantSubvols {
				if info.MountPoints[mp] != dev {
					t.Errorf("MountPoints[%q] = %q, want %q", mp, info.MountPoints[mp], dev)
				}
			}
			if len(tt.wantSubvols) == 0 && len(info.MountPoints) != 0 {
				t.Errorf("MountPoints = %v, want empty", info.MountPoints)
			}
		})
	}
}

func TestNewPartitionInfo_EFIUUIDError(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("blkid", errors.New("exit status 2"))
	defer primitives.SetCommandRunner(rec)()

	if _, err := NewPartitionInfo(&shared.InstallContext{}, "/dev/vda", "/dev/vda1", "/dev/vda2", RootSetup{RootUUID: "R"}, shared.FileSystemExt4); err == nil {
		t.Fatal("NewPartitionInfo() = nil, want blkid failure")
	}
}

// --- SetupRootFilesystem -----------------------------------------------------

func TestSetupRootFilesystem_Ext4(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	setup, err := SetupRootFilesystem(&shared.InstallContext{}, "/dev/vda2", shared.FileSystemExt4)
	if err != nil {
		t.Fatalf("SetupRootFilesystem() = %v", err)
	}
	if setup.RootDevice != "/dev/vda2" || setup.RootUUID != "ROOT-UUID" {
		t.Errorf("setup = %+v", setup)
	}
	if !hasArgv(rec.CallsFor("mkfs.ext4"), "mkfs.ext4", "-F", "-E", "lazy_itable_init=0", "-L", "aelum", "/dev/vda2") {
		t.Errorf("mkfs.ext4 not called correctly: %v", rec.CallsFor("mkfs.ext4"))
	}
}

func TestSetupRootFilesystem_Btrfs(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n").
		OnArgsOutput([]string{"btrfs", "subvolume", "list"}, "ID 256 gen 1 top level 5 path @\n")
	defer primitives.SetCommandRunner(rec)()

	setup, err := SetupRootFilesystem(&shared.InstallContext{}, "/dev/vda2", shared.FileSystemBtrfs)
	if err != nil {
		t.Fatalf("SetupRootFilesystem() = %v", err)
	}
	if setup.RootUUID != "ROOT-UUID" {
		t.Errorf("RootUUID = %q", setup.RootUUID)
	}
	if !rec.Called("mkfs.btrfs") || !rec.Called("btrfs") {
		t.Error("btrfs setup commands not run")
	}
}

func TestSetupRootFilesystem_FormatFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("wipefs", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if _, err := SetupRootFilesystem(&shared.InstallContext{}, "/dev/vda2", shared.FileSystemExt4); err == nil {
		t.Fatal("SetupRootFilesystem() = nil, want wipefs failure")
	}
}

// --- applyLvm ----------------------------------------------------------------

func TestApplyLvm_Btrfs(t *testing.T) {
	rootLV := shared.LvmRootLvPath
	homeLV := shared.LvmHomeLvPath
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"blockdev", "--getsize64"}, "34359738368\n"). // 32 GiB
		OnArgsOutput([]string{"vgs"}, "32768\n").                           // 32 GiB in MiB
		OnArgsOutput([]string{"btrfs", "subvolume", "list"}, "ID 256 gen 1 top level 5 path @\nID 257 gen 1 top level 5 path @home\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", rootLV}, "ROOT-LV-UUID\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", homeLV}, "HOME-LV-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	setup, err := applyLvm(&shared.InstallContext{}, "/dev/vda2", shared.FileSystemBtrfs)
	if err != nil {
		t.Fatalf("applyLvm() = %v", err)
	}
	if setup.RootDevice != rootLV || setup.HomeLV != homeLV {
		t.Errorf("devices wrong: root=%q home=%q", setup.RootDevice, setup.HomeLV)
	}
	if setup.RootUUID != "ROOT-LV-UUID" || setup.HomeUUID != "HOME-LV-UUID" {
		t.Errorf("UUIDs wrong: root=%q home=%q", setup.RootUUID, setup.HomeUUID)
	}
	for _, name := range []string{"pvcreate", "vgcreate", "lvcreate"} {
		if !rec.Called(name) {
			t.Errorf("%s not called", name)
		}
	}
}

func TestApplyLvm_DeviceTooSmall(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"blockdev", "--getsize64"}, "1073741824\n") // 1 GiB
	defer primitives.SetCommandRunner(rec)()

	if _, err := applyLvm(&shared.InstallContext{}, "/dev/vda2", shared.FileSystemExt4); err == nil {
		t.Fatal("applyLvm() = nil, want device-too-small error")
	}
}

// --- EncryptRoot -------------------------------------------------------------

func TestEncryptRoot(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", shared.LuksMapperPath}, "ROOT-UUID\n").
		OnArgsOutput([]string{"cryptsetup", "luksUUID"}, "LUKS-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	setup, err := EncryptRoot(&shared.InstallContext{}, "/dev/vda2", "secret", shared.FileSystemExt4)
	if err != nil {
		t.Fatalf("EncryptRoot() = %v", err)
	}
	if setup.RootDevice != shared.LuksMapperPath || setup.RootUUID != "ROOT-UUID" || setup.LuksUUID != "LUKS-UUID" {
		t.Errorf("setup = %+v", setup)
	}

	// The passphrase must be fed via stdin (never argv) and the mapper must be
	// closed again.
	for _, c := range rec.Calls() {
		for _, a := range c.Args {
			if a == "secret" {
				t.Errorf("passphrase leaked into argv: %v", c.Args)
			}
		}
	}
	formatCall, ok := findCall(rec, "cryptsetup", "luksFormat")
	if !ok || formatCall.Stdin != "secret" {
		t.Errorf("luksFormat stdin = %q, want secret", formatCall.Stdin)
	}
	if !rec.Called("cryptsetup") || !hasArgv(rec.CallsFor("cryptsetup"), "cryptsetup", "close", shared.LuksMapperName) {
		t.Error("luks mapper not closed")
	}
}

func TestEncryptRoot_FormatFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"cryptsetup", "luksFormat"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if _, err := EncryptRoot(&shared.InstallContext{}, "/dev/vda2", "secret", shared.FileSystemExt4); err == nil {
		t.Fatal("EncryptRoot() = nil, want luksFormat failure")
	}
}

// --- Run ---------------------------------------------------------------------

func TestRun_PlainExt4(t *testing.T) {
	fakeLsblk(t, lsblkExt4)
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda1"}, "EFI-UUID\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if ctx.Partitions == nil {
		t.Fatal("Partitions not populated")
	}
	if ctx.Partitions.EFIPart != "/dev/vda1" || ctx.Partitions.EFIUUID != "EFI-UUID" {
		t.Errorf("EFI info wrong: %+v", ctx.Partitions)
	}
	if ctx.Partitions.RootPart != "/dev/vda2" || ctx.Partitions.RootUUID != "ROOT-UUID" {
		t.Errorf("root info wrong: %+v", ctx.Partitions)
	}

	for _, want := range [][]string{
		{"sgdisk", "-Z", "/dev/vda"},
		{"sgdisk", "-n", "1:0:+512M"},
		{"sgdisk", "-n", "2:0:0"},
		{"partprobe", "/dev/vda"},
	} {
		if !hasArgvPrefix(rec.CallsFor(want[0]), want...) {
			t.Errorf("command %v not run; got %v", want, rec.CallsFor(want[0]))
		}
	}
	if !rec.Called("mkfs.fat") || !rec.Called("mkfs.ext4") {
		t.Error("filesystems not created")
	}
}

func TestRun_ProbesPartitionTableBeforeFormattingESP(t *testing.T) {
	fakeLsblk(t, lsblkExt4)
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda1"}, "EFI-UUID\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	probe, format := -1, -1
	for i, c := range rec.Calls() {
		switch c.Args[0] {
		case "partprobe":
			if probe == -1 {
				probe = i
			}
		case "mkfs.fat":
			if format == -1 {
				format = i
			}
		}
	}
	if probe == -1 || format == -1 {
		t.Fatalf("partprobe=%d mkfs.fat=%d, want both to run", probe, format)
	}
	if probe > format {
		t.Errorf("partprobe ran at %d, after mkfs.fat at %d; the new partition nodes may not exist yet", probe, format)
	}
}

func TestRun_EFIByIndexFallback(t *testing.T) {
	fakeLsblk(t, lsblkNoFstype)
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda1"}, "EFI-UUID\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if ctx.Partitions.EFIPart != "/dev/vda1" {
		t.Errorf("EFIPart = %q, want /dev/vda1 (index fallback)", ctx.Partitions.EFIPart)
	}
}

func TestRun_NoEFIPartition(t *testing.T) {
	fakeLsblk(t, lsblkNoEFI)
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	err := Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "EFI (vfat) partition not found") {
		t.Fatalf("Run() error = %v, want EFI-not-found", err)
	}
}

func TestRun_Preconditions(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *shared.InstallContext
		wantMsg string
	}{
		{"no disk", &shared.InstallContext{}, "disk ID not set"},
		{"bad filesystem", &shared.InstallContext{DiskID: "/dev/vda", FileSystem: "xfs"}, "unsupported filesystem"},
		{"luks without passphrase", &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4, Luks: true}, "no passphrase"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeLsblk(t, lsblkExt4)
			defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
			err := Run(tt.ctx)
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Run() error = %v, want %q", err, tt.wantMsg)
			}
		})
	}
}

func TestRun_SgdiskFails(t *testing.T) {
	fakeLsblk(t, lsblkExt4)
	rec := primitivestest.NewRecorder().OnArgsError([]string{"sgdisk", "-Z"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	err := Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "sgdisk -Z failed") {
		t.Fatalf("Run() error = %v, want sgdisk -Z failure", err)
	}
}

// --- small helpers -----------------------------------------------------------

func hasArgv(calls [][]string, want ...string) bool {
	return hasArgvPrefix(calls, want...)
}

func hasArgvPrefix(calls [][]string, want ...string) bool {
	for _, argv := range calls {
		if len(argv) < len(want) {
			continue
		}
		match := true
		for i := range want {
			if argv[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func findCall(rec *primitivestest.Recorder, prefix ...string) (primitivestest.Call, bool) {
	for _, c := range rec.Calls() {
		if hasArgvPrefix([][]string{c.Args}, prefix...) {
			return c, true
		}
	}
	return primitivestest.Call{}, false
}
