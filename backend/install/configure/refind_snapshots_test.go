package configure

import (
	"errors"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// --- the snapshot menu fragment on the ESP ------------------------------------

// TestWriteRefindConfig_SnapshotFragment covers the installer half of the snapshot
// boot entries: a btrfs install gets a second menu fragment, empty, plus the
// include that pulls it in. It has to be written before the include list is
// collected, because refind.conf can only include a file that exists.
func TestWriteRefindConfig_SnapshotFragment(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemBtrfs,
		Partitions: &shared.PartitionInfo{RootUUID: "1a2b3c4d-0000-0000-0000-000000000000", RootPart: "/dev/sda2"},
	}
	if err := writeRefindConfig(shared.BootMountPoint, ctx, "vmlinuz-linux-lts", "initramfs-linux-lts.img", displayMode{}, false, false); err != nil {
		t.Fatalf("writeRefindConfig() = %v, want nil", err)
	}

	id := shared.InstallID(ctx.Partitions.RootUUID)
	path := shared.BootMountPoint + "/" + shared.RefindSnapshotEntryPath(id)
	content, ok := fs.Read(path)
	if !ok {
		t.Fatalf("snapshot menu fragment not written at %s (paths: %v)", path, fs.Paths())
	}
	if strings.Contains(content, "menuentry") {
		t.Errorf("the installer wrote a boot entry, not a placeholder:\n%s", content)
	}

	conf, _ := fs.Read(shared.BootMountPoint + "/" + shared.RefindConfigPath)
	if want := "include " + shared.RefindSnapshotEntryInclude(id); !strings.Contains(conf, want) {
		t.Errorf("refind.conf missing %q:\n%s", want, conf)
	}
	// The install's own entry is still there and still the only one that boots
	// the live system.
	if !strings.Contains(conf, "include "+shared.RefindEntryInclude(id)) {
		t.Errorf("refind.conf lost the install's own entry:\n%s", conf)
	}
}

func TestWriteRefindConfig_NoSnapshotFragmentForExt4(t *testing.T) {
	defer primitives.SetCommandRunner(primitivestest.NewRecorder())()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{
		FileSystem: shared.FileSystemExt4,
		Partitions: &shared.PartitionInfo{RootUUID: "1a2b3c4d-0000-0000-0000-000000000000", RootPart: "/dev/sda2"},
	}
	if err := writeRefindConfig(shared.BootMountPoint, ctx, "vmlinuz-linux-lts", "initramfs-linux-lts.img", displayMode{}, false, false); err != nil {
		t.Fatalf("writeRefindConfig() = %v, want nil", err)
	}

	id := shared.InstallID(ctx.Partitions.RootUUID)
	if fs.Exists(shared.BootMountPoint + "/" + shared.RefindSnapshotEntryPath(id)) {
		t.Error("an ext4 install got a snapshot menu fragment")
	}
	conf, _ := fs.Read(shared.BootMountPoint + "/" + shared.RefindConfigPath)
	if strings.Contains(conf, "snapshots.conf") {
		t.Errorf("refind.conf references a snapshot fragment for an ext4 install:\n%s", conf)
	}
}

// --- the installed-system half ------------------------------------------------

// calledWith reports whether the recorder ran exactly this argv.
func calledWith(rec *primitivestest.Recorder, want ...string) bool {
	for _, c := range rec.Calls() {
		if hasCall([][]string{c.Args}, want...) {
			return true
		}
	}
	return false
}

func TestConfigureSnapshotBootEntries(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	ctx := &shared.InstallContext{FileSystem: shared.FileSystemBtrfs}
	if err := configureSnapshotBootEntries(ctx); err != nil {
		t.Fatalf("configureSnapshotBootEntries() = %v, want nil", err)
	}

	for _, path := range []string{snapshotScriptPath, snapperPluginPath} {
		if _, ok := fs.Read(shared.MountRoot + path); !ok {
			t.Errorf("%s was not written (paths: %v)", path, fs.Paths())
			continue
		}
		if mode := fs.Mode(shared.MountRoot + path); mode&0o111 == 0 {
			t.Errorf("%s is not executable (%v)", path, mode)
		}
	}
	if _, ok := fs.Read(shared.MountRoot + snapshotUnitPath); !ok {
		t.Errorf("%s was not written", snapshotUnitPath)
	}
	if !calledWith(rec, "arch-chroot", shared.MountRoot, "systemctl", "enable", snapshotUnitName) {
		t.Errorf("the snapshot unit was not enabled; calls = %v", rec.Calls())
	}
}

// TestConfigureSnapshotBootEntries_MissingPluginDir pins the one directory the
// installer has to create itself: snapper ships no plugins directory, and writing
// into a directory that is not there is not a warning, it is a broken install.
func TestConfigureSnapshotBootEntries_MissingPluginDir(t *testing.T) {
	rec := mountedRunner().OnError("mkdir", errors.New("read-only filesystem"))
	defer primitives.SetCommandRunner(rec)()

	err := configureSnapshotBootEntries(&shared.InstallContext{FileSystem: shared.FileSystemBtrfs})
	if err == nil || !strings.Contains(err.Error(), snapperPluginDir) {
		t.Fatalf("configureSnapshotBootEntries() = %v, want the mkdir failure", err)
	}
}

// TestConfigureSnapper_InstallsSnapshotHooksAndMenu is the end-to-end shape of the
// snapper step: configs, transaction hooks, and the boot-menu machinery, all
// because the install chose btrfs.
func TestConfigureSnapper_InstallsSnapshotHooksAndMenu(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(snapperContext()); err != nil {
		t.Fatalf("ConfigureSnapper() = %v, want nil", err)
	}

	pre, ok := fs.Read(shared.MountRoot + pacmanPreHookPath)
	if !ok {
		t.Fatalf("pacman pre-transaction hook not written (paths: %v)", fs.Paths())
	}
	post, ok := fs.Read(shared.MountRoot + pacmanPostHookPath)
	if !ok {
		t.Fatalf("pacman post-transaction hook not written")
	}
	for _, want := range []string{"When = PreTransaction", "Depends = snapper", "create -t pre", pacmanPreHookState} {
		if !strings.Contains(pre, want) {
			t.Errorf("pre hook missing %q:\n%s", want, pre)
		}
	}
	for _, want := range []string{"When = PostTransaction", "Depends = snapper", "--pre-number", pacmanPreHookState} {
		if !strings.Contains(post, want) {
			t.Errorf("post hook missing %q:\n%s", want, post)
		}
	}
	// Both halves trigger on the same transactions, and only on ones that change
	// packages: a bare -Sy or a cache clean has no targets and no snapshot pair.
	for _, hook := range []string{pre, post} {
		for _, want := range []string{"Operation = Install", "Operation = Upgrade", "Operation = Remove", "Target = *"} {
			if !strings.Contains(hook, want) {
				t.Errorf("hook missing %q:\n%s", want, hook)
			}
		}
		// pacman allows exactly one Type per hook: declaring two makes pacman warn
		// and the last one silently wins, which would leave the hooks matching
		// nothing useful. Package is the type that covers every transaction.
		if n := strings.Count(hook, "Type = "); n != 1 {
			t.Errorf("hook declares %d Type lines, want exactly 1:\n%s", n, hook)
		}
		if !strings.Contains(hook, "Type = Package") {
			t.Errorf("hook does not trigger on Type = Package:\n%s", hook)
		}
	}

	for _, path := range []string{snapshotScriptPath, snapperPluginPath, snapshotUnitPath} {
		if !fs.Exists(shared.MountRoot + path) {
			t.Errorf("%s was not installed by the snapper step", path)
		}
	}
}

// TestConfigureSnapper_NoSnapshotHooksForExt4 keeps the ext4 gate: no snapper,
// no transaction snapshots, no snapshot boot entries.
func TestConfigureSnapper_NoSnapshotHooksForExt4(t *testing.T) {
	rec := mountedRunner()
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(&shared.InstallContext{FileSystem: shared.FileSystemExt4}); err != nil {
		t.Fatalf("ConfigureSnapper(ext4) = %v, want nil", err)
	}

	for _, path := range []string{pacmanPreHookPath, pacmanPostHookPath, snapshotScriptPath, snapperPluginPath, snapshotUnitPath} {
		if fs.Exists(shared.MountRoot + path) {
			t.Errorf("%s was installed for an ext4 install", path)
		}
	}
}

// TestConfigureSnapper_SkipsMenuWithoutRootConfig: the boot menu lists roots, so
// when the root config could not be created there is nothing to list and no
// reason to install a generator for it.
func TestConfigureSnapper_SkipsMenuWithoutRootConfig(t *testing.T) {
	rec := mountedRunner().OnArgsError(
		[]string{"findmnt", "-n", "-o", "SOURCE", shared.MountRoot + shared.BtrfsSnapshotsMountPoint},
		errors.New("not mounted"))
	defer primitives.SetCommandRunner(rec)()
	fs := primitivestest.NewMemFS()
	defer primitives.SetFileSystem(fs)()

	if err := ConfigureSnapper(snapperContext()); err != nil {
		t.Fatalf("ConfigureSnapper() = %v, want nil", err)
	}

	if !fs.Exists(shared.MountRoot + pacmanPostHookPath) {
		t.Error("the transaction hooks were skipped although the home config exists")
	}
	if fs.Exists(shared.MountRoot + snapshotScriptPath) {
		t.Error("the snapshot menu generator was installed with no root config")
	}
}

// --- the generated files themselves -------------------------------------------

// TestSnapshotMenuScript pins the parts of the generator that the installed system
// depends on: where it writes, what it copies, and that it never invents a kernel
// command line of its own.
func TestSnapshotMenuScript(t *testing.T) {
	script := snapshotMenuScript

	for _, want := range []string{
		"#!/bin/sh",
		"set -u",
		"MAX_SNAPSHOTS=",
		`SNAPPER_CONFIG="root"`,
		// The fragment it owns, spelled the way shared.RefindSnapshotEntryName
		// spells it.
		`out="$esp/EFI/refind/aelum/$install_id-snapshots.conf"`,
		// Everything bootable is copied from this install's own entry, so LUKS and
		// LVM options stay the installer's business.
		`base_entry="$esp/EFI/refind/aelum/$install_id.conf"`,
		`base_options=$(sed -n 's/^[[:space:]]*options`,
		`rootflags=subvol=$subvol_base/$1/snapshot`,
		// One entry, with the snapshots inside it: snapper keeps dozens, and a row
		// of icons that size is not a menu. A submenuentry inherits the loader, so
		// it only states which subvolume becomes root.
		`echo "menuentry \"$target_entry_title\" {"`,
		`echo "    icon $SNAPSHOT_ICON"`,
		`echo "    submenuentry \"$title\" {"`,
		// Menu naming (Option A): the number stays, because it is what snapper and
		// btrfs-assistant show, and the words say what the state is for.
		`title="Snapshot $number"`,
		`entry_title="Aelum snapshot $number"`,
		`phrase="Before system update"`,
		`phrase="After system update"`,
		`case "$phrase" in pacman) phrase="" ;;`,
		// '|' is the field separator of the collected list, so it cannot be allowed
		// inside a field: a description containing one would shift every field after
		// it and put a title in the wrong column.
		`tr -cd ' -~' | tr -d '|'`,
		// The entry boots the newest snapshot taken before a change, else the newest
		// important one, else the newest one - not the newest one full stop, which
		// is usually the state that already contains the change.
		`[ "$type" = pre ]`,
		`[ "$important" = yes ]`,
		// snapper writes userdata as <key>/<value> pairs - measured on a real
		// install's "after installation" snapshot - so that is what the generator
		// reads, with the flat spelling accepted too rather than treating a snapshot
		// it cannot parse as one nobody marked.
		`<key>important</key>[[:space:]]*<value>`,
		// A timeline snapshot is a timestamp, not a decision: it stays in the
		// submenu and is never the recovery entry.
		`case "$cleanup" in timeline) continue ;; esac`,
		// Snapper keeps a dozen timeline snapshots, so a plain newest-N list would
		// crowd every transaction pair out of the menu: the states someone had a
		// reason to keep are collected first.
		`sort -t'|' -k1,1n -k2,2rn`,
		// The snapshot that *is* the system is not offered: this install's own entry
		// already boots it (it names no subvolume), and a rollback moves it.
		`if [ "$number" = "$default_number" ]; then`,
		`default_number=${default_path##*/` + shared.BtrfsSnapshotsDirName + `/}`,
		// No snapshots, no entry: an icon that boots nothing is worse than no icon.
		`if [ -n "$target_number" ]; then`,
		// The install id is the installer's derivation: eight hex characters of the
		// root filesystem UUID.
		`install_id=$(printf '%s' "$root_uuid" | tr 'A-Z' 'a-z' | tr -cd 'a-z0-9' | cut -c1-8)`,
		// Written to a temporary name and renamed, so rEFInd never reads a fragment
		// that is half-written.
		`tmp="$out.tmp"`,
		`mv -f "$tmp" "$out"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generator is missing %q", want)
		}
	}

	// A backtick inside one of these files would have ended the Go raw string
	// early, and a $() or "" in a title would end the rEFInd stanza early.
	if strings.Contains(script, "```") {
		t.Error("generator contains a backtick")
	}
	// rEFInd draws its menu with a bitmap font covering ASCII 32-126 and nothing
	// else: anything wider becomes a fallback box. The generator is ASCII, and
	// menu_text is what keeps the titles it builds ASCII too.
	for i, line := range strings.Split(script, "\n") {
		// A Go comment inside the raw string is not a shell comment: "//" is a
		// command to /bin/sh, and the apostrophe in the text after it opens a quote
		// that swallows the lines which follow - which is how menu_text once stopped
		// being defined at all.
		if strings.HasPrefix(line, "//") {
			t.Errorf("generator line %d is a Go comment, not a shell comment: %q", i+1, line)
		}
		for _, r := range line {
			if r > 127 {
				t.Errorf("generator line %d is not ASCII, and rEFInd's font is: %q", i+1, line)
				break
			}
		}
	}
	if !strings.Contains(script, `tr -cd ' -~'`) {
		t.Error("generator does not cut menu titles to printable ASCII")
	}
	// Only the free-text part of a title is length-bounded. Cutting a whole title
	// would cut the date, which is what a snapshot is actually identified by.
	if !strings.Contains(script, "menu_phrase") || !strings.Contains(script, "cut -c1-48") {
		t.Error("generator does not bound only the description part of a title")
	}
	// The snapshot entry carries the theme's history glyph rather than the Aelum
	// mark, so it never reads as a second copy of the system.
	if !strings.Contains(script, `SNAPSHOT_ICON="/`+shared.RefindSnapshotIconPath+`"`) {
		t.Errorf("generator does not use the theme's snapshot icon (%s)", shared.RefindSnapshotIconPath)
	}
	// The live root is a snapshot, and a snapshot does not contain its parent's
	// subvolumes, so the only history this system can read is the mounted one. With
	// no mount the generator must refuse to rewrite the fragment from an empty list:
	// that would delete the entry being booted.
	if !strings.Contains(script, `findmnt -n -o SOURCE "$snapshot_dir"`) {
		t.Error("generator rewrites the boot menu even when it cannot see the history")
	}
	// What the system is is named by the filesystem's default subvolume - the pointer
	// a rollback moves - so the path of the history is derived from it, not from how
	// / happens to be mounted.
	if !strings.Contains(script, `btrfs subvolume get-default "$root"`) {
		t.Error("generator does not derive the history from the default subvolume")
	}
	// The installer runs the same script against the system it is still installing,
	// so every path it resolves has to be under the root it was given. A single
	// absolute one would make the install-time run write about the live system.
	if !strings.Contains(script, `--root)`) {
		t.Error("generator has no --root, so the installer cannot write the menu before the first boot")
	}
	// The default root is "/" - the service and snapper's plugin run this with no
	// argument at all - and stripping a trailing slash used to turn that into an
	// empty path. "btrfs subvolume get-default "" then fails, and the stanzas name
	// subvolumes that are not in the filesystem tree: an entry that cannot mount a
	// root. Measured on a real install, where every snapshot entry stopped booting
	// after the install's own first boot rewrote the menu this way.
	if !strings.Contains(script, `case $root in
    "/") ;;
    */) root=${root%/} ;;
esac`) {
		t.Error("generator can reduce the default root \"/\" to an empty path")
	}
	// And when btrfs does not answer at all there is no correct path to name a
	// snapshot by, so the fragment already on the ESP is the better answer.
	if !strings.Contains(script, `cannot read the default subvolume`) {
		t.Error("generator guesses a history path when the default subvolume is unknown")
	}
	for _, want := range []string{
		`root="/"`,
		`for candidate in "$root/efi" "$root/boot"`,
		`findmnt -no UUID "$root"`,
		`"$root/etc/snapper/configs/$SNAPPER_CONFIG"`,
		`snapshot_dir="$root${snapshot_dir%/}/` + shared.BtrfsSnapshotsDirName + `"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generator resolves %q outside the root it was given", want)
		}
	}
}

func TestSnapperSnapshotPlugin(t *testing.T) {
	plugin := snapperSnapshotPlugin
	for _, want := range []string{
		"#!/bin/sh",
		"create-snapshot-post|delete-snapshot-post",
		`[ "$2" = "/" ] || exit 0`,
		"systemctl start --no-block " + snapshotUnitName,
		"exit 0",
	} {
		if !strings.Contains(plugin, want) {
			t.Errorf("snapper plugin is missing %q", want)
		}
	}
	if !strings.HasPrefix(snapperPluginPath, snapperPluginDir+"/1") {
		t.Error("snapper plugin name does not start with a digit, so snapper would ignore it")
	}
}

func TestSnapshotMenuUnit(t *testing.T) {
	unit := snapshotMenuUnit
	for _, want := range []string{
		"[Unit]",
		"Type=oneshot",
		"ExecStart=" + snapshotScriptPath,
		"After=local-fs.target",
		"[Install]",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit is missing %q:\n%s", want, unit)
		}
	}
}

// TestSnapshotUnitPathIsWhatSystemctlEnables: the unit is enabled by name, so the
// name and the file have to agree or systemd enables nothing.
func TestSnapshotUnitPathIsWhatSystemctlEnables(t *testing.T) {
	if got := "/etc/systemd/system/" + snapshotUnitName; got != snapshotUnitPath {
		t.Errorf("unit name %q does not match its path %q", snapshotUnitName, snapshotUnitPath)
	}
}

// TestWriteInitialSnapshotMenu: the install-time run is the installed system's own
// script, asked about the target with --root rather than through a chroot (a chroot
// shows the live system's mount table, which is the mistake this whole layout was
// built to stop). A failure here costs a boot menu, not an install: the placeholder
// is a valid file and the installed system rebuilds it at every boot.
func TestWriteInitialSnapshotMenu(t *testing.T) {
	t.Run("runs the installed script against the target", func(t *testing.T) {
		rec := primitivestest.NewRecorder()
		defer primitives.SetCommandRunner(rec)()

		if err := writeInitialSnapshotMenu(&shared.InstallContext{}); err != nil {
			t.Fatalf("writeInitialSnapshotMenu() = %v, want nil", err)
		}
		if !hasCall(rec.CallsFor(shared.MountRoot+snapshotScriptPath),
			shared.MountRoot+snapshotScriptPath, "--root", shared.MountRoot) {
			t.Errorf("generator was not run against the target; calls = %v", rec.Calls())
		}
		for _, c := range rec.Calls() {
			if c.Args[0] == "arch-chroot" {
				t.Errorf("the menu generator was run in a chroot, where it would read the live system's mounts: %v", c.Args)
			}
		}
	})

	t.Run("a failed run is not a failed install", func(t *testing.T) {
		rec := primitivestest.NewRecorder().OnArgsError(
			[]string{shared.MountRoot + snapshotScriptPath}, errors.New("no mounted ESP"))
		defer primitives.SetCommandRunner(rec)()

		if err := writeInitialSnapshotMenu(&shared.InstallContext{}); err != nil {
			t.Fatalf("writeInitialSnapshotMenu() = %v, want nil", err)
		}
	})
}
