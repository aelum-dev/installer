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
		// Root-only: pkexec runs it as root, and anything else is a
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

// TestSnapshotRestorePolkitIsScoped: the policy's defaults refuse the action
// everywhere, and the only grant is the rules file's - local, active, admin.
// A grant that dropped subject.active would hand the action to every SSH session;
// one that dropped user_is_admin would hand it to the greeter; an allow_* of yes
// would answer before the rules were even consulted. All three are the same bug:
// a root action that answers for someone who never asked.
func TestSnapshotRestorePolkitIsScoped(t *testing.T) {
	policy := snapshotRestorePolicy
	for _, want := range []string{
		`<action id="` + snapshotRestorePolkitAction + `">`,
		"<allow_any>no</allow_any>",
		"<allow_inactive>no</allow_inactive>",
		"<allow_active>no</allow_active>",
		// The annotation is what makes `pkexec <helper>` resolve to this action;
		// without it pkexec refuses the program before polkit is consulted at all.
		`<annotate key="org.freedesktop.policykit.exec.path">` + snapshotRestoreScriptPath + `</annotate>`,
	} {
		if !strings.Contains(policy, want) {
			t.Errorf("polkit policy is missing %q:\n%s", want, policy)
		}
	}

	rules := snapshotRestoreRules
	for _, want := range []string{
		"polkit.addRule",
		`action.id === "` + snapshotRestorePolkitAction + `"`,
		"subject.local",
		"subject.active",
		"polkit.user_is_admin",
		"polkit.Result.YES",
	} {
		if !strings.Contains(rules, want) {
			t.Errorf("polkit rules are missing %q:\n%s", want, rules)
		}
	}
	for _, unwanted := range []string{"allow_any>yes", "allow_inactive>yes", "allow_active>yes", "NOPASSWD"} {
		if strings.Contains(policy, unwanted) {
			t.Errorf("polkit policy contains %q; the grant belongs to the rules file only", unwanted)
		}
	}
}

// TestConfigureSnapshotRestore installs the helper executable and the polkit
// policy + rules that authorize it.
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
	// pkexec's own authenticity check for a program an action execs: root-owned
	// and not group- or world-writable. The 0755 the helper is written with is
	// what satisfies it.
	if mode := fs.Mode(scriptPath); mode&0o022 != 0 {
		t.Errorf("%s is group/world-writable (%v): pkexec refuses to exec such a program", scriptPath, mode)
	}

	policyPath := shared.MountRoot + snapshotRestorePolicyPath
	policy, ok := fs.Read(policyPath)
	if !ok {
		t.Fatalf("%s was not written (paths: %v)", policyPath, fs.Paths())
	}
	if !strings.Contains(policy, `<action id="`+snapshotRestorePolkitAction+`">`) {
		t.Errorf("polkit action missing from %s:\n%s", policyPath, policy)
	}

	rulesPath := shared.MountRoot + snapshotRestoreRulesPath
	rules, ok := fs.Read(rulesPath)
	if !ok {
		t.Fatalf("%s was not written (paths: %v)", rulesPath, fs.Paths())
	}
	if !strings.Contains(rules, "polkit.addRule") {
		t.Errorf("polkit rule missing from %s:\n%s", rulesPath, rules)
	}

	if !hasCall(rec.CallsFor("mkdir"), "mkdir", "-p", shared.MountRoot+"/usr/local/bin") {
		t.Errorf("the helper directory was not created; calls = %v", rec.Calls())
	}
}

// TestConfigureSnapper_InstallsRestoreBeforeTheSnapshot is the ordering that is
// easy to get wrong and impossible to fix later: a snapshot runs the binaries and
// authorization rules it contains. A helper written after the "after installation"
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
	if !fs.Exists(shared.MountRoot + snapshotRestorePolicyPath) {
		t.Fatalf("%s was not installed by the snapper step", snapshotRestorePolicyPath)
	}
	if !fs.Exists(shared.MountRoot + snapshotRestoreRulesPath) {
		t.Fatalf("%s was not installed by the snapper step", snapshotRestoreRulesPath)
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
// restore, so it gets neither the helper nor its polkit authorization.
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
	if fs.Exists(shared.MountRoot + snapshotRestorePolicyPath) {
		t.Errorf("%s was installed for an ext4 install", snapshotRestorePolicyPath)
	}
	if fs.Exists(shared.MountRoot + snapshotRestoreRulesPath) {
		t.Errorf("%s was installed for an ext4 install", snapshotRestoreRulesPath)
	}
}
