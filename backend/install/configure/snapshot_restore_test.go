package configure

import (
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestSnapshotRestoreScript pins the parts of the helper the prompt and the
// acceptance test depend on.
func TestSnapshotRestoreScript(t *testing.T) {
	script := snapshotRestoreScript
	for _, want := range []string{
		"#!/bin/sh",
		"set -u",
		// Root-only: the sudoers rule runs it as root, and anything else is a
		// mistake worth naming rather than a confusing snapper failure.
		`[ "$(id -u)" = 0 ]`,
		// The guard that makes "no number" mean the right thing: snapper's no-number
		// rollback means "the state I am booted in", which is only correct from
		// inside a snapshot. The marker is what says this boot is one.
		`AELUM_SNAPSHOT_MARKER`,
		snapshotBootMarkerPath,
		`[ -f "$marker" ]`,
		// The rollback itself, --no-dbus like the pacman hooks: the D-Bus path adds
		// a service that can fail, and from a desktop session that failure would
		// look like the restore failing.
		"snapper --no-dbus -c " + snapperRootConfig + " rollback",
		// The prompt's last button reboots without rolling back again - the OK
		// button's call already did that, and a second rollback would take a second
		// undo snapshot of a system that is already restored.
		"--reboot-only)",
		`exec systemctl reboot`,
		// An unknown argument is a mistake worth naming, not a silent rollback.
		`usage: aelum-snapshot-restore`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("restore helper is missing %q", want)
		}
	}

	// The same traps the other shell files had: a Go comment inside the raw string
	// is a command to /bin/sh.
	for i, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, "//") {
			t.Errorf("restore helper line %d is a Go comment, not a shell comment: %q", i+1, line)
		}
	}
}

// TestSnapshotRestoreSudoersIsNarrow: the passwordless rule names one absolute
// program and nothing else. A broader rule (NOPASSWD: ALL, or a wildcard path)
// would hand every wheel user a passwordless root shell, which is not what an OK
// button is for.
func TestSnapshotRestoreSudoersIsNarrow(t *testing.T) {
	line := "%wheel ALL=(ALL) NOPASSWD: " + snapshotRestoreScriptPath
	if !strings.Contains(snapshotRestoreSudoers, line) {
		t.Errorf("sudoers rule does not contain %q:\n%s", line, snapshotRestoreSudoers)
	}
	for _, unwanted := range []string{"NOPASSWD: ALL", "NOPASSWD: *"} {
		if strings.Contains(snapshotRestoreSudoers, unwanted) {
			t.Errorf("sudoers rule contains %q; the rule must name one program", unwanted)
		}
	}
}

// TestConfigureSnapshotRestore installs the helper executable and the sudoers rule
// with the mode sudo accepts (sudo refuses a world-writable sudoers file).
func TestConfigureSnapshotRestore(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := configureSnapshotRestore(&shared.InstallContext{FileSystem: shared.FileSystemBtrfs}); err != nil {
		t.Fatalf("configureSnapshotRestore() = %v, want nil", err)
	}

	scriptPath := shared.MountRoot + snapshotRestoreScriptPath
	content, ok := fs.Read(scriptPath)
	if !ok {
		t.Fatalf("%s was not written (paths: %v)", scriptPath, fs.Paths())
	}
	if !strings.Contains(content, "#!/bin/sh") {
		t.Errorf("%s is not a script:\n%s", scriptPath, content)
	}
	if mode := fs.Mode(scriptPath); mode&0o111 == 0 {
		t.Errorf("%s is not executable (%v): the prompt runs it, it does not read it", scriptPath, mode)
	}

	sudoersPath := shared.MountRoot + snapshotRestoreSudoersPath
	sudoers, ok := fs.Read(sudoersPath)
	if !ok {
		t.Fatalf("%s was not written (paths: %v)", sudoersPath, fs.Paths())
	}
	if !strings.Contains(sudoers, "%wheel ALL=(ALL) NOPASSWD: "+snapshotRestoreScriptPath) {
		t.Errorf("sudoers rule missing from %s:\n%s", sudoersPath, sudoers)
	}
	if mode := fs.Mode(sudoersPath); mode&0o022 != 0 {
		t.Errorf("%s is group/world-writable (%v): sudo refuses a writable sudoers file", sudoersPath, mode)
	}

	if !hasCall(rec.CallsFor("mkdir"), "mkdir", "-p", shared.MountRoot+"/usr/local/bin") {
		t.Errorf("the helper directory was not created; calls = %v", rec.Calls())
	}
}

// TestConfigureSnapper_InstallsRestoreBeforeTheSnapshot is the ordering that is
// easy to get wrong and impossible to fix later: a snapshot runs the binaries and
// sudoers rules it contains. A helper written after the "after installation"
// snapshot is absent from every state this system can boot into.
func TestConfigureSnapper_InstallsRestoreBeforeTheSnapshot(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(snapperContext()); err != nil {
		t.Fatalf("ConfigureSnapper() = %v, want nil", err)
	}

	snapshot := snapperCallIndex(rec, "arch-chroot", shared.MountRoot, "snapper", "--no-dbus", "-c", snapperRootConfig,
		"create", "-t", "single", "-d", "after installation", "-c", "number", "-u", "important=yes")
	if snapshot < 0 {
		t.Fatalf("no \"after installation\" snapshot; calls = %v", rec.Calls())
	}

	if !fs.Exists(shared.MountRoot + snapshotRestoreScriptPath) {
		t.Fatalf("%s was not installed by the snapper step", snapshotRestoreScriptPath)
	}
	if !fs.Exists(shared.MountRoot + snapshotRestoreSudoersPath) {
		t.Fatalf("%s was not installed by the snapper step", snapshotRestoreSudoersPath)
	}
	written := snapperCallIndex(rec, "mkdir", "-p", shared.MountRoot+"/usr/local/bin")
	if written < 0 {
		t.Fatalf("the helper directory was not created; calls = %v", rec.Calls())
	}
	if written > snapshot {
		t.Error("the restore helper was installed after the snapshot was taken, so it is not inside it")
	}
}

// TestConfigureSnapper_NoRestoreForExt4: an ext4 install has no snapshot to
// restore, so it gets neither the helper nor the passwordless rule.
func TestConfigureSnapper_NoRestoreForExt4(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(&shared.InstallContext{FileSystem: shared.FileSystemExt4}); err != nil {
		t.Fatalf("ConfigureSnapper(ext4) = %v, want nil", err)
	}
	if fs.Exists(shared.MountRoot + snapshotRestoreScriptPath) {
		t.Errorf("%s was installed for an ext4 install", snapshotRestoreScriptPath)
	}
	if fs.Exists(shared.MountRoot + snapshotRestoreSudoersPath) {
		t.Errorf("%s was installed for an ext4 install", snapshotRestoreSudoersPath)
	}
}
