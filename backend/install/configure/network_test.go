package configure

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// The resolver link is about the boots that come after the install, so these
// tests are mostly about the two things that can quietly undo it: the regular
// file the base image already put there, and the order this runs in.

func TestConfigureNetworkDNSLinksResolvConfIntoRun(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	// The base package ships a plain file at this path; the interesting case is
	// the one where something is already there.
	fs := primitivestest.NewMemFS().WithFile(shared.MountRoot+resolvConfPath, "# Resolver configuration file.\n")
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureNetworkDNS(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigureNetworkDNS() = %v, want nil", err)
	}

	target, ok := fs.SymlinkTarget(shared.MountRoot + resolvConfPath)
	if !ok {
		t.Fatalf("%s is not a symlink; known paths = %v", resolvConfPath, fs.Paths())
	}
	if target != networkManagerResolvConf {
		t.Errorf("link target = %q, want %q", target, networkManagerResolvConf)
	}
	if _, stillAFile := fs.Read(shared.MountRoot + resolvConfPath); stillAFile {
		t.Error("the regular file is still there: a link was written next to it, not over it")
	}
}

// TestConfigureNetworkDNSIsAbsolute: the target is absolute on purpose.
// NetworkManager's rc-manager treats a link to exactly its own internal path as
// its own and refreshes it when DNS changes, which is what notifies everything
// else. A relative link resolves the same way and earns no such refresh.
func TestConfigureNetworkDNSIsAbsolute(t *testing.T) {
	if !strings.HasPrefix(networkManagerResolvConf, "/run/") {
		t.Errorf("resolver target = %q, want an absolute path under /run", networkManagerResolvConf)
	}
	if networkManagerResolvConf != "/run/NetworkManager/resolv.conf" {
		t.Error("the target must be NetworkManager's own runtime resolv.conf, spelled exactly as it writes it")
	}
}

// TestConfigureNetworkDNSFailureIsNotAFailedInstall: an install that could not
// create the link is an install with the old behaviour, not a broken system.
func TestConfigureNetworkDNSFailureIsNotAFailedInstall(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS().WithSymlinkError(shared.MountRoot+resolvConfPath, errors.New("read-only file system"))
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureNetworkDNS(&shared.InstallContext{}); err != nil {
		t.Fatalf("ConfigureNetworkDNS() = %v, want nil: a resolver link is not worth failing an install over", err)
	}
}

// TestConfigureNetworkDNSRunsBeforeTheInstallationSnapshot is the ordering that
// decides whether this feature exists at all: a snapshot only contains what was
// written before it was taken, so a link created after the "after installation"
// snapshot is missing from every snapshot the system has - including the factory
// state a broken system is restored to, which is the one that had no resolver.
func TestConfigureNetworkDNSRunsBeforeTheInstallationSnapshot(t *testing.T) {
	rec := mountedRunner().OnArgsOutput([]string{"find", shared.BootMountPoint}, shared.BootMountPoint+"/vmlinuz-linux\n")
	fs := primitivestest.NewMemFS().
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
		t.Fatalf("the configuration never took the \"after installation\" snapshot; calls = %v", len(rec.Calls()))
	}
	if !watch.linkAt {
		t.Error("/etc/resolv.conf was not linked when the \"after installation\" snapshot was taken, so no snapshot of this system has the link")
	}
	if _, ok := fs.SymlinkTarget(shared.MountRoot + resolvConfPath); !ok {
		t.Errorf("%s is not a link in the installed system", resolvConfPath)
	}
}

// snapshotWatcher records whether the resolver link existed at the moment the
// "after installation" snapshot command ran - the only moment that decides what
// is inside that snapshot.
type snapshotWatcher struct {
	rec     *primitivestest.Recorder
	fs      *primitivestest.MemFS
	linkAt  bool
	checked bool
}

func (w *snapshotWatcher) observe(cmd *primitives.Command) {
	if w.checked {
		return
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "snapper") || !strings.Contains(joined, "after installation") {
		return
	}
	_, w.linkAt = w.fs.SymlinkTarget(shared.MountRoot + resolvConfPath)
	w.checked = true
}

func (w *snapshotWatcher) Run(cmd *primitives.Command) error {
	w.observe(cmd)
	return w.rec.Run(cmd)
}

func (w *snapshotWatcher) Output(cmd *primitives.Command) ([]byte, error) {
	w.observe(cmd)
	return w.rec.Output(cmd)
}

func (w *snapshotWatcher) CombinedOutput(cmd *primitives.Command) ([]byte, error) {
	w.observe(cmd)
	return w.rec.CombinedOutput(cmd)
}
