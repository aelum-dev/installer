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
	fs := primitivestest.NewMemFS().
		WithFile(shared.MountRoot+"/etc/fstab", "UUID=ROOT-UUID / ext4 rw 0 1\nUUID=EFI-UUID /boot vfat rw 0 2\n").
		WithFile(shared.MountRoot+"/etc/passwd", "root:x:0:0::/root:/bin/bash\nalice:x:1000:1000::/home/alice:/bin/bash\n").
		WithFile(shared.MountRoot+"/etc/hostname", "aelum\n")
	defer primitives.SetFileSystem(fs)()
	rec := recorderMounted()
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Username:   "alice",
		Hostname:   "aelum",
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID"},
	}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
}

func TestRun_PassesWithLuksAndLvm(t *testing.T) {
	entry := "rd.luks.name=LUKS-UUID=aelum root=" + shared.LvmRootLvPath + " rw quiet splash rootflags=subvol=@\n"
	fs := primitivestest.NewMemFS().
		WithFile(shared.MountRoot+"/etc/fstab", "UUID=ROOT-UUID / ext4 rw 0 1\nUUID=EFI-UUID /boot vfat rw 0 2\nUUID=HOME-UUID /home ext4 rw 0 2\n").
		WithFile(shared.MountRoot+"/etc/passwd", "alice:x:1000:1000::/home/alice:/bin/bash\n").
		WithFile(shared.MountRoot+"/etc/hostname", "aelum\n").
		WithFile(shared.MountRoot+"/etc/mkinitcpio.conf", "HOOKS=(base systemd plymouth sd-encrypt lvm2 filesystems)\n").
		WithFile(shared.BootMountPoint+"/loader/entries/"+shared.BootEntryFileName+".conf", entry)
	defer primitives.SetFileSystem(fs)()
	rec := recorderMounted()
	defer primitives.SetCommandRunner(rec)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Username:   "alice",
		Hostname:   "aelum",
		FileSystem: shared.FileSystemBtrfs,
		Luks:       true,
		Lvm:        true,
		Partitions: &shared.PartitionInfo{
			RootUUID: "ROOT-UUID",
			EFIUUID:  "EFI-UUID",
			LuksUUID: "LUKS-UUID",
			HomeUUID: "HOME-UUID",
		},
	}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
}

// --- checkFstab --------------------------------------------------------------

func TestCheckFstab(t *testing.T) {
	ctx := &shared.InstallContext{Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID"}}
	const path = shared.MountRoot + "/etc/fstab"

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
			fs:        primitivestest.NewMemFS().WithFile(path, "UUID=EFI-UUID /boot vfat rw 0 2\n"),
			wantIssue: "fstab missing root UUID ROOT-UUID",
		},
		{
			name:      "missing EFI UUID",
			fs:        primitivestest.NewMemFS().WithFile(path, "UUID=ROOT-UUID / ext4 rw 0 1\n"),
			wantIssue: "fstab missing EFI UUID EFI-UUID",
		},
		{
			name: "complete",
			fs:   primitivestest.NewMemFS().WithFile(path, "UUID=ROOT-UUID / ext4 rw 0 1\nUUID=EFI-UUID /boot vfat rw 0 2\n"),
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
	const (
		entryPath = shared.BootMountPoint + "/loader/entries/" + shared.BootEntryFileName + ".conf"
		confPath  = shared.MountRoot + "/etc/mkinitcpio.conf"
	)
	ctx := &shared.InstallContext{Luks: true, Partitions: &shared.PartitionInfo{LuksUUID: "LUKS-UUID", RootUUID: "ROOT-UUID"}}

	tests := []struct {
		name      string
		fs        *primitivestest.MemFS
		wantIssue string
	}{
		{
			name:      "missing boot entry",
			fs:        primitivestest.NewMemFS().WithFile(confPath, "HOOKS=(sd-encrypt)\n"),
			wantIssue: "failed to read boot entry",
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

// --- checkLvm ----------------------------------------------------------------

func TestCheckLvm(t *testing.T) {
	const (
		entryPath = shared.BootMountPoint + "/loader/entries/" + shared.BootEntryFileName + ".conf"
		confPath  = shared.MountRoot + "/etc/mkinitcpio.conf"
		fstabPath = shared.MountRoot + "/etc/fstab"
	)
	ctx := &shared.InstallContext{Lvm: true, Partitions: &shared.PartitionInfo{HomeUUID: "HOME-UUID"}}

	tests := []struct {
		name      string
		fs        *primitivestest.MemFS
		wantIssue string
	}{
		{
			name:      "missing boot entry",
			fs:        primitivestest.NewMemFS().WithFile(confPath, "HOOKS=(lvm2)\n"),
			wantIssue: "failed to read boot entry",
		},
		{
			name: "entry lacks root LV",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "root=UUID=ROOT-UUID rw\n").
				WithFile(confPath, "HOOKS=(lvm2)\n").
				WithFile(fstabPath, "UUID=HOME-UUID /home ext4 rw 0 2\n"),
			wantIssue: "does not reference the root LV",
		},
		{
			name: "fstab lacks home UUID",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "root="+shared.LvmRootLvPath+" rw\n").
				WithFile(confPath, "HOOKS=(lvm2)\n").
				WithFile(fstabPath, "UUID=ROOT-UUID / ext4 rw 0 1\n"),
			wantIssue: "fstab missing home LV UUID HOME-UUID",
		},
		{
			name: "missing lvm2 hook",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "root="+shared.LvmRootLvPath+" rw\n").
				WithFile(confPath, "HOOKS=(base systemd)\n").
				WithFile(fstabPath, "UUID=HOME-UUID /home ext4 rw 0 2\n"),
			wantIssue: "lvm2 hook not found",
		},
		{
			name: "complete",
			fs: primitivestest.NewMemFS().
				WithFile(entryPath, "root="+shared.LvmRootLvPath+" rw\n").
				WithFile(confPath, "HOOKS=(base systemd lvm2 filesystems)\n").
				WithFile(fstabPath, "UUID=HOME-UUID /home ext4 rw 0 2\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer primitives.SetFileSystem(tt.fs)()
			var issues, info []string
			checkLvm(ctx, &issues, &info)
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
