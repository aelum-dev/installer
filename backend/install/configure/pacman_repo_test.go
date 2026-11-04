package configure

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// The stock pacman.conf as pacstrap lays it down, trimmed to the parts this
// step must not disturb.
const stockPacmanConf = `[options]
SigLevel = Required DatabaseOptional
Include = /etc/pacman.d/mirrorlist

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist
`

func pacmanRepoFS(preloads ...func(*primitivestest.MemFS)) *primitivestest.MemFS {
	fs := primitivestest.NewMemFS().WithFile(shared.MountRoot+"/etc/pacman.conf", stockPacmanConf)
	for _, preload := range preloads {
		preload(fs)
	}
	return fs
}

func withKeyring(fs *primitivestest.MemFS) {
	fs.WithFile(shared.MountRoot+shared.AelumKeyringPath, "binary keyring fixture")
}

func withPacmanConf(content string) func(*primitivestest.MemFS) {
	return func(fs *primitivestest.MemFS) {
		fs.WithFile(shared.MountRoot+"/etc/pacman.conf", content)
	}
}

func TestConfigurePacmanRepo_AppendsBlockAndTrustsKey(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := pacmanRepoFS(withKeyring)
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil", err)
	}

	content, ok := fs.Read(shared.MountRoot + "/etc/pacman.conf")
	if !ok {
		t.Fatal("pacman.conf was not written")
	}
	// The stock content is kept: this is an append to a pacman-owned file, not
	// a rewrite of it.
	if !strings.Contains(content, "Include = /etc/pacman.d/mirrorlist") {
		t.Errorf("the stock pacman.conf was not preserved:\n%s", content)
	}
	for _, want := range []string{
		"[" + shared.PacmanRepoName + "]",
		"SigLevel = " + shared.PacmanRepoSigLevel,
		"Server = " + shared.PacmanRepoServer,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the repository block is missing %q:\n%s", want, content)
		}
	}
	if strings.Count(content, "["+shared.PacmanRepoName+"]") != 1 {
		t.Errorf("the repository section was not written exactly once:\n%s", content)
	}

	if !hasCall(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "pacman-key", "--add", shared.AelumKeyringPath) {
		t.Errorf("the signing key was not added to the target keyring; calls = %v", rec.Calls())
	}
	if !hasCall(rec.CallsFor("arch-chroot"), "arch-chroot", shared.MountRoot, "pacman-key", "--lsign-key", shared.AelumSigningKeyFingerprint) {
		t.Errorf("the signing key was not locally signed; calls = %v", rec.Calls())
	}
}

func TestConfigurePacmanRepo_Idempotent(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	// A pacman.conf that already carries the block (the installer re-run over
	// an existing system).
	enabled := stockPacmanConf + "\n[aelum]\nSigLevel = Required DatabaseRequired\nServer = https://cdn.aelum.dev/repo/$arch\n"
	fs := pacmanRepoFS(withKeyring, withPacmanConf(enabled))
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil", err)
	}

	content, _ := fs.Read(shared.MountRoot + "/etc/pacman.conf")
	if content != enabled {
		t.Errorf("an enabled pacman.conf was modified:\n%s", content)
	}
	if strings.Count(content, "["+shared.PacmanRepoName+"]") != 1 {
		t.Errorf("a second [%s] section was appended; a later section would shadow the first", shared.PacmanRepoName)
	}
}

func TestConfigurePacmanRepo_MissingKeyringFails(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := pacmanRepoFS() // no keyring file: a broken live ISO
	defer primitives.SetFileSystem(fs)()

	err := ConfigurePacmanRepo(&shared.InstallContext{})
	if err == nil {
		t.Fatal("ConfigurePacmanRepo() = nil, want an error when the live ISO shipped no key")
	}
	if !strings.Contains(err.Error(), shared.AelumKeyringPath) {
		t.Errorf("the error does not name the missing file: %v", err)
	}
	for _, argv := range rec.CallsFor("arch-chroot") {
		if strings.Contains(strings.Join(argv, " "), "pacman-key") {
			t.Errorf("pacman-key ran although the key file is missing: %v", argv)
		}
	}
}

func TestConfigurePacmanRepo_KeyImportFailureIsFatal(t *testing.T) {
	rec := mountedRunner().OnArgsError(
		[]string{"arch-chroot", shared.MountRoot, "pacman-key"}, errors.New("keyserver said no"))
	defer primitives.SetCommandRunner(rec)()
	fs := pacmanRepoFS(withKeyring)
	defer primitives.SetFileSystem(fs)()

	err := ConfigurePacmanRepo(&shared.InstallContext{})
	if err == nil {
		t.Fatal("ConfigurePacmanRepo() = nil, want an error when pacman-key fails")
	}
	if !strings.Contains(err.Error(), "signing key") {
		t.Errorf("the error does not explain which key step failed: %v", err)
	}
}

// TestConfigurePacmanRepoIsInsideTheFactorySnapshot is the ordering claim, in
// the same shape as the resolver link's: a snapshot only contains what was
// written before it was taken, so a repository enabled after the "after
// installation" snapshot is missing from the factory state a broken system is
// restored to — and that state could not install anything.
func TestConfigurePacmanRepoIsInsideTheFactorySnapshot(t *testing.T) {
	rec := mountedRunner().OnArgsOutput([]string{"find", shared.BootMountPoint}, shared.BootMountPoint+"/vmlinuz-linux\n")
	// The keyring file stands in for the isocopy job that carries it during
	// SetupUsers, the step Steps() puts before this one.
	fs := pacmanRepoFS(withKeyring).
		WithFile(shared.MountRoot+"/etc/locale.gen", "#en_US.UTF-8 UTF-8\n").
		WithFile(shared.MountRoot+"/etc/mkinitcpio.conf", "HOOKS=(base udev autodetect modconf block filesystems)\n").
		WithFile("/proc/meminfo", "MemTotal:        8155136 kB\n").
		WithFile(shared.MountRoot+"/boot/vmlinuz-linux", "kernel").
		WithFile(shared.MountRoot+"/boot/initramfs-linux.img", "initrd")
	defer primitives.SetFileSystem(fs)()
	watch := &snapshotWatcher{rec: rec, fs: fs}
	defer primitives.SetCommandRunner(watch)()

	ctx := &shared.InstallContext{
		DiskID:     "/dev/vda",
		Partitions: &shared.PartitionInfo{RootUUID: "ROOT-UUID", EFIUUID: "EFI-UUID"},
		Username:   "alice",
		Hostname:   "aelum",
		Region:     shared.Regions["en_US"],
		FileSystem: shared.FileSystemBtrfs,
	}
	if err := Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	if !watch.checked {
		t.Fatal("the configuration never took the \"after installation\" snapshot")
	}
	if !watch.repoAt {
		t.Error("the [aelum] repository was not enabled when the snapshot was taken")
	}
	if !watch.keyTrusted {
		t.Error("pacman-key did not trust the Aelum key before the snapshot was taken")
	}
}
