package configure

import (
	"errors"
	"slices"
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
	important, ok := fs.Read(shared.MountRoot + pacmanImportantHookPath)
	if !ok {
		t.Fatalf("pacman important-transaction hook not written (paths: %v)", fs.Paths())
	}
	for _, want := range []string{"When = PreTransaction", "Depends = snapper", "create -t pre", pacmanPreHookState} {
		if !strings.Contains(pre, want) {
			t.Errorf("pre hook missing %q:\n%s", want, pre)
		}
	}
	for _, want := range []string{"When = PostTransaction", "Depends = snapper", "--pre-number", pacmanPreHookState, pacmanPairState} {
		if !strings.Contains(post, want) {
			t.Errorf("post hook missing %q:\n%s", want, post)
		}
	}
	// The pre hook clears both state files first. A transaction killed between its
	// pre and post hooks leaves a number behind, and without this the next
	// transaction pairs with it: one pair whose diff spans two transactions.
	if !strings.Contains(pre, "rm -f "+pacmanPreHookState+" "+pacmanPairState) {
		t.Errorf("pre hook does not clear the state of an interrupted transaction:\n%s", pre)
	}
	// --no-dbus everywhere: the default path asks snapperd, and a snapperd that is
	// masked or crashed would drop the undo path with no message and no failed
	// install. The hooks run as root, so snapper can do the work itself.
	for name, hook := range map[string]string{"pre": pre, "post": post, "important": important} {
		if !strings.Contains(hook, "snapper --no-dbus -c "+snapperRootConfig) {
			t.Errorf("%s hook talks to snapperd instead of running snapper directly:\n%s", name, hook)
		}
	}
	// The third half marks the pair it did not create: the snapshots already exist,
	// and marking them is what keeps a kernel update's state out of the ordinary
	// count quota. Its Trigger is the narrow one, so it runs only for those.
	for _, want := range []string{
		"When = PostTransaction", "Depends = snapper", pacmanPairState,
		"modify -u important=yes",
		"Target = " + pacmanImportantTargetList,
	} {
		if !strings.Contains(important, want) {
			t.Errorf("important hook missing %q:\n%s", want, important)
		}
	}
	if strings.Contains(important, "Target = *") {
		t.Error("the important hook triggers on every transaction, so nothing is distinguished:\n" + important)
	}
	for _, pkg := range []string{"linux-lts", "glibc", "systemd", "mkinitcpio", "btrfs-progs", "refind"} {
		if !slices.Contains(pacmanImportantTargets, pkg) {
			t.Errorf("%s is not an important transaction; a rollback of it would be cleaned away", pkg)
		}
	}
	// All three trigger on the same operations and only on ones that change
	// packages: a bare -Sy or a cache clean has no targets and no snapshot pair.
	// Only the two that create the pair trigger on every target.
	for name, hook := range map[string]string{"pre": pre, "post": post, "important": important} {
		for _, want := range []string{"Operation = Install", "Operation = Upgrade", "Operation = Remove"} {
			if !strings.Contains(hook, want) {
				t.Errorf("%s hook missing %q:\n%s", name, want, hook)
			}
		}
		if name != "important" && !strings.Contains(hook, "Target = *") {
			t.Errorf("%s hook does not trigger on every package target:\n%s", name, hook)
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
		// The Exec line must survive pacman's argument splitting as one command:
		// the sh -c wrapper's quotes are the only single quotes on the line. A
		// single quote nested inside ends the quoting there, pacman splits the
		// Exec at the next space, and sh receives a truncated command — the exact
		// failure an earlier printf '%s %s\n' caused on a real installed system,
		// where every transaction died on "unexpected end of file from `if'".
		// The fake command runner never parses the hook, so this is the seam
		// that catches it.
		execLine := ""
		for _, line := range strings.Split(hook, "\n") {
			if strings.HasPrefix(line, "Exec = ") {
				execLine = strings.TrimPrefix(line, "Exec = ")
			}
		}
		if execLine == "" {
			t.Errorf("%s hook has no Exec line:\n%s", name, hook)
			continue
		}
		if !strings.HasPrefix(execLine, `/bin/sh -c '`) || !strings.HasSuffix(execLine, `'`) {
			t.Errorf("%s hook Exec is not a single-quoted sh command: %s", name, execLine)
		}
		if n := strings.Count(execLine, `'`); n != 2 {
			t.Errorf("%s hook Exec carries %d single quotes, want only the wrapper pair: %s", name, n, execLine)
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

	for _, path := range []string{pacmanPreHookPath, pacmanPostHookPath, pacmanImportantHookPath, snapshotScriptPath, snapperPluginPath, snapshotUnitPath} {
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
		// Menu naming: number, date and type - the columns of snapper's own list,
		// minus its cleanup column. rEFInd neither wraps nor ellipsizes menu text
		// - a 60-character title lost its selection text entirely on a 1280x720
		// screen - so the titles are a bounded shape. The words that say what a
		// state is for belong to the restore prompt.
		`title="Snapshot $number"`,
		`entry_title="Aelum snapshot $number"`,
		`stamp=$(menu_text "$(human_date "$date")")`,
		`title="$title - $stamp"`,
		`entry_title="$entry_title - $stamp"`,
		`title="$title - $type"`,
		`entry_title="$entry_title - $type"`,
		// The date is spelled the way snapper's list spells it - short date,
		// 12-hour clock with an unpadded hour - so a menu title and a snapper row
		// name the same state in the same words.
		`printf '%s/%s/%s %s:%s %s'`,
		`hour=$((hour % 12))`,
		// The marker's phrase fallback: without a description, the snapshot's own
		// type names it; "pacman" is the installer's tag, not words for a person.
		`phrase="Before system update"`,
		`phrase="After system update"`,
		`case "$phrase" in pacman) phrase="" ;;`,
		// '|' is the field separator of the collected list, so it cannot appear
		// in any field; menu_text is what keeps the one field read from info.xml
		// (the date) free of it and of anything rEFInd cannot draw.
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
		// A snapshot boot carries the words that say what the booted snapshot is
		// for: the prompt reads phrase and date out of the same marker the state
		// generator writes the number into. The phrase is the snapshot's own
		// description, unaltered and in any language - the prompt is a desktop
		// window, not a firmware menu. Only a run from inside this boot enriches
		// it - --root is a history belonging to a system that is not the one running.
		"marker=${AELUM_SNAPSHOT_MARKER:-" + shared.SnapshotBootMarkerPath + "}",
		`if [ "$root" = "/" ] && [ -r "$marker" ]; then`,
		`boot_number=$(sed -n 's/^SNAPSHOT=//p' "$marker" | head -n 1)`,
		`printf 'SNAPSHOT_PHRASE="%s"\n' "$(quote "$boot_phrase")"`,
		`printf 'SNAPSHOT_DATE="%s"\n' "$(quote "$boot_stamp")"`,
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
	// No description reaches a menu title at all: the titles are number, date
	// and type. menu_phrase - the old length-bounded description half - must stay
	// gone, and the description must reach the user only through the marker and
	// the prompt.
	if strings.Contains(script, "menu_phrase") {
		t.Error("generator still builds a menu phrase; rEFInd titles carry the number, the date and the type only")
	}
	if !strings.Contains(script, `phrase=$(printf '%s' "$2" | tr -d '\n\r'`) {
		t.Error("snapshot_phrase does not take the description verbatim")
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
		// Not only create and delete: a transactional rollback changes the default
		// subvolume and creates no snapshot, and snapper modify rewrites the
		// description and important= marking a menu title is built from. A plugin
		// that ignores those keeps a menu naming a state the system no longer is.
		"create-snapshot-post|delete-snapshot-post|set-default-snapshot-post|modify-snapshot-post|set-read-only-post",
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
