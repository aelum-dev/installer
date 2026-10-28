package shared

import (
	"strconv"
	"strings"
)

// BtrfsSubvolume represents a btrfs subvolume with its mount point and properties.
type BtrfsSubvolume struct {
	Name        string
	MountPoint  string
	Compression string
	NoDataCow   bool
	// SnapshotRoot marks the subvolume that holds the system. The installer does
	// not install into it directly: it installs into the initial snapshot inside
	// it, and that snapshot is what the filesystem's default subvolume points at.
	// Nothing mounts or boots the named subvolume itself - it is the container the
	// history lives in, which is what keeps the history at one fixed path across a
	// rollback.
	SnapshotRoot bool
}

// btrfsSubvolumes is the canonical btrfs subvolume scheme.
// Used by format_disk.go and generate_fstab.go to ensure consistency.
var btrfsSubvolumes = []BtrfsSubvolume{
	{Name: "@", MountPoint: "/", Compression: "zstd", SnapshotRoot: true},
	{Name: "@home", MountPoint: "/home", Compression: "zstd", NoDataCow: false},
	{Name: "@srv", MountPoint: "/srv", Compression: "zstd", NoDataCow: false},
	{Name: "@cache", MountPoint: "/var/cache", Compression: "", NoDataCow: true},
	{Name: "@tmp", MountPoint: "/var/tmp", Compression: "", NoDataCow: true},
	{Name: "@log", MountPoint: "/var/log", Compression: "zstd", NoDataCow: false},
}

// GetBtrfsSubvolumes returns the standard btrfs subvolume scheme.
// Used by format_disk.go and generate_fstab.go to ensure consistency.
func GetBtrfsSubvolumes() []BtrfsSubvolume {
	return btrfsSubvolumes
}

// RootSubvolume returns the subvolume the layout mounts at "/", or nil for a
// scheme without one (an ext4 install).
func RootSubvolume() *BtrfsSubvolume {
	return FindSubvolumeByMountPoint(btrfsSubvolumes, "/")
}

// LiveSubvolumeName returns the path, from the top level of the filesystem, of the
// subvolume that holds the running system: the initial snapshot for the root
// config, the named subvolume for every other one. It is what the installer mounts
// during an install and what snapper's rollback replaces.
func LiveSubvolumeName(sv *BtrfsSubvolume) string {
	if sv == nil {
		return ""
	}
	if !sv.SnapshotRoot {
		return sv.Name
	}
	return SubvolumePath(SnapshotDirName(sv),
		SubvolumePath(decString(BtrfsInitialSnapshotNumber), BtrfsSnapshotSubvolumeName))
}

// SnapshotDirName returns the path, from the top level of the filesystem, of the
// .snapshots subvolume belonging to one subvolume.
func SnapshotDirName(sv *BtrfsSubvolume) string {
	if sv == nil {
		return BtrfsSnapshotsDirName
	}
	return SubvolumePath(sv.Name, BtrfsSnapshotsDirName)
}

// FindSubvolumeByMountPoint finds a subvolume by its mount point.
func FindSubvolumeByMountPoint(subvolumes []BtrfsSubvolume, mountPoint string) *BtrfsSubvolume {
	for i := range subvolumes {
		if subvolumes[i].MountPoint == mountPoint {
			return &subvolumes[i]
		}
	}
	return nil
}

// SubvolumePath returns the full path for a subvolume under the given root.
func SubvolumePath(root, subvolumeName string) string {
	return strings.Join([]string{root, subvolumeName}, "/")
}

// decString renders an int without importing strconv into every caller.
func decString(n int) string {
	return strconv.Itoa(n)
}
