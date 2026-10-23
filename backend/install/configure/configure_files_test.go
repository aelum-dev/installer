package configure

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// mountedRunner is a CommandRunner whose findmnt check passes, so the steps that
// require a mounted /mnt run without one.
func mountedRunner() *primitivestest.Recorder {
	return primitivestest.NewRecorder().OnOutput("findmnt", "/dev/vda2\n")
}

// hasCall reports whether calls contains argv exactly.
func hasCall(calls [][]string, want ...string) bool {
	for _, argv := range calls {
		if len(argv) != len(want) {
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

// --- GenerateFstab -----------------------------------------------------------

func TestGenerateFstab(t *testing.T) {
	base := func(fs string, mutate func(*shared.InstallContext)) *shared.InstallContext {
		ctx := &shared.InstallContext{
			DiskID:     "/dev/vda",
			FileSystem: fs,
			Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID"},
		}
		if mutate != nil {
			mutate(ctx)
		}
		return ctx
	}

	tests := []struct {
		name string
		ctx  *shared.InstallContext
		want []string
	}{
		{
			name: "ext4",
			ctx:  base(shared.FileSystemExt4, nil),
			want: []string{"UUID=ROOT-UUID", "UUID=EFI-UUID", "ext4", " / "},
		},
		{
			name: "btrfs",
			ctx:  base(shared.FileSystemBtrfs, nil),
			want: []string{"UUID=ROOT-UUID", "subvol=@", "btrfs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := mountedRunner()
			defer primitives.SetCommandRunner(rec)()
			fs := primitivestest.NewMemFS()
			defer primitives.SetFileSystem(fs)()

			if err := GenerateFstab(tt.ctx); err != nil {
				t.Fatalf("GenerateFstab() = %v, want nil", err)
			}
			content, ok := fs.Read(shared.MountRoot + "/etc/fstab")
			if !ok {
				t.Fatal("fstab was not written")
			}
			for _, want := range append(tt.want,
				shared.InstalledESPMountPoint,
				shared.InstalledESPMountPoint+"/"+shared.InstallBootDir(shared.InstallID(tt.ctx.Partitions.RootUUID)),
				"/boot  none  bind",
			) {
				if !strings.Contains(content, want) {
					t.Errorf("fstab missing %q:\n%s", want, content)
				}
			}
		})
	}
}

func TestGenerateFstab_Preconditions(t *testing.T) {
	valid := func() *shared.InstallContext {
		return &shared.InstallContext{
			DiskID:     "/dev/vda",
			FileSystem: shared.FileSystemExt4,
			Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID"},
		}
	}

	tests := []struct {
		name    string
		mounted bool
		mutate  func(*shared.InstallContext)
		wantMsg string
	}{
		{"not mounted", false, nil, "not mounted"},
		{"no disk", true, func(c *shared.InstallContext) { c.DiskID = "" }, "disk ID not set"},
		{"no partitions", true, func(c *shared.InstallContext) { c.Partitions = nil }, "partitions not initialized"},
		{"no filesystem", true, func(c *shared.InstallContext) { c.FileSystem = "" }, "filesystem not set"},
		{"no UUIDs", true, func(c *shared.InstallContext) { c.Partitions.RootUUID = "" }, "UUIDs not set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := primitivestest.NewRecorder()
			if tt.mounted {
				rec.OnOutput("findmnt", "/dev/vda2\n")
			} else {
				rec.OnError("findmnt", errors.New("exit status 1"))
			}
			defer primitives.SetCommandRunner(rec)()
			defer primitives.SetFileSystem(primitivestest.NewMemFS())()

			ctx := valid()
			if tt.mutate != nil {
				tt.mutate(ctx)
			}
			err := GenerateFstab(ctx)
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("GenerateFstab() error = %v, want %q", err, tt.wantMsg)
			}
		})
	}
}

func TestGenerateFstab_WriteError(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS().WithWriteError(shared.MountRoot+"/etc/fstab", errors.New("disk full"))
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID"},
	}
	if err := GenerateFstab(ctx); err == nil {
		t.Fatal("GenerateFstab() = nil, want write error")
	}
}

// --- verifyFstab -------------------------------------------------------------

func TestVerifyFstab(t *testing.T) {
	const path = shared.MountRoot + "/etc/fstab"
	valid := "UUID=ROOT-UUID  /  ext4  defaults  0 1\n" +
		"UUID=EFI-UUID  /efi  vfat  umask=0077 0 2\n" +
		"/efi/aelum/rootuuid  /boot  none  bind,x-systemd.requires-mounts-for=/efi  0 0\n"

	tests := []struct {
		name      string
		content   string
		root, efi string
		wantMsg   string
	}{
		{"ok", valid, "ROOT-UUID", "EFI-UUID", ""},
		{"empty root", valid, "", "EFI-UUID", "root UUID is empty"},
		{"empty efi", valid, "ROOT-UUID", "", "EFI UUID is empty"},
		{"too short", "ROOT-UUID EFI-UUID", "ROOT-UUID", "EFI-UUID", "too short"},
		{"missing root", valid, "OTHER", "EFI-UUID", "missing root UUID OTHER"},
		{"missing efi", valid, "ROOT-UUID", "OTHER", "missing EFI UUID OTHER"},
		{
			name:    "missing root mount point",
			content: "UUID=ROOT-UUID\nUUID=EFI-UUID\n/efi/aelum/rootuuid /boot none bind\n" + strings.Repeat("x", 50),
			root:    "ROOT-UUID",
			efi:     "EFI-UUID",
			wantMsg: "missing / mount point",
		},
		{
			name:    "missing per-install boot directory",
			content: "UUID=ROOT-UUID  /  ext4  defaults  0 1\nUUID=EFI-UUID  /efi  vfat  umask=0077 0 2\n" + strings.Repeat("x", 50),
			root:    "ROOT-UUID",
			efi:     "EFI-UUID",
			wantMsg: "missing this installation's boot directory",
		},
		{
			name:    "missing boot bind mount",
			content: "UUID=ROOT-UUID  /  ext4  defaults  0 1\nUUID=EFI-UUID  /efi  vfat  umask=0077 0 2\n/efi/aelum/rootuuid  /boot  none  defaults  0 0\n",
			root:    "ROOT-UUID",
			efi:     "EFI-UUID",
			wantMsg: "missing the /boot bind mount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := primitivestest.NewMemFS().WithFile(path, tt.content)
			defer primitives.SetFileSystem(fs)()

			err := verifyFstab(tt.root, tt.efi, "rootuuid")
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("verifyFstab() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("verifyFstab() error = %v, want %q", err, tt.wantMsg)
			}
		})
	}
}

func TestVerifyFstab_ReadError(t *testing.T) {
	defer primitives.SetFileSystem(primitivestest.NewMemFS())()
	err := verifyFstab("ROOT-UUID", "EFI-UUID", "rootuuid")
	if err == nil || !strings.Contains(err.Error(), "failed to read fstab") {
		t.Fatalf("verifyFstab() error = %v, want read failure", err)
	}
}

// --- writeRefindConfig -------------------------------------------------------

func TestWriteRefindConfig(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", RootPart: "/dev/sda2"},
	}
	if err := writeRefindConfig(shared.BootMountPoint, ctx, "vmlinuz-linux-lts", "initramfs-linux-lts.img", displayMode{Width: 1920, Height: 1080}, true, false); err != nil {
		t.Fatalf("writeRefindConfig() = %v, want nil", err)
	}

	id := shared.InstallID(ctx.Partitions.RootUUID)
	path := shared.BootMountPoint + "/" + shared.RefindConfigPath
	content, ok := fs.Read(path)
	if !ok {
		t.Fatalf("rEFInd config not written at %s", path)
	}
	for _, want := range []string{
		"include " + shared.RefindThemeInclude,
		"include " + shared.RefindEntryInclude(id),
	} {
		if !strings.Contains(content, want) {
			t.Errorf("refind.conf missing %q:\n%s", want, content)
		}
	}

	entryPath := shared.BootMountPoint + "/" + shared.RefindEntryPath(id)
	entry, ok := fs.Read(entryPath)
	if !ok {
		t.Fatalf("rEFInd menu entry not written at %s", entryPath)
	}
	for _, want := range []string{
		`menuentry "` + shared.RefindEntryTitle("/dev/sda2") + `"`,
		"icon /" + shared.RefindIconPath,
		"loader /" + shared.InstallBootFilePath(id, "vmlinuz-linux-lts"),
		"initrd /" + shared.InstallBootFilePath(id, "initramfs-linux-lts.img"),
		`options "root=UUID=ROOT-UUID`,
	} {
		if !strings.Contains(entry, want) {
			t.Errorf("menu entry missing %q:\n%s", want, entry)
		}
	}
}

// TestWriteRefindConfig_PreservesOtherInstalls covers installing Aelum twice on
// the same ESP: the second install must add its own fragment without deleting
// the first install's entry.
func TestWriteRefindConfig_PreservesOtherInstalls(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	firstCtx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "1111AAAA-0000-0000-0000-000000000000", RootPart: "/dev/sda2"},
	}
	firstID := shared.InstallID(firstCtx.Partitions.RootUUID)
	firstEntryPath := shared.BootMountPoint + "/" + shared.RefindEntryPath(firstID)
	fs := primitivestest.NewMemFS().WithFile(firstEntryPath, `menuentry "Aelum (sda2)" {`+"\n}\n")
	defer primitives.SetFileSystem(fs)()

	secondCtx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "2222BBBB-0000-0000-0000-000000000000", RootPart: "/dev/sda3"},
	}
	if err := writeRefindConfig(shared.BootMountPoint, secondCtx, "vmlinuz-linux-lts", "initramfs-linux-lts.img", displayMode{}, false, false); err != nil {
		t.Fatalf("writeRefindConfig() = %v, want nil", err)
	}

	if _, ok := fs.Read(firstEntryPath); !ok {
		t.Fatalf("first install's menu entry was removed: %s", firstEntryPath)
	}
	conf, _ := fs.Read(shared.BootMountPoint + "/" + shared.RefindConfigPath)
	for _, id := range []string{firstID, shared.InstallID(secondCtx.Partitions.RootUUID)} {
		if !strings.Contains(conf, "include "+shared.RefindEntryInclude(id)) {
			t.Errorf("refind.conf missing include for %s:\n%s", id, conf)
		}
	}

	secondID := shared.InstallID(secondCtx.Partitions.RootUUID)
	secondEntry, ok := fs.Read(shared.BootMountPoint + "/" + shared.RefindEntryPath(secondID))
	if !ok {
		t.Fatalf("second install's menu entry was not written")
	}
	if want := "loader /" + shared.InstallBootFilePath(secondID, "vmlinuz-linux-lts"); !strings.Contains(secondEntry, want) {
		t.Errorf("second install's entry missing %q:\n%s", want, secondEntry)
	}
	if strings.Contains(secondEntry, shared.InstallBootFilePath(firstID, "vmlinuz-linux-lts")) {
		t.Errorf("second install's entry points at the first install's kernel:\n%s", secondEntry)
	}
}

func TestWriteRefindConfig_WriteFails(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	path := shared.BootMountPoint + "/" + shared.RefindConfigPath
	fs := primitivestest.NewMemFS().WithWriteError(path, errors.New("read-only fs"))
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID"}}
	err := writeRefindConfig(shared.BootMountPoint, ctx, "vmlinuz-linux", "initramfs-linux.img", displayMode{}, false, false)
	if err == nil || !strings.Contains(err.Error(), "failed to write rEFInd config") {
		t.Fatalf("writeRefindConfig() error = %v, want write failure", err)
	}
}

// --- ConfigurePlymouth -------------------------------------------------------

func TestConfigurePlymouth_Full(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePlymouth(&shared.InstallContext{Username: "alice"}); err != nil {
		t.Fatalf("ConfigurePlymouth() = %v, want nil", err)
	}

	conf, ok := fs.Read(shared.MountRoot + "/etc/plymouth/plymouthd.conf")
	if !ok || !strings.Contains(conf, "Theme=aelum") {
		t.Errorf("plymouthd.conf not written with the aelum theme: %q", conf)
	}

	unit, ok := fs.Read(shared.MountRoot + bootProgressUnitPath)
	if !ok || !strings.Contains(unit, "boot:complete") {
		t.Errorf("boot-progress unit not written: %q", unit)
	}

	if !hasCall(rec.CallsFor("cp"), "cp", "-a", aelumThemeISOPath+"/.", shared.MountRoot+"/usr/share/plymouth/themes/aelum/") {
		t.Errorf("theme copy not issued: %v", rec.CallsFor("cp"))
	}
	if !rec.Called("ln") {
		t.Error("default theme symlink not created")
	}
	if !hasCall(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "plymouth-set-default-theme", "-R", "aelum") {
		t.Errorf("plymouth-set-default-theme not run: %v", rec.CallsFor("arch-chroot"))
	}
	if !hasCall(rec.CallsFor("touch"), "touch", shared.MountRoot+"/home/alice/.hushlogin") {
		t.Errorf(".hushlogin not created: %v", rec.CallsFor("touch"))
	}
}

func TestConfigurePlymouth_PlymouthdWriteFails(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS().WithWriteError(shared.MountRoot+"/etc/plymouth/plymouthd.conf", errors.New("read-only fs"))
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePlymouth(&shared.InstallContext{}); err == nil {
		t.Fatal("ConfigurePlymouth() = nil, want write error")
	}
}

func TestInstallBootProgressUnit(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := installBootProgressUnit(&shared.InstallContext{}); err != nil {
		t.Fatalf("installBootProgressUnit() = %v, want nil", err)
	}
	unit, ok := fs.Read(shared.MountRoot + bootProgressUnitPath)
	if !ok || !strings.Contains(unit, "ExecStart=-/usr/bin/plymouth update --status=boot:complete") {
		t.Errorf("unit content wrong: %q", unit)
	}
	if !hasCall(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "systemctl", "enable", "aelum-boot-progress.service") {
		t.Errorf("unit not enabled: %v", rec.CallsFor("arch-chroot"))
	}

	// Enabling is best-effort: a failure must not fail the step.
	rec2 := primitivestest.NewRecorder().OnError("arch-chroot", errors.New("no systemd"))
	rec2Restore := primitives.SetCommandRunner(rec2)
	defer rec2Restore()
	if err := installBootProgressUnit(&shared.InstallContext{}); err != nil {
		t.Errorf("installBootProgressUnit() = %v, want nil even when enabling fails", err)
	}
}

func TestInstallBootProgressUnit_WriteFails(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	fs := primitivestest.NewMemFS().WithWriteError(shared.MountRoot+bootProgressUnitPath, errors.New("disk full"))
	defer primitives.SetFileSystem(fs)()

	if err := installBootProgressUnit(&shared.InstallContext{}); err == nil {
		t.Fatal("installBootProgressUnit() = nil, want write error")
	}
}

// --- ConfigureZram / calculateZramSizeMiB ------------------------------------

func TestCalculateZramSizeMiB(t *testing.T) {
	tests := []struct {
		name string
		fs   *primitivestest.MemFS
		want int
	}{
		{"read error defaults", primitivestest.NewMemFS(), 2048},
		{"no MemTotal line defaults", primitivestest.NewMemFS().WithFile("/proc/meminfo", "SwapTotal: 0 kB\n"), 2048},
		{"4 GiB → 2 GiB", primitivestest.NewMemFS().WithFile("/proc/meminfo", "MemTotal:        4194304 kB\n"), 2048},
		{"8 GiB → 4 GiB", primitivestest.NewMemFS().WithFile("/proc/meminfo", "MemTotal:        8388608 kB\n"), 4096},
		{"32 GiB → capped at 8 GiB", primitivestest.NewMemFS().WithFile("/proc/meminfo", "MemTotal:       33554432 kB\n"), zramSizeMaxMiB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			if got := calculateZramSizeMiB(); got != tt.want {
				t.Errorf("calculateZramSizeMiB() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConfigureZram(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS().WithFile("/proc/meminfo", "MemTotal:       16777216 kB\n")
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureZram(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigureZram() = %v, want nil", err)
	}
	conf, ok := fs.Read(shared.MountRoot + "/etc/systemd/zram-generator.conf")
	if !ok || !strings.Contains(conf, "zram-size = 8192") {
		t.Errorf("zram config wrong: %q", conf)
	}
}

// --- EnableServices ----------------------------------------------------------

func TestEnableServices(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()

	if err := EnableServices(&shared.InstallContext{}); err != nil {
		t.Fatalf("EnableServices() = %v, want nil", err)
	}
	for _, svc := range append(append([]string{}, requiredServices...), optionalServices...) {
		if !hasCall(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "systemctl", "enable", svc) {
			t.Errorf("service %q not enabled: %v", svc, rec.CallsFor("arch-chroot"))
		}
	}

	rec2 := primitivestest.NewRecorder().
		OnOutput("findmnt", "/dev/vda2\n").
		OnArgsError([]string{"arch-chroot", shared.MountRoot, "systemctl", "enable", "greetd"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec2)()
	if err := EnableServices(&shared.InstallContext{}); err == nil || !strings.Contains(err.Error(), "greetd") {
		t.Fatalf("EnableServices() error = %v, want a greetd failure", err)
	}

	// An optional service (bluetooth) failing must not abort the install.
	rec3 := primitivestest.NewRecorder().
		OnOutput("findmnt", "/dev/vda2\n").
		OnArgsError([]string{"arch-chroot", shared.MountRoot, "systemctl", "enable", "bluetooth"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec3)()
	if err := EnableServices(&shared.InstallContext{}); err != nil {
		t.Fatalf("EnableServices() = %v, want nil when an optional service fails", err)
	}
}

// --- ConfigureSystemBase -----------------------------------------------------

func TestConfigureSystemBase(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS().WithFile(shared.MountRoot+"/etc/locale.gen", "#en_US.UTF-8 UTF-8\n")
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{
		Username: "alice",
		Hostname: "aelum",
		Region:   shared.Regions["en_US"],
	}
	if err := ConfigureSystemBase(ctx); err != nil {
		t.Fatalf("ConfigureSystemBase() = %v, want nil", err)
	}

	if got, _ := fs.Read(shared.MountRoot + "/etc/hostname"); got != "aelum\n" {
		t.Errorf("hostname = %q", got)
	}
	if got, _ := fs.Read(shared.MountRoot + "/etc/locale.gen"); !strings.Contains(got, "en_US.UTF-8 UTF-8") || strings.Contains(got, "#en_US.UTF-8") {
		t.Errorf("locale.gen not uncommented: %q", got)
	}
	if got, _ := fs.Read(shared.MountRoot + "/etc/locale.conf"); got != "LANG=en_US.UTF-8\n" {
		t.Errorf("locale.conf = %q", got)
	}
	vconsole, _ := fs.Read(shared.MountRoot + "/etc/vconsole.conf")
	if !strings.Contains(vconsole, "KEYMAP=us") || !strings.Contains(vconsole, "FONT=ter-132n") {
		t.Errorf("vconsole.conf = %q", vconsole)
	}
	mirror, ok := fs.Read(shared.MountRoot + "/etc/pacman.d/mirrorlist")
	if !ok || !strings.Contains(mirror, "Server = ") {
		t.Errorf("mirrorlist not written/uncommented (ok=%v):\n%s", ok, mirror)
	}
}

func TestConfigureSystemBase_Preconditions(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*shared.InstallContext)
		wantMsg string
	}{
		{"no username", func(c *shared.InstallContext) { c.Username = "" }, "username not set"},
		{"no region", func(c *shared.InstallContext) { c.Region.Code = "" }, "region not set"},
		{"no hostname", func(c *shared.InstallContext) { c.Hostname = "" }, "hostname not set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := mountedRunner()
			defer primitives.SetCommandRunner(rec)()
			defer primitives.SetFileSystem(primitivestest.NewMemFS())()

			ctx := &shared.InstallContext{Username: "alice", Hostname: "aelum", Region: shared.Regions["en_US"]}
			tt.mutate(ctx)
			err := ConfigureSystemBase(ctx)
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error = %v, want %q", err, tt.wantMsg)
			}
		})
	}
}

func TestConfigureSystemBase_LocaleNotInGen(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS().WithFile(shared.MountRoot+"/etc/locale.gen", "#de_DE.UTF-8 UTF-8\n")
	defer primitives.SetFileSystem(fs)()

	// A locale.gen without the locale must not abort the install after the
	// (long) pacstrap steps: append the UTF-8 line and let locale-gen verify it.
	ctx := &shared.InstallContext{Username: "alice", Hostname: "aelum", Region: shared.Regions["en_US"]}
	if err := ConfigureSystemBase(ctx); err != nil {
		t.Fatalf("ConfigureSystemBase() = %v, want nil", err)
	}
	got, _ := fs.Read(shared.MountRoot + "/etc/locale.gen")
	if !strings.Contains(got, "en_US.UTF-8 UTF-8") {
		t.Errorf("locale.gen missing appended locale: %q", got)
	}
}

func TestConfigureSystemBase_AlreadyEnabledLocale(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS().WithFile(shared.MountRoot+"/etc/locale.gen", "en_US.UTF-8 UTF-8\n#de_DE.UTF-8 UTF-8\n")
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{Username: "alice", Hostname: "aelum", Region: shared.Regions["en_US"]}
	if err := ConfigureSystemBase(ctx); err != nil {
		t.Fatalf("ConfigureSystemBase() = %v, want nil for an already-enabled locale", err)
	}
	got, _ := fs.Read(shared.MountRoot + "/etc/locale.gen")
	if strings.Count(got, "en_US.UTF-8 UTF-8") != 1 {
		t.Errorf("locale.gen should contain the locale exactly once: %q", got)
	}
}

func TestEnableLocale_Forms(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"commented no space", "#en_US.UTF-8 UTF-8\n", "en_US.UTF-8 UTF-8\n"},
		{"commented with space", "# en_US.UTF-8 UTF-8\n", "en_US.UTF-8 UTF-8\n"},
		{"already enabled", "en_US.UTF-8 UTF-8\n", "en_US.UTF-8 UTF-8\n"},
		{"appended when absent", "#de_DE.UTF-8 UTF-8\n", "#de_DE.UTF-8 UTF-8\nen_US.UTF-8 UTF-8\n"},
		{"similar locale untouched", "#en_US.ISO-8859-1 ISO-8859-1\n", "#en_US.ISO-8859-1 ISO-8859-1\nen_US.UTF-8 UTF-8\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := enableLocale(tt.content, "en_US.UTF-8"); got != tt.want {
				t.Errorf("enableLocale() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- configureInitramfs ------------------------------------------------------

func TestConfigureInitramfs(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *shared.InstallContext
		content string
		want    []string
	}{
		{
			name:    "plain",
			ctx:     &shared.InstallContext{},
			content: "MODULES=()\nHOOKS=(base udev autodetect)\n",
			want:    []string{"HOOKS=(base systemd plymouth", "filesystems fsck)"},
		},
		{
			name:    "luks",
			ctx:     &shared.InstallContext{Luks: true},
			content: "MODULES=()\nHOOKS=(base)\n",
			want:    []string{"sd-encrypt", "filesystems fsck)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := primitivestest.NewRecorder()
			defer primitives.SetCommandRunner(rec)()
			fs := primitivestest.NewMemFS().WithFile(shared.MountRoot+"/etc/mkinitcpio.conf", tt.content)
			defer primitives.SetFileSystem(fs)()

			if err := configureInitramfs(tt.ctx); err != nil {
				t.Fatalf("configureInitramfs() = %v, want nil", err)
			}
			got, _ := fs.Read(shared.MountRoot + "/etc/mkinitcpio.conf")
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("mkinitcpio.conf missing %q:\n%s", want, got)
				}
			}
			if !hasCall(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "mkinitcpio", "-P") {
				t.Errorf("initramfs not regenerated: %v", rec.CallsFor("arch-chroot"))
			}
		})
	}
}

func TestConfigureInitramfs_NoHooks(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	defer primitives.SetFileSystem(primitivestest.NewMemFS().WithFile(shared.MountRoot+"/etc/mkinitcpio.conf", "MODULES=()\n"))()

	err := configureInitramfs(&shared.InstallContext{})
	if err == nil || !strings.Contains(err.Error(), "HOOKS not found") {
		t.Fatalf("error = %v, want HOOKS-not-found", err)
	}
}

// --- installLuksHint ---------------------------------------------------------

func TestInstallLuksHint(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := installLuksHint(&shared.InstallContext{}); err != nil {
		t.Fatalf("installLuksHint() = %v, want nil", err)
	}

	for _, p := range []string{luksHintScriptPath, luksHintServicePath, luksPoweroffUnitPath, luksCryptsetupDropInPath(), luksHintInstallHookPath} {
		if _, ok := fs.Read(shared.MountRoot + p); !ok {
			t.Errorf("LUKS hint file not written: %s", p)
		}
	}
	if mode := fs.Mode(shared.MountRoot + luksHintScriptPath); mode.Perm() != 0o755 {
		t.Errorf("watcher mode = %v, want 0755", mode.Perm())
	}
}

func TestInstallLuksHint_WriteFails(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	fs := primitivestest.NewMemFS().WithWriteError(shared.MountRoot+luksHintServicePath, errors.New("disk full"))
	defer primitives.SetFileSystem(fs)()

	if err := installLuksHint(&shared.InstallContext{}); err == nil {
		t.Fatal("installLuksHint() = nil, want write error")
	}
}

func TestInstallLuksHint_ChmodFails(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	fs := primitivestest.NewMemFS().WithChmodError(shared.MountRoot+luksHintScriptPath, errors.New("operation not permitted"))
	defer primitives.SetFileSystem(fs)()

	if err := installLuksHint(&shared.InstallContext{}); err == nil {
		t.Fatal("installLuksHint() = nil, want chmod error")
	}
}

// --- ConfigureGreetd ---------------------------------------------------------

func TestConfigureGreetd(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureGreetd(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigureGreetd() = %v, want nil", err)
	}
	if _, ok := fs.Read(shared.MountRoot + "/etc/greetd/config.toml"); !ok {
		t.Error("greetd config.toml not written")
	}
	for _, argv := range [][]string{
		{"arch-chroot", shared.MountRoot, "systemd-sysusers", "--root=/", "--no-pager"},
		{"arch-chroot", shared.MountRoot, "systemd-tmpfiles", "--create", "--root=/"},
	} {
		if !hasCall(rec.CallsFor("arch-chroot"), argv...) {
			t.Errorf("expected command not run: %v", argv)
		}
	}
}

func TestConfigureGreetd_Failure(t *testing.T) {
	rec := primitivestest.NewRecorder().OnError("arch-chroot", errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()
	defer primitives.SetFileSystem(primitivestest.NewMemFS())()

	if err := ConfigureGreetd(&shared.InstallContext{}); err == nil {
		t.Fatal("ConfigureGreetd() = nil, want failure")
	}
}
