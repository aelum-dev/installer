package configure

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// TestSnapshotStateUnitNamesMatchTheirMounts: a mount unit must be named after the
// path it mounts, with every "/" replaced by "-". systemd refuses a mount unit
// whose name does not match its Where=, so a name spelled by hand would be a unit
// that never loads.
func TestSnapshotStateUnitNamesMatchTheirMounts(t *testing.T) {
	if len(snapshotStateDirs) == 0 {
		t.Fatal("no snapshot-boot state directories are handled at all")
	}
	for _, where := range snapshotStateDirs {
		name := snapshotStateUnitName(where)
		want := strings.ReplaceAll(strings.TrimPrefix(where, "/"), "/", "-") + ".mount"
		if name != want {
			t.Errorf("snapshotStateUnitName(%q) = %q, want %q", where, name, want)
		}
		if !strings.HasPrefix(where, "/") {
			t.Errorf("state directory %q is not an absolute path", where)
		}
		if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
			t.Errorf("unit name %q is not the escaped form of %q", name, where)
		}
		if !strings.HasSuffix(name, ".mount") {
			t.Errorf("unit name %q is not a mount unit", name)
		}
	}
	if got := snapshotStateUnitNames(); len(got) != len(snapshotStateDirs) {
		t.Errorf("snapshotStateUnitNames() returned %d names for %d directories", len(got), len(snapshotStateDirs))
	}
}

// TestSnapshotStateGenerator pins the parts of the generator that a snapshot boot
// depends on.
func TestSnapshotStateGenerator(t *testing.T) {
	script := snapshotStateGenerator
	for _, want := range []string{
		"#!/bin/sh",
		"set -u",
		// systemd's calling convention: the first argument is where units go.
		`gen_dir=${1:-}`,
		`[ -d "$gen_dir" ] || exit 0`,
		// The detector: the subvolume the root is on, and the one the system boots
		// by default. Both are readable at generator time; a writability probe is
		// not (an ordinary boot is read-only there too).
		`/proc/self/mountinfo`,
		`match($0, /subvolid=[0-9]+/)`,
		`btrfs subvolume get-default /`,
		`[ "$root_subvol" != "$default_subvol" ] || exit 0`,
		// The test seam, and the reason it exists: this is otherwise only ever
		// observable in a real boot. They choose where the information comes from,
		// not what the answer is.
		`AELUM_MOUNTINFO`,
		`AELUM_GET_DEFAULT`,
		// Every directory the installer knows about, and only those.
		`for where in ` + strings.Join(snapshotStateDirs, " ") + `; do`,
		// A tmpfs, mounted where systemd mounts a fstab entry.
		`echo "What=tmpfs"`,
		`echo "Where=$where"`,
		`echo "Type=tmpfs"`,
		`X-mount.mkdir`,
		// Before local-fs.target, and before the unit this all exists for.
		`echo "After=local-fs-pre.target"`,
		`echo "Before=local-fs.target systemd-logind.service"`,
		// A mount unit named after its path.
		`unit_name() {`,
		`printf '%s.mount' "${1#/}" | tr '/' '-'`,
		`> "$gen_dir/$name"`,
		// And pulled in. A mount unit is started by whatever requires it, and
		// nothing requires a unit nobody has heard of: measured on a real snapshot
		// boot, /var/lib/systemd was mounted only because systemd-logind and four
		// other units have RequiresMountsFor= on it, and /var/lib/greeter - which
		// nobody asks for - stayed inactive. This is the link systemd's own
		// generators write.
		`mkdir -p "$gen_dir/local-fs.target.requires"`,
		`ln -sf "../$name" "$gen_dir/local-fs.target.requires/$name"`,
		// The announcement. The generator is the only thing that knows a snapshot
		// boot is one at the moment it happens; the prompt unit and the restore
		// helper both read this marker, and the number in it is what the prompt
		// shows. It is written only past the snapshot-boot guard - an ordinary boot
		// writes nothing at all - and the mount root the number is cut from is the
		// same mountinfo field the detector already reads.
		`print $4, substr($0, RSTART + 9, RLENGTH - 9)`,
		`AELUM_SNAPSHOT_MARKER`,
		snapshotBootMarkerPath,
		`printf 'SNAPSHOT=%s\n' "$snapshot_number" > "$marker" || true`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generator is missing %q", want)
		}
	}

	// The condition that looks right and is not. Measured on a real snapshot boot:
	// ConditionPathIsReadWrite=!/ was unmet while the same root returned EROFS on
	// every write, because a btrfs read-only subvolume mounted with rw looks
	// read-write to the mount table. Naming it in a comment is fine - it is the
	// reason this generator exists - but using it as a directive would silently
	// turn the whole feature off. The same applies to the two shell traps below:
	// describing them is the point of the comment, acting on them is the bug.
	var code []string
	for _, line := range strings.Split(script, "\n") {
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			code = append(code, line)
		}
	}
	for _, line := range code {
		if strings.Contains(line, "ConditionPathIsReadWrite") {
			t.Errorf("generator decides with ConditionPathIsReadWrite, which is unmet in a real snapshot boot: %q", line)
		}
	}
	// A generator is not a service: it writes units and exits.
	for _, unwanted := range []string{"[Service]", "ExecStart=", "systemctl "} {
		if strings.Contains(strings.Join(code, "\n"), unwanted) {
			t.Errorf("generator contains %q, which is not what a generator does", unwanted)
		}
	}
	// systemd passes three arguments to a generator. Measured on a real boot: a
	// bare $2 under "set -u" failed the script on every boot, which is the same
	// failure as having no generator at all - so this script never looks past the
	// first argument.
	for _, unwanted := range []string{"$2", "$3"} {
		if strings.Contains(strings.Join(code, "\n"), unwanted) {
			t.Errorf("generator reads %s, which systemd does not guarantee to pass: a bare positional read under set -u fails every real boot", unwanted)
		}
	}
	// The other half of the same measurement: a failed redirection on a *special
	// builtin* (": > file") exits a non-interactive shell. That is how this
	// generator used to die with status 1 before it wrote a single unit.
	if strings.Contains(strings.Join(code, "\n"), ": > ") {
		t.Error("generator redirects into a special builtin (\": > file\"), which exits the shell when the write fails")
	}
	// Same rule as the menu generator: a Go comment inside the raw string is a
	// command to /bin/sh.
	for i, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, "//") {
			t.Errorf("generator line %d is a Go comment, not a shell comment: %q", i+1, line)
		}
	}
}

// TestConfigureSnapshotStateDirs installs the generator, executable, in the
// directory systemd reads - and a failure there is an error, because a generator
// written into a directory that is not there is no generator.
func TestConfigureSnapshotStateDirs(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := configureSnapshotStateDirs(&shared.InstallContext{FileSystem: shared.FileSystemBtrfs}); err != nil {
		t.Fatalf("configureSnapshotStateDirs() = %v, want nil", err)
	}

	path := shared.MountRoot + snapshotStateGeneratorPath
	content, ok := fs.Read(path)
	if !ok {
		t.Fatalf("%s was not written (paths: %v)", path, fs.Paths())
	}
	if !strings.Contains(content, "#!/bin/sh") {
		t.Errorf("%s is not a script:\n%s", path, content)
	}
	if mode := fs.Mode(path); mode&0o111 == 0 {
		t.Errorf("%s is not executable (%v): systemd runs generators, it does not read them", path, mode)
	}
	if !hasCall(rec.CallsFor("mkdir"), "mkdir", "-p", shared.MountRoot+systemdGeneratorDir) {
		t.Errorf("the generator directory was not created; calls = %v", rec.Calls())
	}
	// A generator is found by systemd; nothing is enabled.
	for _, c := range rec.Calls() {
		if c.Args[0] == "arch-chroot" && len(c.Args) > 3 && c.Args[3] == "enable" {
			t.Errorf("a generator was enabled (%v); systemd runs generators by path", c.Args)
		}
	}
}

// TestConfigureSnapshotStateDirs_MissingGeneratorDir: writing into a directory that
// could not be created is not a warning, it is a feature that does not exist.
func TestConfigureSnapshotStateDirs_MissingGeneratorDir(t *testing.T) {
	rec := mountedRunner().OnError("mkdir", errors.New("read-only filesystem"))
	defer primitives.SetCommandRunner(rec)()

	err := configureSnapshotStateDirs(&shared.InstallContext{FileSystem: shared.FileSystemBtrfs})
	if err == nil || !strings.Contains(err.Error(), systemdGeneratorDir) {
		t.Fatalf("configureSnapshotStateDirs() = %v, want the mkdir failure", err)
	}
}

// TestConfigureSnapper_InstallsSnapshotStateGeneratorBeforeTheSnapshot is the part
// that is easy to get wrong and impossible to fix later: a snapshot boots the
// generators and units the snapshot contains. A generator written after the "after
// installation" snapshot is absent from every snapshot this system has, and a
// snapshot boot comes up at a text login.
func TestConfigureSnapper_InstallsSnapshotStateGeneratorBeforeTheSnapshot(t *testing.T) {
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

	path := shared.MountRoot + snapshotStateGeneratorPath
	if !fs.Exists(path) {
		t.Fatalf("%s was not installed by the snapper step", path)
	}
	written := -1
	for i, c := range rec.Calls() {
		if len(c.Args) > 0 && c.Args[0] == "mkdir" && strings.Contains(strings.Join(c.Args, " "), systemdGeneratorDir) {
			written = i
			break
		}
	}
	if written < 0 {
		t.Fatalf("the generator directory was not created; calls = %v", rec.Calls())
	}
	if written > snapshot {
		t.Error("the snapshot state generator was installed after the snapshot was taken, so it is not inside it")
	}
}

// TestConfigureSnapper_NoSnapshotStateGeneratorForExt4: an ext4 install has no
// snapshot to boot, so it gets no generator over its own state directories.
func TestConfigureSnapper_NoSnapshotStateGeneratorForExt4(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(&shared.InstallContext{FileSystem: shared.FileSystemExt4}); err != nil {
		t.Fatalf("ConfigureSnapper(ext4) = %v, want nil", err)
	}
	if fs.Exists(shared.MountRoot + snapshotStateGeneratorPath) {
		t.Errorf("%s was installed for an ext4 install", snapshotStateGeneratorPath)
	}
}
