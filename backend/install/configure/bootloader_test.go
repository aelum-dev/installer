package configure

import (
	"errors"
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

// --- applyLoaderConfDefaults (pure) ------------------------------------------

func TestApplyLoaderConfDefaults(t *testing.T) {
	const entry = "aelum.conf"
	// BootLoaderTimeout is 0 today; build the expected timeout line from the
	// constant so the test tracks it.
	timeout := "timeout 0"

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty appends both",
			content: "",
			want:    "default " + entry + "\n" + timeout + "\n",
		},
		{
			name:    "replaces existing default",
			content: "default old.conf\n",
			want:    "default " + entry + "\n" + timeout + "\n",
		},
		{
			name:    "replaces existing timeout",
			content: "timeout 9\n",
			want:    timeout + "\ndefault " + entry + "\n",
		},
		{
			name:    "replaces both in place",
			content: "default old.conf\ntimeout 9\neditor 1\n",
			want:    "default " + entry + "\n" + timeout + "\neditor 1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyLoaderConfDefaults(tt.content, entry); got != tt.want {
				t.Errorf("applyLoaderConfDefaults(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
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

func TestInstallSystemdBoot(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := installSystemdBoot(shared.BootMountPoint); err != nil {
		t.Fatalf("installSystemdBoot() = %v, want nil", err)
	}
	calls := rec.CallsFor("bootctl")
	want := []string{"bootctl", "install", "--esp-path=" + shared.BootMountPoint, "--graceful", "--efi-boot-option-description=" + shared.BootOptionDescription}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("bootctl argv = %v, want %v", calls, want)
	}

	rec.Reset()
	rec.OnError("bootctl", errors.New("exit status 1"))
	if err := installSystemdBoot(shared.BootMountPoint); err == nil {
		t.Fatal("installSystemdBoot() = nil, want error")
	}
}

func TestCleanupEsp(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := cleanupEsp(shared.BootMountPoint); err != nil {
		t.Fatalf("cleanupEsp() = %v, want nil", err)
	}
	calls := rec.CallsFor("bootctl")
	want := []string{"bootctl", "cleanup", "--esp-path=" + shared.BootMountPoint, "--graceful"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("bootctl argv = %v, want %v", calls, want)
	}
}

func TestGenerateRandomSeed(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := generateRandomSeed(shared.BootMountPoint); err != nil {
		t.Fatalf("generateRandomSeed() = %v, want nil", err)
	}
	calls := rec.CallsFor("bootctl")
	want := []string{"bootctl", "random-seed", "--esp-path=" + shared.BootMountPoint}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("bootctl argv = %v, want %v", calls, want)
	}

	rec.Reset()
	rec.OnError("bootctl", errors.New("exit status 1"))
	if err := generateRandomSeed(shared.BootMountPoint); err == nil {
		t.Fatal("generateRandomSeed() = nil, want error")
	}
}

// --- verifyBootloaderInstallation --------------------------------------------

func TestVerifyBootloaderInstallation_Success(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := verifyBootloaderInstallation(shared.BootMountPoint); err != nil {
		t.Fatalf("verifyBootloaderInstallation() = %v, want nil", err)
	}
	// The entry file existence probe must have run.
	entry := shared.BootMountPoint + "/" + shared.BootLoaderEntriesDir + "/" + shared.BootEntryFileName + ".conf"
	probed := false
	for _, argv := range rec.CallsFor("test") {
		if reflect.DeepEqual(argv, []string{"test", "-f", entry}) {
			probed = true
		}
	}
	if !probed {
		t.Errorf("entry existence was not probed; test calls = %v", rec.CallsFor("test"))
	}
}

func TestVerifyBootloaderInstallation_ReportsIssues(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsError([]string{"bootctl", "is-installed"}, errors.New("exit status 1")).
		OnArgsError([]string{"test", "-f"}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	err := verifyBootloaderInstallation(shared.BootMountPoint)
	if err == nil {
		t.Fatal("verifyBootloaderInstallation() = nil, want verification failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bootloader verification failed") ||
		!strings.Contains(msg, "loader.conf is MISSING") ||
		!strings.Contains(msg, "boot entry file is MISSING") {
		t.Errorf("error = %q, want verification issues listed", msg)
	}
}

// --- ConfigureBootloader -----------------------------------------------------

// validBootCtx is a context that passes every ConfigureBootloader precondition.
func validBootCtx() *shared.InstallContext {
	return &shared.InstallContext{
		DiskID:     "/dev/vda",
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID"},
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
		OnArgsError([]string{"bootctl", "install"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := ConfigureBootloader(validBootCtx())
	if err == nil || !strings.Contains(err.Error(), "bootctl install failed") {
		t.Fatalf("ConfigureBootloader() error = %v, want bootctl install failure", err)
	}
}
