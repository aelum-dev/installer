package format

import (
	"fmt"
	"time"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// The snapshot the system is installed into
// ============================================================================
//
// snapper's rollback names the running system by the filesystem's default
// subvolume, and it only accepts that subvolume as one of its own numbered
// snapshots: Btrfs::getDefault resolves the default id to a path and requires it to
// match "<number>/snapshot" (snapper/Btrfs.cc, idToNum). A layout whose live root
// is a plain subvolume named @ is outside the shape snapper assumes, and
// `snapper rollback` refuses to run on it at all - "Cannot detect ambit since
// default subvolume is unknown" - which is the report every Arch/EndeavourOS
// install of this layout gets.
//
// So the installer builds the shape snapper does assume, before anything is
// installed, in the order snapper's own installation-helper uses (step1, and the
// step_filesystem fallback that does the same thing with plain btrfs commands):
//
//	@/.snapshots                       a subvolume: this install's history
//	@/.snapshots/1/                    plain directory: info.xml lives here
//	@/.snapshots/1/snapshot            the subvolume the system is installed into
//	@/.snapshots/1/snapshot/.snapshots mount point; fstab mounts the real
//	                                   history here (see configure/fstab.go)
//	default subvolume                  = @/.snapshots/1/snapshot
//
// @ is never mounted and never booted. It is the container that keeps the history
// at one fixed path whatever the live root is: a rollback puts a copy of an older
// snapshot where this one is and moves @/.snapshots inside it, so the history's
// path, its numbers, the fstab and the boot entry all stay exactly as they are.
//
// info.xml is written here rather than left to snapper because snapper does not
// exist in the target yet at this point. Its shape is snapper's own
// (Snapshot::writeInfo: type, num, date in UTC as "%F %T", optional description),
// and without it snapper treats the directory as an unused leftover and removes it
// (Snapshots::nextNumber).

// createRootSnapshot builds that skeleton under the already-mounted filesystem tree
// root and returns the path of the subvolume to install into, from the top level of
// the filesystem ("@/.snapshots/1/snapshot").
func createRootSnapshot(treeMount string) (string, error) {
	root := shared.RootSubvolume()
	if root == nil {
		return "", fmt.Errorf("no subvolume mounted at / in the btrfs scheme")
	}

	history := treeMount + "/" + shared.SnapshotDirName(root)
	if out, err := primitives.Run("btrfs", "subvolume", "create", history).CombinedOutput(); err != nil {
		return "", fmt.Errorf("create %s: %w: %s", shared.SnapshotDirName(root), err, out)
	}

	numberDir := fmt.Sprintf("%s/%d", history, shared.BtrfsInitialSnapshotNumber)
	if err := primitives.Run("mkdir", "-p", numberDir).Run(); err != nil {
		return "", fmt.Errorf("create %s: %w", numberDir, err)
	}

	live := numberDir + "/" + shared.BtrfsSnapshotSubvolumeName
	if out, err := primitives.Run("btrfs", "subvolume", "create", live).CombinedOutput(); err != nil {
		return "", fmt.Errorf("create %s: %w: %s", live, err, out)
	}

	if err := writeInitialSnapshotInfo(numberDir); err != nil {
		return "", err
	}

	// The mount point for /.snapshots inside the live root. The history itself is a
	// subvolume and is not part of a snapshot of its parent, so the live root needs
	// a plain directory for the mount to land in - the same one snapper's
	// step_filesystem creates.
	if err := primitives.Run("mkdir", "-p", live+"/"+shared.BtrfsSnapshotsDirName).Run(); err != nil {
		return "", fmt.Errorf("create the .snapshots mount point in %s: %w", live, err)
	}

	return shared.LiveSubvolumeName(root), nil
}

// writeInitialSnapshotInfo writes the info.xml that makes this directory a snapshot
// snapper recognises. It is described the way SUSE describes the same snapshot in
// its own rollback documentation: the first root filesystem, never cleaned up.
func writeInitialSnapshotInfo(numberDir string) error {
	date := time.Now().UTC().Format("2006-01-02 15:04:05")
	content := fmt.Sprintf(`<?xml version="1.0"?>
<snapshot>
  <type>single</type>
  <num>%d</num>
  <date>%s</date>
  <description>first root filesystem</description>
</snapshot>
`, shared.BtrfsInitialSnapshotNumber, date)

	return primitives.WriteFileMode(numberDir+"/info.xml", content, 0o644)
}
