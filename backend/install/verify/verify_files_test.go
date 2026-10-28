package verify

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// recorderMounted returns a CommandRunner whose findmnt check passes and whose
// kernel/initrd probes succeed, so Run exercises the file-driven checks.
func recorderMounted() *primitivestest.Recorder {
	return primitivestest.NewRecorder().
		OnOutput("findmnt", "/dev/vda2\n").
		OnArgsOutput([]string{"find", shared.BootMountPoint}, "vmlinuz-linux-lts\n")
}

// --- Run end to end ----------------------------------------------------------

func TestRun_PassesWhenEverythingMatches(t *testing.T) {
	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Username:   "alice",
		Hostname:   "aelum",
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID", RootPart: "/dev/vda2"},
	}
	id := shared.InstallID(ctx.Partitions.RootUUID)
	fstab := "UUID=ROOT-UUID / ext4 rw 0 1\n" +
		"UUID=EFI-UUID /efi vfat rw 0 2\n" +
		"/efi/" + shared.InstallBootDir(id) + " /boot none bind 0 0\n"
	fs := primitivestest.NewMemFS().
		WithFile(shared.MountRoot+"/etc/fstab", fstab).
		WithFile(shared.MountRoot+"/etc/passwd", "root:x:0:0::/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/bash\n").
		WithFile(shared.MountRoot+"/etc/hostname", "aelum\n").
		WithFile(shared.ESPMountPoint+"/"+shared.RefindThemeConfigPath, "banner themes/aelum/background.png\n").
		WithFile(shared.ESPMountPoint+"/"+shared.RefindConfigPath, "include "+shared.RefindThemeInclude+"\ninclude "+shared.RefindEntryInclude(id)+"\n").
		WithFile(shared.ESPMountPoint+"/"+shared.RefindEntryPath(id), `menuentry "`+shared.RefindEntryTitle("/dev/vda2")+`" {`+"\n    loader /"+shared.InstallBootFilePath(id, "vmlinuz-linux-lts")+"\n    options root=UUID=ROOT-UUID\n}\n")
	defer primitives.SetFileSystem(fs)()
	rec := recorderMounted()
	defer primitives.SetCommandRunner(rec)()

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
}

func TestRun_PassesWithLuks(t *testing.T) {
	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Username:   "alice",
		Hostname:   "aelum",
		FileSystem: shared.FileSystemBtrfs,
		Luks:       true,
		Partitions: &shared.PartitionInfo{
			RootUUID: "ROOT-UUID",
			EFIUUID:  "EFI-UUID",
			LuksUUID: "LUKS-UUID",
			RootPart: "/dev/vda2",
		},
	}
	id := shared.InstallID(ctx.Partitions.RootUUID)
	entry := `menuentry "` + shared.RefindEntryTitle("/dev/vda2") + `" {` +
		"\n    loader /" + shared.InstallBootFilePath(id, "vmlinuz-linux-lts") +
		"\n    options rd.luks.name=LUKS-UUID=aelum root=UUID=ROOT-UUID" +
		" rw quiet splash\n}\n"
	fstab := "UUID=ROOT-UUID / btrfs compress=zstd 0 1\n" +
		"UUID=ROOT-UUID /.snapshots btrfs subvol=@/.snapshots,compress=zstd 0 0\n" +
		"UUID=EFI-UUID /efi vfat rw 0 2\n" +
		"/efi/" + shared.InstallBootDir(id) + " /boot none bind 0 0\n"
	fs := primitivestest.NewMemFS().
		WithFile(shared.MountRoot+"/etc/fstab", fstab).
		WithFile(shared.MountRoot+"/etc/passwd", "alice:x:1000:1000::/home/alice:/bin/bash\n").
		WithFile(shared.MountRoot+"/etc/hostname", "aelum\n").
		WithFile(shared.MountRoot+"/etc/mkinitcpio.conf", "HOOKS=(base systemd plymouth sd-encrypt filesystems)\n").
		WithFile(shared.ESPMountPoint+"/"+shared.RefindThemeConfigPath, "banner themes/aelum/background.png\n").
		WithFile(shared.ESPMountPoint+"/"+shared.RefindConfigPath, "include "+shared.RefindThemeInclude+"\ninclude "+shared.RefindEntryInclude(id)+"\n").
		WithFile(shared.ESPMountPoint+"/"+shared.RefindEntryPath(id), entry)
	defer primitives.SetFileSystem(fs)()
	rec := recorderMounted()
	defer primitives.SetCommandRunner(rec)()

	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
}

// --- checkFstab --------------------------------------------------------------

func TestCheckFstab(t *testing.T) {
	ctx := &shared.InstallContext{Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID"}}
	const path = shared.MountRoot + "/etc/fstab"
	const bootLines = "UUID=EFI-UUID /efi vfat rw 0 2\n/efi/aelum/rootuuid /boot none bind 0 0\n"

	tests := []struct {
		name      string
		fs        *primitivestest.MemFS
		wantIssue string
	}{
		{
			name:      "missing",
			fs:        primitivestest.NewMemFS(),
			wantIssue: "fstab not found",
		},
		{
			name:      "empty",
			fs:        primitivestest.NewMemFS().WithFile(path, "   \n"),
			wantIssue: "fstab is empty",
		},
		{
			name:      "missing root UUID",
			fs:        primitivestest.NewMemFS().WithFile(path, bootLines),
			wantIssue: "fstab missing root UUID ROOT-UUID",
		},
		{
			name:      "missing EFI UUID",
			fs:        primitivestest.NewMemFS().WithFile(path, "UUID=ROOT-UUID / ext4 rw 0 1\n/efi/aelum/rootuuid /boot none bind 0 0\n"),
			wantIssue: "fstab missing EFI UUID EFI-UUID",
		},
		{
			name:      "missing install boot directory",
			fs:        primitivestest.NewMemFS().WithFile(path, "UUID=ROOT-UUID / ext4 rw 0 1\nUUID=EFI-UUID /efi vfat rw 0 2\n/efi/aelum/other /boot none bind 0 0\n"),
			wantIssue: "fstab missing this installation's boot directory /efi/aelum/rootuuid",
		},
		{
			name:      "missing boot bind",
			fs:        primitivestest.NewMemFS().WithFile(path, "UUID=ROOT-UUID / ext4 rw 0 1\nUUID=EFI-UUID /efi vfat rw 0 2\n/efi/aelum/rootuuid /boot none defaults 0 0\n"),
			wantIssue: "fstab missing the /boot bind mount",
		},
		{
			name: "complete",
			fs:   primitivestest.NewMemFS().WithFile(path, "UUID=ROOT-UUID / ext4 rw 0 1\n"+bootLines),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			var issues, info []string
			checkFstab(ctx, &issues, &info)
			if tt.wantIssue == "" {
				if len(issues) != 0 {
					t.Fatalf("issues = %v, want none", issues)
				}
				return
			}
			if len(issues) != 1 || !strings.Contains(issues[0], tt.wantIssue) {
				t.Fatalf("issues = %v, want one containing %q", issues, tt.wantIssue)
			}
		})
	}
}

// --- checkLuks ---------------------------------------------------------------

func TestCheckLuks(t *testing.T) {
	const confPath = shared.MountRoot + "/etc/mkinitcpio.conf"
	ctx := &shared.InstallContext{Luks: true, Partitions: &shared.PartitionInfo{LuksUUID: "LUKS-UUID", RootUUID: "ROOT-UUID"}}
	entryPath := bootEntryPath(ctx)

	tests := []struct {
		name      string
		fs        *primitivestest.MemFS
		wantIssue string
	}{
		{
			name:      "missing rEFInd config",
			fs:        primitivestest.NewMemFS().WithFile(confPath, "HOOKS=(sd-encrypt)\n"),
			wantIssue: "failed to read rEFInd config",
		},
		{
			name: "entry without mapping",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "root=UUID=ROOT-UUID rw\n").
				WithFile(confPath, "HOOKS=(sd-encrypt)\n"),
			wantIssue: "missing rd.luks.name=",
		},
		{
			name: "entry maps wrong mapper",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "rd.luks.name=LUKS-UUID=other root=UUID=ROOT-UUID\n").
				WithFile(confPath, "HOOKS=(sd-encrypt)\n"),
			wantIssue: "does not target aelum",
		},
		{
			name: "missing dm-crypt hook",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "rd.luks.name=LUKS-UUID=aelum root=UUID=ROOT-UUID\n").
				WithFile(confPath, "HOOKS=(base systemd)\n"),
			wantIssue: "sd-encrypt",
		},
		{
			name: "complete",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "rd.luks.name=LUKS-UUID=aelum root=UUID=ROOT-UUID\n").
				WithFile(confPath, "HOOKS=(base systemd sd-encrypt filesystems)\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			var issues, info []string
			checkLuks(ctx, &issues, &info)
			if tt.wantIssue == "" {
				if len(issues) != 0 {
					t.Fatalf("issues = %v, want none", issues)
				}
				return
			}
			if len(issues) != 1 || !strings.Contains(issues[0], tt.wantIssue) {
				t.Fatalf("issues = %v, want one containing %q", issues, tt.wantIssue)
			}
		})
	}
}

// --- checkBootloader ---------------------------------------------------------

func TestCheckBootloader(t *testing.T) {
	ctx := &shared.InstallContext{Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", RootPart: "/dev/vda2"}}
	id := shared.InstallID(ctx.Partitions.RootUUID)
	confPath := shared.ESPMountPoint + "/" + shared.RefindConfigPath
	entryPath := shared.ESPMountPoint + "/" + shared.RefindEntryPath(id)
	themeConfPath := shared.ESPMountPoint + "/" + shared.RefindThemeConfigPath
	includes := "include " + shared.RefindThemeInclude + "\ninclude " + shared.RefindEntryInclude(id) + "\n"
	entry := `menuentry "` + shared.RefindEntryTitle("/dev/vda2") + `" {` + "\n    loader /" + shared.InstallBootFilePath(id, "vmlinuz-linux-lts") + "\n}"

	tests := []struct {
		name      string
		fs        *primitivestest.MemFS
		rec       *primitivestest.Recorder
		wantIssue string
	}{
		{
			name: "missing binary",
			fs: primitivestest.NewMemFS().
				WithFile(confPath, includes).
				WithFile(entryPath, entry).
				WithFile(themeConfPath, "banner themes/aelum/background.png\n"),
			rec:       primitivestest.NewRecorder().OnArgsError([]string{"test", "-f"}, errors.New("missing")),
			wantIssue: "rEFInd binary not found",
		},
		{
			name:      "missing config",
			fs:        primitivestest.NewMemFS(),
			rec:       primitivestest.NewRecorder(),
			wantIssue: "rEFInd config not found",
		},
		{
			name:      "missing menu entry include",
			fs:        primitivestest.NewMemFS().WithFile(confPath, "timeout 5\n"),
			rec:       primitivestest.NewRecorder(),
			wantIssue: "missing the include for this installation's menu entry",
		},
		{
			name:      "missing theme include",
			fs:        primitivestest.NewMemFS().WithFile(confPath, "include "+shared.RefindEntryInclude(id)+"\n"),
			rec:       primitivestest.NewRecorder(),
			wantIssue: "missing the Aelum theme include",
		},
		{
			name: "missing menu entry file",
			fs: primitivestest.NewMemFS().
				WithFile(confPath, includes),
			rec:       primitivestest.NewRecorder(),
			wantIssue: "rEFInd menu entry not found",
		},
		{
			name: "wrong menu entry title",
			fs: primitivestest.NewMemFS().
				WithFile(confPath, includes).
				WithFile(entryPath, `menuentry "`+shared.BootEntryTitle+`" {}`),
			rec:       primitivestest.NewRecorder(),
			wantIssue: "is missing the " + shared.RefindEntryTitle("/dev/vda2") + " stanza",
		},
		{
			name: "entry loads another install's kernel",
			fs: primitivestest.NewMemFS().
				WithFile(confPath, includes).
				WithFile(entryPath, `menuentry "`+shared.RefindEntryTitle("/dev/vda2")+`" {`+"\n    loader /aelum/other0000/vmlinuz-linux-lts\n}"),
			rec:       primitivestest.NewRecorder(),
			wantIssue: "does not load a kernel from this installation's boot directory",
		},
		{
			name: "missing theme files",
			fs: primitivestest.NewMemFS().
				WithFile(confPath, includes).
				WithFile(entryPath, entry),
			rec:       primitivestest.NewRecorder(),
			wantIssue: "Aelum rEFInd theme not found",
		},
		{
			name: "complete",
			fs: primitivestest.NewMemFS().
				WithFile(confPath, includes).
				WithFile(entryPath, entry).
				WithFile(themeConfPath, "banner themes/aelum/background.png\n"),
			rec: primitivestest.NewRecorder(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			defer primitives.SetCommandRunner(tt.rec)()
			var issues, info []string
			checkBootloader(ctx, &issues, &info)
			if tt.wantIssue == "" {
				if len(issues) != 0 {
					t.Fatalf("issues = %v, want none", issues)
				}
				return
			}
			if len(issues) != 1 || !strings.Contains(issues[0], tt.wantIssue) {
				t.Fatalf("issues = %v, want one containing %q", issues, tt.wantIssue)
			}
		})
	}
}

// --- checkUserAccount / checkHostname ----------------------------------------

func TestCheckUserAccount(t *testing.T) {
	const path = shared.MountRoot + "/etc/passwd"
	ctx := &shared.InstallContext{Username: "alice"}

	tests := []struct {
		name      string
		fs        *primitivestest.MemFS
		wantIssue string
	}{
		{"missing passwd", primitivestest.NewMemFS(), "failed to read"},
		{"absent user", primitivestest.NewMemFS().WithFile(path, "root:x:0:0::/root:/bin/bash\n"), "not found"},
		{"present user", primitivestest.NewMemFS().WithFile(path, "alice:x:1000:1000::/home/alice:/bin/bash\n"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			var issues, info []string
			checkUserAccount(ctx, &issues, &info)
			if tt.wantIssue == "" {
				if len(issues) != 0 || len(info) != 1 {
					t.Fatalf("issues = %v info = %v, want only info", issues, info)
				}
				return
			}
			if len(issues) != 1 || !strings.Contains(issues[0], tt.wantIssue) {
				t.Fatalf("issues = %v, want one containing %q", issues, tt.wantIssue)
			}
		})
	}
}

func TestCheckHostname(t *testing.T) {
	const path = shared.MountRoot + "/etc/hostname"
	ctx := &shared.InstallContext{Hostname: "aelum"}

	tests := []struct {
		name      string
		fs        *primitivestest.MemFS
		wantIssue string
	}{
		{"missing", primitivestest.NewMemFS(), "failed to read"},
		{"mismatch", primitivestest.NewMemFS().WithFile(path, "wronghost\n"), "mismatch"},
		{"match", primitivestest.NewMemFS().WithFile(path, "aelum\n"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			var issues, info []string
			checkHostname(ctx, &issues, &info)
			if tt.wantIssue == "" {
				if len(issues) != 0 || len(info) != 1 {
					t.Fatalf("issues = %v info = %v, want only info", issues, info)
				}
				return
			}
			if len(issues) != 1 || !strings.Contains(issues[0], tt.wantIssue) {
				t.Fatalf("issues = %v, want one containing %q", issues, tt.wantIssue)
			}
		})
	}
}

// TestCheckFstab_ReadError verifies an unreadable fstab is reported rather than
// crashing the check.
func TestCheckFstab_ReadError(t *testing.T) {
	ctx := &shared.InstallContext{}
	fs := primitivestest.NewMemFS().WithReadError(shared.MountRoot+"/etc/fstab", errors.New("io error"))
	defer primitives.SetFileSystem(fs)()

	var issues, info []string
	checkFstab(ctx, &issues, &info)
	if len(issues) != 1 || !strings.Contains(issues[0], "fstab not found or unreadable") {
		t.Fatalf("issues = %v, want unreadable-fstab issue", issues)
	}
}

// --- checkLvm ----------------------------------------------------------------

func TestCheckLvm(t *testing.T) {
	const confPath = shared.MountRoot + "/etc/mkinitcpio.conf"
	lvmCtx := func() *shared.InstallContext {
		return &shared.InstallContext{Lvm: true, Partitions: &shared.PartitionInfo{
			VGName:     "1a2b3c4d",
			RootUUID:   "ROOT-UUID",
			HomeDevice: "/dev/1a2b3c4d/home",
			HomeUUID:   "HOME-UUID",
		}}
	}
	ctx := lvmCtx()
	entryPath := bootEntryPath(ctx)
	goodEntry := "options root=UUID=ROOT-UUID rw quiet\n"
	goodHooks := "HOOKS=(base systemd lvm2 filesystems fsck)\n"

	goodRecorder := func() *primitivestest.Recorder {
		return primitivestest.NewRecorder().
			OnArgsOutput([]string{"vgs"}, "  1a2b3c4d\n").
			OnArgsOutput([]string{"lvs"}, "  root\n  home\n").
			OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/1a2b3c4d/root"}, "ROOT-UUID\n")
	}

	tests := []struct {
		name      string
		ctx       *shared.InstallContext
		rec       *primitivestest.Recorder
		fs        *primitivestest.MemFS
		wantIssue string
	}{
		{
			name:      "no volume group recorded",
			ctx:       &shared.InstallContext{Lvm: true, Partitions: &shared.PartitionInfo{}},
			rec:       goodRecorder(),
			fs:        primitivestest.NewMemFS(),
			wantIssue: "no volume group was recorded",
		},
		{
			name:      "volume group missing",
			ctx:       ctx,
			rec:       goodRecorder().OnArgsError([]string{"vgs"}, errors.New("not found")),
			fs:        primitivestest.NewMemFS().WithFile(entryPath, goodEntry).WithFile(confPath, goodHooks),
			wantIssue: "volume group 1a2b3c4d not found",
		},
		{
			name:      "missing home logical volume",
			ctx:       ctx,
			rec:       goodRecorder().OnArgsOutput([]string{"lvs"}, "  root\n"),
			fs:        primitivestest.NewMemFS().WithFile(entryPath, goodEntry).WithFile(confPath, goodHooks),
			wantIssue: "logical volume home missing",
		},
		{
			name:      "root LV UUID does not match",
			ctx:       ctx,
			rec:       goodRecorder().OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/1a2b3c4d/root"}, "OTHER-UUID\n"),
			fs:        primitivestest.NewMemFS().WithFile(entryPath, goodEntry).WithFile(confPath, goodHooks),
			wantIssue: "root LV filesystem UUID is OTHER-UUID",
		},
		{
			name:      "missing lvm2 hook",
			ctx:       ctx,
			rec:       goodRecorder(),
			fs:        primitivestest.NewMemFS().WithFile(entryPath, goodEntry).WithFile(confPath, "HOOKS=(base systemd filesystems fsck)\n"),
			wantIssue: "lvm2 hook not found",
		},
		{
			name:      "boot entry does not use the root UUID",
			ctx:       ctx,
			rec:       goodRecorder(),
			fs:        primitivestest.NewMemFS().WithFile(entryPath, "options rw quiet\n").WithFile(confPath, goodHooks),
			wantIssue: "does not select the root by UUID",
		},
		{
			name:      "boot entry references a mapper path",
			ctx:       ctx,
			rec:       goodRecorder(),
			fs:        primitivestest.NewMemFS().WithFile(entryPath, "options root=UUID=ROOT-UUID rw root=/dev/mapper/1a2b3c4d-root\n").WithFile(confPath, goodHooks),
			wantIssue: "device-mapper path",
		},
		{
			name: "complete",
			ctx:  ctx,
			rec:  goodRecorder(),
			fs:   primitivestest.NewMemFS().WithFile(entryPath, goodEntry).WithFile(confPath, goodHooks),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			defer primitives.SetCommandRunner(tt.rec)()
			var issues, info []string
			checkLvm(tt.ctx, &issues, &info)
			if tt.wantIssue == "" {
				if len(issues) != 0 {
					t.Fatalf("issues = %v, want none", issues)
				}
				return
			}
			if len(issues) != 1 || !strings.Contains(issues[0], tt.wantIssue) {
				t.Fatalf("issues = %v, want one containing %q", issues, tt.wantIssue)
			}
		})
	}
}
