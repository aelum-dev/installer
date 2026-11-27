package configure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
			name: "btrfs names no subvolume",
			ctx: &shared.InstallContext{
				FileSystem: shared.FileSystemBtrfs,
				Partitions: &shared.PartitionInfo{RootUUID: rootUUID},
			},
			// The kernel mounts the filesystem's default subvolume: the subvolume the
			// installer installed into and the one snapper's rollback replaces. The entry
			// therefore needs no change after a rollback and stays byte-identical (see
			// format.root_snapshot).
			want: "root=UUID=" + rootUUID + suffix,
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
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", LuksUUID: "LUKS-UUID", RootPart: "/dev/sda2"},
	}
	includes := []string{"aelum/aaaa1111.conf", "aelum/bbbb2222.conf"}
	got := refindConfig(ctx, false, includes)

	for _, want := range []string{
		"resolution max",
		"timeout 5",
		fmt.Sprintf("default_selection %q", shared.RefindEntryTitle("/dev/sda2")),
		"scan_all_linux_kernels false",
		"dont_scan_files + BOOTX64.EFI,bootx64.efi",
		"use_graphics_for osx,linux",
		"include " + shared.RefindThemeInclude,
		"include aelum/aaaa1111.conf",
		"include aelum/bbbb2222.conf",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("refind.conf missing %q:\n%s", want, got)
		}
	}
	// Other OS loaders are auto-detected: rEFInd's own directory is skipped by
	// rEFInd, and only the fallback BOOTX64.EFI copies are suppressed.
	for _, unwanted := range []string{"dont_scan_dirs", "systemd-bootx64.efi", "menuentry"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("refind.conf unexpectedly contains %q:\n%s", unwanted, got)
		}
	}
	if strings.Contains(got, "font "+shared.RefindThemeFont2x) {
		t.Errorf("1080p refind.conf unexpectedly selects the 2x font:\n%s", got)
	}
}

func TestRefindEntry(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Luks:       true,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", LuksUUID: "LUKS-UUID", RootPart: "/dev/sda2"},
	}
	id := shared.InstallID(ctx.Partitions.RootUUID)
	got := refindEntry(ctx, id, "vmlinuz-linux-lts", "initramfs-linux-lts.img")

	for _, want := range []string{
		`menuentry "` + shared.RefindEntryTitle("/dev/sda2") + `" {`,
		"icon /" + shared.RefindIconPath,
		"loader /" + shared.InstallBootFilePath(id, "vmlinuz-linux-lts"),
		"initrd /" + shared.InstallBootFilePath(id, "initramfs-linux-lts.img"),
		fmt.Sprintf("    options %q", bootEntryOptions(ctx)),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rEFInd menu entry missing %q:\n%s", want, got)
		}
	}

	// No rootflags= in a btrfs entry: a pinned subvolume would make a rollback change
	// something the boot does not read.
	if strings.Contains(got, "rootflags=") {
		t.Errorf("rEFInd menu entry must not pin a subvolume:\n%s", got)
	}
}

func TestRefindConfig_HiDPI(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID"},
	}

	// The panel's width alone decides the sheet — rEFInd neither scales a custom
	// font nor measures the panel — and the 2x sheet has to land after the theme
	// include, because the last "font" token rEFInd reads is the one it uses.
	for _, tc := range []struct {
		name   string
		width  int
		want2x bool
	}{
		{"1080p", 1920, false},
		{"2K", 2560, true},
		{"4K", 3840, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// drmRoot is unread when a preferred mode is known, which is the
			// branch this table exercises.
			hiDPI := fontIsHiDPI(displayMode{Width: tc.width, Height: 1080}, true, "")
			got := refindConfig(ctx, hiDPI, nil)

			if !strings.Contains(got, "resolution max") {
				t.Fatalf("%s refind.conf missing \"resolution max\":\n%s", tc.name, got)
			}
			font := "font " + shared.RefindThemeFont2x
			if strings.Contains(got, font) != tc.want2x {
				t.Fatalf("%s: 2x font present=%v want=%v:\n%s", tc.name, !tc.want2x, tc.want2x, got)
			}
			if tc.want2x {
				theme := strings.Index(got, "include "+shared.RefindThemeInclude)
				if theme < 0 || strings.Index(got, font) < theme {
					t.Errorf("%s: the 2x font must be written after the theme include:\n%s", tc.name, got)
				}
			}
		})
	}
}

// TestFontIsHiDPIWithoutPreferredMode covers the fallback a live session takes
// when no connector answers with a preferred mode: the font still has to follow
// the panel, and the answer then comes from any connector's mode list.
func TestFontIsHiDPIWithoutPreferredMode(t *testing.T) {
	root := t.TempDir()
	writeDRMConnector(t, root, "card0-eDP-1", "connected", "enabled", "1920x1080\n3840x2160\n")
	if !fontIsHiDPI(displayMode{}, false, root) {
		t.Error("fontIsHiDPI() = false for a connector advertising 4K; want true")
	}

	root = t.TempDir()
	writeDRMConnector(t, root, "card0-eDP-1", "connected", "enabled", "1920x1080\n")
	if fontIsHiDPI(displayMode{}, false, root) {
		t.Error("fontIsHiDPI() = true for a 1080p-only connector; want false")
	}
}

func TestRefindConfig_NoDetectedMode(t *testing.T) {
	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID"},
	}
	got := refindConfig(ctx, false, nil)

	if !strings.Contains(got, "resolution max") {
		t.Fatalf("fallback refind.conf missing \"resolution max\":\n%s", got)
	}
	if strings.Contains(got, "font "+shared.RefindThemeFont2x) {
		t.Fatalf("1x fallback unexpectedly selects the 2x font:\n%s", got)
	}
}

func TestRefindEntryIncludes(t *testing.T) {
	dir := shared.BootMountPoint + "/" + shared.RefindEntriesDir
	fs := primitivestest.NewMemFS().
		WithFile(dir+"/bbbb2222.conf", "").
		WithFile(dir+"/aaaa1111.conf", "").
		WithFile(dir+"/bbbb2222-snapshots.conf", "").
		WithFile(dir+"/aaaa1111-snapshots.conf", "").
		WithFile(dir+"/cccc3333-snapshots.conf", "").
		WithFile(dir+"/notes.txt", "")
	defer primitives.SetFileSystem(fs)()

	got, err := refindEntryIncludes(shared.BootMountPoint)
	if err != nil {
		t.Fatalf("refindEntryIncludes() = %v, want nil", err)
	}
	// Each entry is followed by its own snapshots. Sorted by name alone, the
	// snapshots would come first: "-" sorts before ".".
	want := []string{
		"aelum/aaaa1111.conf",
		"aelum/aaaa1111-snapshots.conf",
		"aelum/bbbb2222.conf",
		"aelum/bbbb2222-snapshots.conf",
		"aelum/cccc3333-snapshots.conf",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("refindEntryIncludes() = %v, want %v", got, want)
	}

	// A first install has no fragment directory yet.
	defer primitives.SetFileSystem(primitivestest.NewMemFS())()
	if got, err := refindEntryIncludes(shared.BootMountPoint); err != nil || len(got) != 0 {
		t.Fatalf("refindEntryIncludes(empty) = %v, %v; want none", got, err)
	}
}

// --- display detection -------------------------------------------------------

func writeDRMConnector(t *testing.T, root, connector, status, enabled, modes string) {
	t.Helper()
	dir := filepath.Join(root, connector)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if status != "" {
		if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if enabled != "" {
		if err := os.WriteFile(filepath.Join(dir, "enabled"), []byte(enabled+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "modes"), []byte(modes), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPreferredDisplayMode(t *testing.T) {
	t.Run("uses first advertised mode", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-eDP-1", "connected", "enabled", "2560x1440\n3840x2160\n")
		mode, ok := preferredDisplayMode(root)
		if !ok || mode != (displayMode{Width: 2560, Height: 1440}) {
			t.Fatalf("preferredDisplayMode() = %+v, %v; want 2560x1440, true", mode, ok)
		}
	})

	t.Run("ignores disconnected connectors", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-eDP-1", "connected", "enabled", "1920x1080\n")
		writeDRMConnector(t, root, "card0-DP-1", "disconnected", "enabled", "3840x2160\n")
		mode, ok := preferredDisplayMode(root)
		if !ok || mode != (displayMode{Width: 1920, Height: 1080}) {
			t.Fatalf("preferredDisplayMode() = %+v, %v; want 1920x1080, true", mode, ok)
		}
	})

	t.Run("prefers largest enabled connector", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-eDP-1", "connected", "enabled", "1920x1080\n")
		writeDRMConnector(t, root, "card0-DP-1", "connected", "enabled", "3840x2160\n")
		mode, ok := preferredDisplayMode(root)
		if !ok || mode != (displayMode{Width: 3840, Height: 2160}) {
			t.Fatalf("preferredDisplayMode() = %+v, %v; want 3840x2160, true", mode, ok)
		}
	})

	t.Run("ignores disabled connectors", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-eDP-1", "connected", "enabled", "1920x1080\n")
		writeDRMConnector(t, root, "card0-DP-1", "connected", "disabled", "3840x2160\n")
		mode, ok := preferredDisplayMode(root)
		if !ok || mode != (displayMode{Width: 1920, Height: 1080}) {
			t.Fatalf("preferredDisplayMode() = %+v, %v; want 1920x1080, true", mode, ok)
		}
	})

	t.Run("falls back to connected connector", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-eDP-1", "connected", "disabled", "1920x1080\n")
		mode, ok := preferredDisplayMode(root)
		if !ok || mode != (displayMode{Width: 1920, Height: 1080}) {
			t.Fatalf("preferredDisplayMode() = %+v, %v; want 1920x1080, true", mode, ok)
		}
	})

	t.Run("missing sysfs", func(t *testing.T) {
		if _, ok := preferredDisplayMode(filepath.Join(t.TempDir(), "absent")); ok {
			t.Fatal("preferredDisplayMode() ok = true, want false when /sys/class/drm is absent")
		}
	})
}

func TestDisplayIsHiDPI(t *testing.T) {
	t.Run("4K", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-DP-1", "connected", "enabled", "1920x1080\n3840x2160\n")
		if !displayIsHiDPI(root) {
			t.Fatal("displayIsHiDPI() = false, want true for a 4K mode")
		}
	})

	t.Run("ultrawide", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-HDMI-A-1", "connected", "enabled", "3440x1440\n")
		if !displayIsHiDPI(root) {
			t.Fatal("displayIsHiDPI() = false, want true for a 3440px mode")
		}
	})

	t.Run("1080p", func(t *testing.T) {
		root := t.TempDir()
		writeDRMConnector(t, root, "card0-eDP-1", "connected", "enabled", "1920x1080\n1280x720\n")
		if displayIsHiDPI(root) {
			t.Fatal("displayIsHiDPI() = true, want false for a 1920px mode")
		}
	})

	t.Run("missing sysfs", func(t *testing.T) {
		if displayIsHiDPI(filepath.Join(t.TempDir(), "absent")) {
			t.Fatal("displayIsHiDPI() = true, want false when /sys/class/drm is absent")
		}
	})
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

func TestDiscoverKernelFiles_PrefersAelumKernel(t *testing.T) {
	// This installation's boot directory can hold more than one kernel image.
	// The Aelum kernel must win even when find(1) lists another one first.
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

func TestInstallRefindTheme(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	if err := installRefindTheme(shared.BootMountPoint); err != nil {
		t.Fatalf("installRefindTheme() = %v, want nil", err)
	}
	dest := shared.BootMountPoint + "/" + shared.RefindThemeDir
	calls := rec.CallsFor("mkdir")
	wantMkdir := []string{"mkdir", "-p", dest}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], wantMkdir) {
		t.Fatalf("mkdir argv = %v, want %v", calls, wantMkdir)
	}
	calls = rec.CallsFor("cp")
	wantCp := []string{"cp", "-r", shared.RefindThemeSourceDir + "/.", dest + "/"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], wantCp) {
		t.Fatalf("cp argv = %v, want %v", calls, wantCp)
	}
}

func TestInstallRefindTheme_MissingOnISO(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"test", "-d"}, errors.New("missing"))
	defer primitives.SetCommandRunner(rec)()

	err := installRefindTheme(shared.BootMountPoint)
	if err == nil || !strings.Contains(err.Error(), "theme not found on ISO") {
		t.Fatalf("installRefindTheme() error = %v, want missing-theme failure", err)
	}
	if calls := rec.CallsFor("cp"); len(calls) != 0 {
		t.Fatalf("cp ran without a theme source: %v", calls)
	}
}

func TestInstallRefindTheme_CopyFailure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("cp", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := installRefindTheme(shared.BootMountPoint)
	if err == nil || !strings.Contains(err.Error(), "failed to copy Aelum rEFInd theme") {
		t.Fatalf("installRefindTheme() error = %v, want copy failure", err)
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
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1", RootPart: "/dev/vda2"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	want := []string{
		"efibootmgr", "--create",
		"--disk", "/dev/vda",
		"--part", "1",
		"--loader", shared.RefindNvramLoader,
		"--label", shared.RefindEntryTitle("/dev/vda2"),
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
	if !bootCreateRan(rec) {
		t.Fatal("efibootmgr --create did not run for an entry belonging to another ESP")
	}
}

// bootCreateRan reports whether the recorder saw an efibootmgr --create call.
func bootCreateRan(rec *primitivestest.Recorder) bool {
	for _, argv := range rec.CallsFor("efibootmgr") {
		if len(argv) > 1 && argv[1] == "--create" {
			return true
		}
	}
	return false
}

func TestRegisterRefindBootEntry_CreatesWhenPartUUIDUnreadable(t *testing.T) {
	// A transient blkid failure must not be read as "this ESP's entry already
	// exists": the firmware can hold a foreign rEFInd entry, and skipping the
	// create would leave this install with no boot entry in the boot order —
	// silently, because registration is best-effort.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"}, `Boot0002* rEFInd HD(1,GPT,OTHER-PARTUUID,0x800,0x32000)/File(\EFI\refind\refind_x64.efi)`+"\n").
		OnArgsError([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, errors.New("blkid: I/O error while reading /dev/vda1"))
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	if !bootCreateRan(rec) {
		t.Fatal("efibootmgr --create did not run although the ESP's PARTUUID was unreadable")
	}
}

func TestRegisterRefindBootEntry_DoesNotMixSignalsFromDifferentEntries(t *testing.T) {
	// Both signals appear in efibootmgr -v, but on different entries: a foreign
	// rEFInd (right loader, wrong ESP) and this ESP's stale Windows entry
	// (right ESP, wrong loader). Neither line is this install's entry, so it
	// has to be created.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"},
			`Boot0001* Windows Boot Manager HD(1,GPT,AELUM-PARTUUID,0x800,0x32000)/File(\EFI\Microsoft\Boot\bootmgfw.efi)`+"\n"+
				`Boot0002* rEFInd HD(1,GPT,OTHER-PARTUUID,0x800,0x32000)/File(\EFI\refind\refind_x64.efi)`+"\n").
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "AELUM-PARTUUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	if !bootCreateRan(rec) {
		t.Fatal("efibootmgr --create did not run although loader and PARTUUID belonged to different entries")
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

// nvramListing is a realistic `efibootmgr -v` for the case that matters: this
// ESP's rEFInd entry exists, but the firmware would still boot the installation
// medium, because the DVD entry is the head of BootOrder.
const nvramListing = `BootCurrent: 0002
Timeout: 0 seconds
BootOrder: 0002,0007,0001
Boot0001* EFI Internal Shell	FV(70692445-c28b-4d0a-87ff-4c7a44f6e5f3)
Boot0002* UEFI QEMU DVD-ROM QM00005	PciRoot(0x0)/Pci(0x1f,0x2)/CDROM(1,0x0,0x800)
Boot0007* Aelum (vda2)	HD(1,GPT,esp-partuuid,0x800,0x200000)/File(\EFI\refind\refind_x64.efi)
`

// nvramListingForeign is the same firmware with no entry for this ESP yet.
const nvramListingForeign = `BootCurrent: 0002
Timeout: 0 seconds
BootOrder: 0002,0001
Boot0001* EFI Internal Shell	FV(70692445-c28b-4d0a-87ff-4c7a44f6e5f3)
Boot0002* UEFI QEMU DVD-ROM QM00005	PciRoot(0x0)/Pci(0x1f,0x2)/CDROM(1,0x0,0x800)
`

func TestRegisterRefindBootEntry_PromotesExistingEntry(t *testing.T) {
	// The regression this pins: a reinstall over a reused ESP keeps that ESP's
	// PARTUUID, so the previous install's entry still matches and nothing needs
	// creating. Having an entry is not enough — it sits behind the medium in the
	// boot order, and reboot lands on the live ISO instead of the new system.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"}, nvramListing).
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "ESP-PARTUUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1", RootPart: "/dev/vda2"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	want := []string{"efibootmgr", "--bootorder", "0007,0002,0001"}
	if !hasCall(rec.CallsFor("efibootmgr"), want...) {
		t.Fatalf("efibootmgr --bootorder argv missing; calls = %v", rec.CallsFor("efibootmgr"))
	}
	if bootCreateRan(rec) {
		t.Fatal("efibootmgr --create ran despite an existing entry")
	}
}

func TestRegisterRefindBootEntry_LeavesFirstEntryAlone(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"},
			strings.Replace(nvramListing, "BootOrder: 0002,0007,0001", "BootOrder: 0007,0002,0001", 1)).
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "ESP-PARTUUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1", RootPart: "/dev/vda2"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	if bootOrderRan(rec) {
		t.Fatal("efibootmgr --bootorder ran although the entry is already first")
	}
}

func TestRegisterRefindBootEntry_PromotesCreatedEntry(t *testing.T) {
	// Nothing matches this ESP yet, so the entry is created. efibootmgr names it
	// on stdout, and that number is what moves ahead of the DVD entry.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"}, nvramListingForeign).
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "AELUM-PARTUUID\n").
		OnArgsOutput([]string{"efibootmgr", "--create"},
			`Boot000A* Aelum (vda2)	HD(1,GPT,aelum-partuuid,0x800,0x200000)/File(\EFI\refind\refind_x64.efi)`+"\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1", RootPart: "/dev/vda2"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	want := []string{"efibootmgr", "--bootorder", "000a,0002,0001"}
	if !hasCall(rec.CallsFor("efibootmgr"), want...) {
		t.Fatalf("efibootmgr --bootorder argv missing; calls = %v", rec.CallsFor("efibootmgr"))
	}
}

func TestRegisterRefindBootEntry_NoBootOrderWritesNothing(t *testing.T) {
	// A firmware holding no BootOrder is booting devices in its own order.
	// Writing an order that lists only Aelum would leave it nothing to fall back
	// to if rEFInd itself failed to load, so the entry is left to the firmware.
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"},
			`Boot0007* Aelum (vda2)	HD(1,GPT,esp-partuuid,0x800,0x200000)/File(\EFI\refind\refind_x64.efi)`+"\n").
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "ESP-PARTUUID\n")
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1", RootPart: "/dev/vda2"},
	}
	if err := registerRefindBootEntry(ctx); err != nil {
		t.Fatalf("registerRefindBootEntry() = %v, want nil", err)
	}
	if bootOrderRan(rec) || bootCreateRan(rec) {
		t.Fatalf("efibootmgr wrote the boot order or created an entry: %v", rec.CallsFor("efibootmgr"))
	}
}

func TestRegisterRefindBootEntry_PromoteFails(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"efibootmgr", "-v"}, nvramListing).
		OnArgsOutput([]string{"blkid", "-s", "PARTUUID", "-o", "value", "/dev/vda1"}, "ESP-PARTUUID\n").
		OnArgsError([]string{"efibootmgr", "--bootorder"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{EFIPart: "/dev/vda1", RootPart: "/dev/vda2"},
	}
	if err := registerRefindBootEntry(ctx); err == nil || !strings.Contains(err.Error(), "efibootmgr --bootorder failed") {
		t.Fatalf("registerRefindBootEntry() error = %v, want efibootmgr failure", err)
	}
}

// bootOrderRan reports whether the recorder saw an efibootmgr --bootorder call.
func bootOrderRan(rec *primitivestest.Recorder) bool {
	for _, argv := range rec.CallsFor("efibootmgr") {
		if len(argv) > 1 && argv[1] == "--bootorder" {
			return true
		}
	}
	return false
}

// --- parseBootState / bootOrderWithFirst (pure) ------------------------------

func TestParseBootState(t *testing.T) {
	order, entry := parseBootState(nvramListing, "ESP-PARTUUID\n")
	if entry != "0007" {
		t.Errorf("parseBootState() entry = %q, want 0007", entry)
	}
	if got := strings.Join(order, ","); got != "0002,0007,0001" {
		t.Errorf("parseBootState() order = %q, want 0002,0007,0001", got)
	}

	// A foreign ESP's PARTUUID identifies no entry here, but the order is still
	// read — the create path needs both halves.
	order, entry = parseBootState(nvramListing, "OTHER-PARTUUID")
	if entry != "" {
		t.Errorf("parseBootState() entry for a foreign PARTUUID = %q, want empty", entry)
	}
	if got := strings.Join(order, ","); got != "0002,0007,0001" {
		t.Errorf("parseBootState() order = %q, want 0002,0007,0001", got)
	}

	// An unreadable PARTUUID matches nothing, so the caller creates an entry.
	if _, entry = parseBootState(nvramListing, ""); entry != "" {
		t.Errorf("parseBootState() entry without a PARTUUID = %q, want empty", entry)
	}
}

func TestBootOrderWithFirst(t *testing.T) {
	tests := []struct {
		name    string
		order   []string
		bootnum string
		want    string
	}{
		{"moves a later entry to the head", []string{"0002", "0007", "0001"}, "0007", "0007,0002,0001"},
		{"an entry already first changes nothing", []string{"0007", "0002"}, "0007", "0007,0002"},
		{"a later duplicate is dropped", []string{"0002", "0007", "0007"}, "0007", "0007,0002"},
		{"an entry absent from the order is added", []string{"0002"}, "000a", "000a,0002"},
		{"case is not significant", []string{"0002", "000A"}, "000a", "000a,0002"},
	}
	for _, tt := range tests {
		got := strings.Join(bootOrderWithFirst(tt.order, tt.bootnum), ",")
		if got != tt.want {
			t.Errorf("%s: bootOrderWithFirst(%v, %q) = %q, want %q", tt.name, tt.order, tt.bootnum, got, tt.want)
		}
	}
}

// --- verifyBootloaderInstallation --------------------------------------------

func TestVerifyBootloaderInstallation_Success(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()
	ctx := &shared.InstallContext{Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", RootPart: "/dev/vda2"}}
	id := shared.InstallID(ctx.Partitions.RootUUID)
	entryPath := shared.BootMountPoint + "/" + shared.RefindEntryPath(id)
	confPath := shared.BootMountPoint + "/" + shared.RefindConfigPath
	fs := primitivestest.NewMemFS().
		WithFile(confPath, "include "+shared.RefindThemeInclude+"\ninclude "+shared.RefindEntryInclude(id)+"\n").
		WithFile(entryPath, `menuentry "`+shared.RefindEntryTitle("/dev/vda2")+`" {`+"\n}\n")
	defer primitives.SetFileSystem(fs)()

	if err := verifyBootloaderInstallation(shared.BootMountPoint, ctx); err != nil {
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
	ctx := &shared.InstallContext{Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", RootPart: "/dev/vda2"}}

	err := verifyBootloaderInstallation(shared.BootMountPoint, ctx)
	if err == nil {
		t.Fatal("verifyBootloaderInstallation() = nil, want verification failure")
	}
	msg := err.Error()
	for _, want := range []string{
		"bootloader verification failed",
		"rEFInd binary is MISSING",
		"rEFInd menu entry is MISSING",
		"rEFInd config is MISSING",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error = %q, want %q", msg, want)
		}
	}
}

func TestVerifyBootloaderInstallation_MissingMenuEntry(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()
	ctx := &shared.InstallContext{Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", RootPart: "/dev/vda2"}}
	id := shared.InstallID(ctx.Partitions.RootUUID)
	fs := primitivestest.NewMemFS().
		WithFile(shared.BootMountPoint+"/"+shared.RefindEntryPath(id), "timeout 5\n")
	defer primitives.SetFileSystem(fs)()

	err := verifyBootloaderInstallation(shared.BootMountPoint, ctx)
	if err == nil || !strings.Contains(err.Error(), "does not contain "+shared.RefindEntryTitle("/dev/vda2")) {
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
	ctx := validBootCtx()
	ctx.Partitions.RootPart = "/dev/vda2"

	if err := ConfigureBootloader(ctx); err != nil {
		t.Fatalf("ConfigureBootloader() = %v, want nil", err)
	}

	id := shared.InstallID(ctx.Partitions.RootUUID)
	confContent, ok := fs.Read(shared.ESPMountPoint + "/" + shared.RefindConfigPath)
	if !ok {
		t.Fatalf("refind.conf not written at %s", shared.ESPMountPoint+"/"+shared.RefindConfigPath)
	}
	for _, want := range []string{
		"include " + shared.RefindThemeInclude,
		"include " + shared.RefindEntryInclude(id),
	} {
		if !strings.Contains(confContent, want) {
			t.Errorf("refind.conf missing %q:\n%s", want, confContent)
		}
	}
	if strings.Contains(confContent, "menuentry") {
		t.Errorf("refind.conf should not hold the menu entry itself:\n%s", confContent)
	}

	entryContent, ok := fs.Read(shared.ESPMountPoint + "/" + shared.RefindEntryPath(id))
	if !ok {
		t.Fatalf("menu entry not written at %s", shared.ESPMountPoint+"/"+shared.RefindEntryPath(id))
	}
	for _, want := range []string{
		`menuentry "` + shared.RefindEntryTitle("/dev/vda2") + `"`,
		"loader /" + shared.InstallBootFilePath(id, "vmlinuz-linux-lts"),
		"initrd /" + shared.InstallBootFilePath(id, "initramfs-linux-lts.img"),
		`options "root=UUID=ROOT-UUID`,
	} {
		if !strings.Contains(entryContent, want) {
			t.Errorf("rEFInd menu entry missing %q:\n%s", want, entryContent)
		}
	}
}
