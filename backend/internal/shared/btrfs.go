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
	{Name: "@srv", MountPoint: "/srv", Compression: "zstd", NoDataCow: false},
	{Name: "@cache", MountPoint: "/var/cache", Compression: "zstd", NoDataCow: true},
	{Name: "@tmp", MountPoint: "/var/tmp", Compression: "zstd", NoDataCow: true},
	{Name: "@log", MountPoint: "/var/log", Compression: "", NoDataCow: true},
	{Name: "@swap", MountPoint: "/swap", Compression: "", NoDataCow: true},
}

// GetBtrfsSubvolumes returns the standard btrfs subvolume scheme.
// Used by format_disk.go and generate_fstab.go to ensure consistency.
func GetBtrfsSubvolumes() []BtrfsSubvolume {
	return btrfsSubvolumes
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
