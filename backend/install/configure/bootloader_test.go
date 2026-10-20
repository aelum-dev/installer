package configure

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// --- bootEntryOptions (pure) -------------------------------------------------

// TestBootEntryOptions pins the kernel command line for every root layout: the
// root device (UUID / LUKS mapper / root LV), the rd.luks.name mapping when
// encrypted, and the btrfs subvol rootflag.
func TestBootEntryOptions(t *testing.T) {
	const (
		rootUUID = "ROOT-UUID"
		luksUUID = "LUKS-UUID"
	)
	suffix := " rw quiet splash loglevel=3 rd.udev.log_level=3 vt.global_cursor_default=0"

	tests := []struct {
		name string
		ctx  *shared.InstallContext
		want string
	}{
		{
			name: "plain ext4",
			ctx: &shared.InstallContext{
				FileSystem: shared.FileSystemExt4,
				Partitions: &shared.PartitionInfo{RootUUID: rootUUID},
			},
			want: "root=UUID=" + rootUUID + suffix,
		},
		{
			name: "btrfs adds subvol rootflag",
			ctx: &shared.InstallContext{
				FileSystem: shared.FileSystemBtrfs,
				Partitions: &shared.PartitionInfo{RootUUID: rootUUID},
			},
			want: "root=UUID=" + rootUUID + suffix + " rootflags=subvol=@",
		},
		{
			name: "luks adds rd.luks.name mapping",
			ctx: &shared.InstallContext{
				FileSystem: shared.FileSystemExt4,
				Luks:       true,
				Partitions: &shared.PartitionInfo{RootUUID: rootUUID, LuksUUID: luksUUID},
			},
			want: "rd.luks.name=" + luksUUID + "=" + shared.LuksMapperName + " root=UUID=" + rootUUID + suffix,
		},
		{
			name: "lvm roots on the root LV",
			ctx: &shared.InstallContext{
				FileSystem: shared.FileSystemExt4,
				Lvm:        true,
				Partitions: &shared.PartitionInfo{RootUUID: rootUUID},
			},
			want: "root=" + shared.LvmRootLvPath + suffix,
		},
		{
			name: "luks+lvm+btrfs combines all three",
			ctx: &shared.InstallContext{
				FileSystem: shared.FileSystemBtrfs,
				Luks:       true,
				Lvm:        true,
				Partitions: &shared.PartitionInfo{RootUUID: rootUUID, LuksUUID: luksUUID},
			},
			want: "rd.luks.name=" + luksUUID + "=" + shared.LuksMapperName + " root=" + shared.LvmRootLvPath + suffix + " rootflags=subvol=@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bootEntryOptions(tt.ctx); got != tt.want {
				t.Errorf("bootEntryOptions() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- refindConfig (pure) -----------------------------------------------------

func TestRefindConfig(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Luks:       true,
		Lvm:        true,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", LuksUUID: "LUKS-UUID"},
	}
	got := refindConfig(ctx, "vmlinuz-linux-lts", "initramfs-linux-lts.img")

	for _, want := range []string{
		"timeout 5",
		"default_selection " + shared.BootEntryTitle,
		"scan_all_linux_kernels false",
		"dont_scan_dirs EFI/systemd",
		"dont_scan_files systemd-bootx64.efi,BOOTX64.EFI,bootx64.efi",
		`menuentry "` + shared.BootEntryTitle + `" {`,
		"icon /" + shared.RefindIconPath,
		"loader /vmlinuz-linux-lts",
		"initrd /initramfs-linux-lts.img",
		fmt.Sprintf("    options %q", bootEntryOptions(ctx)),
		"rootflags=subvol=@",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("refind.conf missing %q:\n%s", want, got)
		}
	}
}

// --- discoverKernelFiles -----------------------------------------------------

func TestDiscoverKernelFiles_Success(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "/mnt/boot/vmlinuz-linux-lts\n")
	defer primitives.SetCommandRunner(rec)()

	kernel, initrd, err := discoverKernelFiles()
	if err != nil {
		t.Fatalf("discoverKernelFiles() = %v, want nil", err)
	}
	if kernel != "vmlinuz-linux-lts" {
		t.Errorf("kernel = %q, want vmlinuz-linux-lts", kernel)
	}
	if initrd != "initramfs-linux-lts.img" {
		t.Errorf("initrd = %q, want initramfs-linux-lts.img", initrd)
	}
}

func TestDiscoverKernelFiles_PrefersAelumKernelOnSharedESP(t *testing.T) {
	// A reused ESP can also hold another distribution's kernel. The Aelum
	// kernel must win even when find(1) lists the other one first.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux\nvmlinuz-linux-lts\n")
	defer primitives.SetCommandRunner(rec)()

	kernel, initrd, err := discoverKernelFiles()
	if err != nil {
		t.Fatalf("discoverKernelFiles() = %v, want nil", err)
	}
	if kernel != "vmlinuz-linux-lts" {
		t.Errorf("kernel = %q, want vmlinuz-linux-lts (not the other ESP kernel)", kernel)
	}
	if initrd != "initramfs-linux-lts.img" {
		t.Errorf("initrd = %q, want initramfs-linux-lts.img", initrd)
	}
}

func TestDiscoverKernelFiles_UsesFirstLineAndFallbackInitrd(t *testing.T) {
	// The main initrd check fails; the -fallback one succeeds.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux\nvmlinuz-lts\n").
		OnArgsError([]string{"test", "-f", shared.BootMountPoint + "/initramfs-linux.img"}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	kernel, initrd, err := discoverKernelFiles()
	if err != nil {
		t.Fatalf("discoverKernelFiles() = %v, want nil", err)
	}
	if kernel != "vmlinuz-linux" {
		t.Errorf("kernel = %q, want the first listed (vmlinuz-linux)", kernel)
	}
	if initrd != "initramfs-linux-fallback.img" {
		t.Errorf("initrd = %q, want the fallback image", initrd)
	}
}

func TestDiscoverKernelFiles_NoKernel(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"find", shared.BootMountPoint}, "")
	defer primitives.SetCommandRunner(rec)()

	if _, _, err := discoverKernelFiles(); err == nil {
		t.Fatal("discoverKernelFiles() = nil, want error for no kernel files")
	}
}

func TestDiscoverKernelFiles_FindFails(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("find", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	_, _, err := discoverKernelFiles()
	if err == nil || !strings.Contains(err.Error(), "failed to list kernel files") {
		t.Fatalf("discoverKernelFiles() error = %v, want list failure", err)
	}
}

func TestDiscoverKernelFiles_NoInitrd(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux\n").
		OnArgsError([]string{"test", "-f"}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	_, _, err := discoverKernelFiles()
	if err == nil || !strings.Contains(err.Error(), "no initrd found") {
		t.Fatalf("discoverKernelFiles() error = %v, want no-initrd", err)
	}
}

// --- command wrappers --------------------------------------------------------

func TestInstallRefind(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := installRefind(shared.BootMountPoint); err != nil {
		t.Fatalf("installRefind() = %v, want nil", err)
	}
	dest := shared.BootMountPoint + "/" + shared.RefindDir
	calls := rec.CallsFor("mkdir")
	wantMkdir := []string{"mkdir", "-p", dest}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], wantMkdir) {
		t.Fatalf("mkdir argv = %v, want %v", calls, wantMkdir)
	}
	calls = rec.CallsFor("cp")
	wantCp := []string{"cp", "-r", shared.RefindSourceDir + "/.", dest + "/"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], wantCp) {
		t.Fatalf("cp argv = %v, want %v", calls, wantCp)
	}
}

func TestInstallRefind_CopyFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("cp", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := installRefind(shared.BootMountPoint)
	if err == nil || !strings.Contains(err.Error(), "failed to copy rEFInd") {
		t.Fatalf("installRefind() error = %v, want copy failure", err)
	}
}

func TestInstallRefind_MkdirFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("mkdir", errors.New("read-only fs"))
	defer primitives.SetCommandRunner(rec)()

	err := installRefind(shared.BootMountPoint)
	if err == nil || !strings.Contains(err.Error(), "failed to create rEFInd directory") {
		t.Fatalf("installRefind() error = %v, want mkdir failure", err)
	}
}

func TestPartitionNumber(t *testing.T) {
	tests := []struct {
		disk, part string
		want       int
	}{
		{"/dev/vda", "/dev/vda1", 1},
		{"/dev/nvme0n1", "/dev/nvme0n1p12", 12},
		{"/dev/mmcblk0", "/dev/mmcblk0p2", 2},
		{"", "/dev/vda1", 0},
		{"/dev/vda", "", 0},
		{"/dev/vda", "/dev/sdb1", 0},
	}
	for _, tt := range tests {
		if got := partitionNumber(tt.disk, tt.part); got != tt.want {
			t.Errorf("partitionNumber(%q, %q) = %d, want %d", tt.disk, tt.part, got, tt.want)
		}
	}
}

func TestRegisterRefindBootEntry(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	want := []string{
		"efibootmgr", "--create",
		"--disk", "/dev/vda",
		"--part", "1",
		"--loader", shared.RefindNvramLoader,
		"--label", shared.BootEntryTitle,
	}
	if !hasCall(rec.CallsFor("efibootmgr"), want...) {
		t.Fatalf("efibootmgr create argv missing; calls = %v", rec.CallsFor("efibootmgr"))
	}
}

func TestRegisterRefindBootEntry_SkipsExistingEntry(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"}, `Boot0001* Aelum HD(1,GPT,ESP-PARTUUID,0x800,0x32000)/File(\EFI\refind\refind_x64.efi)`+"\n").
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "ESP-PARTUUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	for _, argv := range rec.CallsFor("efibootmgr") {
		if len(argv) > 1 && argv[1] == "--create" {
			t.Fatalf("efibootmgr --create ran despite an existing entry: %v", argv)
		}
	}
}

func TestRegisterRefindBootEntry_SkipsExistingEntryForAnotherESP(t *testing.T) {
	// Another disk's rEFInd has the same loader path but belongs to a different
	// ESP (different PARTUUID); Aelum's ESP still needs its own firmware entry.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"}, `Boot0002* rEFInd HD(1,GPT,OTHER-PARTUUID,0x800,0x32000)/File(\EFI\refind\refind_x64.efi)`+"\n").
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "AELUM-PARTUUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	found := false
	for _, argv := range rec.CallsFor("efibootmgr") {
		if len(argv) > 1 && argv[1] == "--create" {
			found = true
		}
	}
	if !found {
		t.Fatal("efibootmgr --create did not run for an entry belonging to another ESP")
	}
}

func TestRegisterRefindBootEntry_Errors(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"efibootmgr", "--create"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1"},
	}
	if err := registerRefindBootEntry(ctx); err == nil || !strings.Contains(err.Error(), "efibootmgr --create failed") {
		t.Fatalf("registerRefindBootEntry() error = %v, want efibootmgr failure", err)
	}

	// Missing EFI partition path is reported before touching firmware.
	ctx.Partitions = &shared.PartitionInfo{}
	if err := registerRefindBootEntry(ctx); err == nil || !strings.Contains(err.Error(), "EFI partition path not set") {
		t.Fatalf("registerRefindBootEntry() error = %v, want missing-EFI error", err)
	}
}

// --- verifyBootloaderInstallation --------------------------------------------

func TestVerifyBootloaderInstallation_Success(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()
	confPath := shared.BootMountPoint + "/" + shared.RefindConfigPath
	fs := primitivestest.NewMemFS().WithFile(confPath, `menuentry "`+shared.BootEntryTitle+`" {`+"\n}\n")
	defer primitives.SetFileSystem(fs)()

	if err := verifyBootloaderInstallation(shared.BootMountPoint); err != nil {
		t.Fatalf("verifyBootloaderInstallation() = %v, want nil", err)
	}
	binary := shared.BootMountPoint + "/" + shared.RefindBinaryPath
	probed := false
	for _, argv := range rec.CallsFor("test") {
		if reflect.DeepEqual(argv, []string{"test", "-f", binary}) {
			probed = true
		}
	}
	if !probed {
		t.Errorf("rEFInd binary existence was not probed; test calls = %v", rec.CallsFor("test"))
	}
}

func TestVerifyBootloaderInstallation_ReportsIssues(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"test", "-f"}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()
	defer primitives.SetFileSystem(primitivestest.NewMemFS())()

	err := verifyBootloaderInstallation(shared.BootMountPoint)
	if err == nil {
		t.Fatal("verifyBootloaderInstallation() = nil, want verification failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bootloader verification failed") ||
		!strings.Contains(msg, "rEFInd binary is MISSING") ||
		!strings.Contains(msg, "rEFInd config is MISSING") {
		t.Errorf("error = %q, want verification issues listed", msg)
	}
}

func TestVerifyBootloaderInstallation_MissingMenuEntry(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()
	confPath := shared.BootMountPoint + "/" + shared.RefindConfigPath
	fs := primitivestest.NewMemFS().WithFile(confPath, "timeout 5\n")
	defer primitives.SetFileSystem(fs)()

	err := verifyBootloaderInstallation(shared.BootMountPoint)
	if err == nil || !strings.Contains(err.Error(), "missing the "+shared.BootEntryTitle+" menu entry") {
		t.Fatalf("verifyBootloaderInstallation() error = %v, want missing-menuentry", err)
	}
}

// --- ConfigureBootloader -----------------------------------------------------

// validBootCtx is a context that passes every ConfigureBootloader precondition.
func validBootCtx() *shared.InstallContext {
	return &shared.InstallContext{
		DiskID:     "/dev/vda",
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIPart: "/dev/vda1"},
	}
}

func TestConfigureBootloader_NotMounted(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("findmnt", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := ConfigureBootloader(validBootCtx())
	if err == nil || !strings.Contains(err.Error(), "not mounted") {
		t.Fatalf("ConfigureBootloader() error = %v, want not mounted", err)
	}
}

func TestConfigureBootloader_Preconditions(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*shared.InstallContext)
		wantMsg string
	}{
		{"missing disk", func(c *shared.InstallContext) { c.DiskID = "" }, "disk ID not set"},
		{"nil partitions", func(c *shared.InstallContext) { c.Partitions = nil }, "partitions not initialized"},
		{"missing filesystem", func(c *shared.InstallContext) { c.FileSystem = "" }, "filesystem not set"},
		{"missing root UUID", func(c *shared.InstallContext) { c.Partitions.RootUUID = "" }, "root UUID not set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := primitivestest.NewRecorder().OnOutput("findmnt", "/dev/vda2\n")
			defer primitives.SetCommandRunner(rec)()

			ctx := validBootCtx()
			tt.mutate(ctx)
			err := ConfigureBootloader(ctx)
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("ConfigureBootloader() error = %v, want %q", err, tt.wantMsg)
			}
		})
	}
}

func TestConfigureBootloader_InstallFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("findmnt", "/dev/vda2\n").
		OnError("cp", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := ConfigureBootloader(validBootCtx())
	if err == nil || !strings.Contains(err.Error(), "failed to copy rEFInd") {
		t.Fatalf("ConfigureBootloader() error = %v, want rEFInd copy failure", err)
	}
}

func TestConfigureBootloader_Success(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnOutput("findmnt", "/dev/vda2\n").
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux-lts\n")
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureBootloader(validBootCtx()); err != nil {
		t.Fatalf("ConfigureBootloader() = %v, want nil", err)
	}
	content, ok := fs.Read(shared.BootMountPoint + "/" + shared.RefindConfigPath)
	if !ok {
		t.Fatalf("refind.conf not written at %s", shared.BootMountPoint+"/"+shared.RefindConfigPath)
	}
	for _, want := range []string{
		`menuentry "` + shared.BootEntryTitle + `"`,
		"loader /vmlinuz-linux-lts",
		"initrd /initramfs-linux-lts.img",
		`options "root=UUID=ROOT-UUID`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("refind.conf missing %q:\n%s", want, content)
		}
	}
}
