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

// lvmBtrfsSubvolumes is the subvolume set for an LVM root LV: the canonical
// scheme minus @home, which lives on the separate home LV instead.
var lvmBtrfsSubvolumes = []BtrfsSubvolume{
	{Name: "@", MountPoint: "/", Compression: "zstd", NoDataCow: false},
	{Name: "@srv", MountPoint: "/srv", Compression: "zstd", NoDataCow: false},
	{Name: "@cache", MountPoint: "/var/cache", Compression: "", NoDataCow: true},
	{Name: "@tmp", MountPoint: "/var/tmp", Compression: "", NoDataCow: true},
	{Name: "@log", MountPoint: "/var/log", Compression: "zstd", NoDataCow: false},
}

// lvmHomeSubvolumes is the subvolume set for the LVM home LV: a single @home.
var lvmHomeSubvolumes = []BtrfsSubvolume{
	{Name: "@home", MountPoint: "/home", Compression: "zstd", NoDataCow: false},
}

// GetLvmBtrfsSubvolumes returns the subvolume scheme for the LVM root LV: the
// canonical set minus @home (which the home LV carries).
func GetLvmBtrfsSubvolumes() []BtrfsSubvolume {
	return lvmBtrfsSubvolumes
}

// GetLvmHomeSubvolumes returns the subvolume scheme for the LVM home LV (@home).
func GetLvmHomeSubvolumes() []BtrfsSubvolume {
	return lvmHomeSubvolumes
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
