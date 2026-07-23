package shared

import "strings"

// BtrfsSubvolume represents a btrfs subvolume with its mount point and properties.
type BtrfsSubvolume struct {
	Name        string
	MountPoint  string
	Compression string
	NoDataCow   bool
}

// btrfsSubvolumes is the canonical btrfs subvolume scheme.
// Used by format_disk.go and generate_fstab.go to ensure consistency.
var btrfsSubvolumes = []BtrfsSubvolume{
	{Name: "@", MountPoint: "/", Compression: "zstd", NoDataCow: false},
	{Name: "@home", MountPoint: "/home", Compression: "zstd", NoDataCow: true},
	{Name: "@root", MountPoint: "/root", Compression: "zstd", NoDataCow: false},
	{Name: "@srv", MountPoint: "/srv", Compression: "zstd", NoDataCow: false},
	{Name: "@cache", MountPoint: "/var/cache", Compression: "zstd", NoDataCow: true},
	{Name: "@tmp", MountPoint: "/var/tmp", Compression: "zstd", NoDataCow: true},
	{Name: "@log", MountPoint: "/var/log", Compression: "", NoDataCow: true},
}

// GetBtrfsSubvolumes returns the standard btrfs subvolume scheme.
// Used by format_disk.go and generate_fstab.go to ensure consistency.
// Returns a copy to prevent external mutation of the canonical list.
func GetBtrfsSubvolumes() []BtrfsSubvolume {
	result := make([]BtrfsSubvolume, len(btrfsSubvolumes))
	copy(result, btrfsSubvolumes)
	return result
}

// GetSubvolumeMountPoints returns all mount points for the given subvolumes.
func GetSubvolumeMountPoints(subvolumes []BtrfsSubvolume) []string {
	mounts := make([]string, len(subvolumes))
	for i, sv := range subvolumes {
		mounts[i] = sv.MountPoint
	}
	return mounts
}

// HasSubvolumeWithMountPoint checks if any subvolume has the given mount point.
func HasSubvolumeWithMountPoint(subvolumes []BtrfsSubvolume, mountPoint string) bool {
	for _, sv := range subvolumes {
		if sv.MountPoint == mountPoint {
			return true
		}
	}
	return false
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
