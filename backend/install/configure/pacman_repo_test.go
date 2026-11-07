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
// step must not disturb. The commented multilib block is here because it is what
// the pacman package really ships, and because the section right above it must
// not be the one that comes out uncommented.
const stockPacmanConf = `[options]
SigLevel = Required DatabaseOptional
Include = /etc/pacman.d/mirrorlist

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist

# If you want to run 32 bit applications on your x86_64 system,
# enable the multilib repositories as required here.

#[multilib-testing]
#Include = /etc/pacman.d/mirrorlist

#[multilib]
#Include = /etc/pacman.d/mirrorlist

# An example of a custom package repository.  See the pacman manpage for
# tips on creating your own repositories.
#[custom]
#SigLevel = Optional TrustAll
#Server = file:///home/custompkgs
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

// withSyncDBs is what a fresh install's target really holds: pacstrap resolved
// every package through the live session's [aelum-offline] repository, and
// pacman wrote that database under the --root it was given.
func withSyncDBs(fs *primitivestest.MemFS) {
	fs.WithFile(shared.MountRoot+shared.PacmanSyncDir+"/aelum-offline.db", "offline mirror fixture")
	fs.WithFile(shared.MountRoot+shared.PacmanSyncDir+"/core.db", "configured repository fixture")
	// A database for a repository this installer enables is not foreign either,
	// which is only true because the config is written first.
	fs.WithFile(shared.MountRoot+shared.PacmanSyncDir+"/multilib.db", "multilib fixture")
}

func TestConfigurePacmanRepo_DropsTheLiveSessionsDatabase(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := pacmanRepoFS(withKeyring, withSyncDBs)
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil", err)
	}

	// The target's pacman.conf names no [aelum-offline], so pacman would never
	// read this database again — and the package manager, which reads the sync
	// directory, listed every package in it a second time.
	if fs.Exists(shared.MountRoot + shared.PacmanSyncDir + "/aelum-offline.db") {
		t.Error("the live session's sync database was left in the target")
	}
	// A database for a repository the config does name is not foreign: it is
	// what the Updates page is about.
	if !fs.Exists(shared.MountRoot + shared.PacmanSyncDir + "/core.db") {
		t.Error("a configured repository's sync database was removed")
	}
	// multilib is configured by this same step, so its database is kept too —
	// that is the order of ConfigurePacmanRepo, in an assertion.
	if !fs.Exists(shared.MountRoot + shared.PacmanSyncDir + "/multilib.db") {
		t.Error("the sync database of a repository this step enables was removed")
	}
}

func TestConfigurePacmanRepo_ForeignDatabaseIsNotFatal(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := pacmanRepoFS(withKeyring, withSyncDBs)
	fs.WithRemoveError(shared.MountRoot+shared.PacmanSyncDir+"/aelum-offline.db", errors.New("read-only tree"))
	defer primitives.SetFileSystem(fs)()

	// A leftover database costs a duplicate row in a GUI, not a broken system.
	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil when only the cleanup failed", err)
	}
	if !fs.Exists(shared.MountRoot + shared.PacmanSyncDir + "/aelum-offline.db") {
		t.Error("the database was reported removed although the removal failed")
	}
}

func TestConfigurePacmanRepo_NoSyncDirectoryIsNotFatal(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := pacmanRepoFS(withKeyring) // an install that never refreshed a database
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil", err)
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
	// A pacman.conf that already carries both blocks: the installer re-run over
	// an existing Aelum system, where multilib is uncommented too.
	installed := strings.Replace(stockPacmanConf, "#[multilib]\n#Include", "[multilib]\nInclude", 1) +
		"\n[aelum]\nSigLevel = Required DatabaseRequired\nServer = https://cdn.aelum.dev/repo/$arch\n"
	fs := pacmanRepoFS(withKeyring, withPacmanConf(installed))
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil", err)
	}

	content, _ := fs.Read(shared.MountRoot + "/etc/pacman.conf")
	if content != installed {
		t.Errorf("an enabled pacman.conf was modified:\n%s", content)
	}
	if strings.Count(content, "["+shared.PacmanRepoName+"]") != 1 {
		t.Errorf("a second [%s] section was appended; a later section would shadow the first", shared.PacmanRepoName)
	}
}

func TestConfigurePacmanRepo_EnablesMultilib(t *testing.T) {
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
	// The stock example, uncommented: the same mirrorlist core and extra use,
	// because it carries $repo. No SigLevel line — multilib is signed with the
	// Arch keys the system already trusts.
	want := "\n[" + shared.PacmanMultilibRepoName + "]\nInclude = " + shared.PacmanMirrorListPath + "\n"
	if !strings.Contains(content, want) {
		t.Errorf("multilib was not enabled on the mirrorlist the file already uses:\n%s", content)
	}
	if strings.Count(content, "\n["+shared.PacmanMultilibRepoName+"]\n") != 1 {
		t.Errorf("the [%s] section was not written exactly once:\n%s", shared.PacmanMultilibRepoName, content)
	}
	// The other commented examples stay commented: [multilib-testing] would add a
	// channel nobody asked for, and an empty [custom] would stop pacman — and with
	// it the package manager — on every command.
	for _, other := range []string{"multilib-testing", "custom"} {
		if hasPacmanSection(content, other) {
			t.Errorf("the [%s] example was enabled too:\n%s", other, content)
		}
	}
	if !hasPacmanSection(content, "core") || !hasPacmanSection(content, "extra") {
		t.Errorf("the stock repositories were disturbed:\n%s", content)
	}
}

func TestConfigurePacmanRepo_LeavesAHandEnabledMultilibAlone(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	// A system where someone enabled multilib by hand, in the shape the stock
	// example suggests. A second [multilib] section would shadow their Server
	// line, which is the same failure the [aelum] idempotence guard exists for.
	handEnabled := "[options]\nSigLevel = Required DatabaseOptional\nInclude = /etc/pacman.d/mirrorlist\n\n" +
		"[multilib]\nSigLevel = Optional TrustAll\nServer = https://mirrors.kernel.org/archlinux/$repo/os/$arch\n"
	fs := pacmanRepoFS(withKeyring, withPacmanConf(handEnabled))
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil", err)
	}

	content, _ := fs.Read(shared.MountRoot + "/etc/pacman.conf")
	if strings.Count(content, "\n["+shared.PacmanMultilibRepoName+"]\n") != 1 {
		t.Errorf("a second [%s] section was appended:\n%s", shared.PacmanMultilibRepoName, content)
	}
	if !strings.Contains(content, "Server = https://mirrors.kernel.org/archlinux/$repo/os/$arch") {
		t.Errorf("the Server line chosen by hand was not preserved:\n%s", content)
	}
}

func TestConfigurePacmanRepo_WritesMultilibWhenTheExampleIsGone(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	// A pacman.conf someone edited by hand: no commented example to uncomment, so
	// the section has to be written.
	handEdited := "[options]\nSigLevel = Required DatabaseOptional\nInclude = /etc/pacman.d/mirrorlist\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n"
	fs := pacmanRepoFS(withKeyring, withPacmanConf(handEdited))
	defer primitives.SetFileSystem(fs)()

	if err := ConfigurePacmanRepo(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigurePacmanRepo() = %v, want nil", err)
	}

	content, _ := fs.Read(shared.MountRoot + "/etc/pacman.conf")
	if !strings.Contains(content, "\n["+shared.PacmanMultilibRepoName+"]\nInclude = "+shared.PacmanMirrorListPath) {
		t.Errorf("multilib was not written into a config that carries no example:\n%s", content)
	}
	if !hasPacmanSection(content, "core") {
		t.Errorf("the existing repositories were not preserved:\n%s", content)
	}
}

func TestUncommentPacmanSection(t *testing.T) {
	t.Run("uncomments the example and stops at the next one", func(t *testing.T) {
		got, found := uncommentPacmanSection(
			"#[multilib]\n#Include = /etc/pacman.d/mirrorlist\n#[custom]\n#Server = file:///home/custompkgs\n",
			shared.PacmanMultilibRepoName)
		if !found {
			t.Fatal("the commented section was not found")
		}
		want := "[multilib]\nInclude = /etc/pacman.d/mirrorlist\n#[custom]\n#Server = file:///home/custompkgs\n"
		if got != want {
			t.Errorf("uncommented the wrong lines:\n%s", got)
		}
	})

	t.Run("a header with nothing under it gets the mirrorlist", func(t *testing.T) {
		// A repository with no Server or Include line is a config error, and a
		// config error stops every pacman command on the system.
		got, found := uncommentPacmanSection(
			"#[multilib]\n\n[extra]\nServer = file:///extra\n", shared.PacmanMultilibRepoName)
		if !found {
			t.Fatal("the commented section was not found")
		}
		if !strings.Contains(got, "[multilib]\nInclude = "+shared.PacmanMirrorListPath+"\n") {
			t.Errorf("the section was left with no download source:\n%s", got)
		}
		if !strings.Contains(got, "[extra]\nServer = file:///extra") {
			t.Errorf("the section after it was changed:\n%s", got)
		}
	})

	t.Run("a config without the section is returned unchanged", func(t *testing.T) {
		content := "[core]\nInclude = /etc/pacman.d/mirrorlist\n"
		got, found := uncommentPacmanSection(content, shared.PacmanMultilibRepoName)
		if found || got != content {
			t.Errorf("found a section that is not there: found=%v\n%s", found, got)
		}
	})
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
		Locale:     "en_US.UTF-8",
		Region:     shared.GetRegion("us"),
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
	if !watch.multilibAt {
		t.Error("multilib was not enabled when the snapshot was taken")
	}
	if !watch.keyTrusted {
		t.Error("pacman-key did not trust the Aelum key before the snapshot was taken")
	}
}
