package shared

// PartitionLayout defines the partition scheme for a disk.
// Each layout specifies the number, size, filesystem, and purpose of partitions.
// For btrfs layouts, Subvolumes defines the subvolume scheme (empty for ext4).
type PartitionLayout struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Partitions  []LayoutPartition `json:"partitions"`
	Subvolumes  []BtrfsSubvol     `json:"subvolumes,omitempty"`
}

// BtrfsSubvol defines a single btrfs subvolume: its name, mount point, and properties.
// The subvolume with MountPoint "/" becomes the default subvolume.
type BtrfsSubvol struct {
	Name       string            `json:"name"`        // Subvolume name (e.g. "@", "@home", "@var")
	MountPoint string            `json:"mountPoint"`  // Mount point in installed system (e.g. "/", "/home", "/var")
	Properties map[string]string `json:"properties"`  // Property → value ("compression" → "zstd", "nodatacow" → "")
}

// LayoutPartition defines a single partition in a layout.
type LayoutPartition struct {
	Name       string `json:"name"`       // Human-readable name (e.g., "EFI", "Root")
	SizeMB     int64  `json:"sizeMb"`     // Size in MB (0 = rest of disk)
	Fstype     string `json:"fstype"`     // Filesystem type (e.g., "vfat", "ext4", "btrfs")
	MountPoint string `json:"mountPoint"` // Mount point in installed system (e.g., "/boot", "/", "/home")
	Options    string `json:"options"`    // Additional mkfs options (e.g., "-f" for btrfs)
}

// GetDefaultLayout returns the standard 2-partition layout (EFI + root).
// Works with both ext4 and btrfs.
func GetDefaultLayout(rootFstype string) PartitionLayout {
	return PartitionLayout{
		Name:        "ext4",
		Description: "Standard layout: EFI system partition + root partition (ext4)",
		Partitions: []LayoutPartition{
			{
				Name:       "EFI",
				SizeMB:     128,
				Fstype:     "vfat",
				MountPoint: "/boot",
			},
			{
				Name:       "Root",
				SizeMB:     0, // Rest of disk
				Fstype:     rootFstype,
				MountPoint: "/",
			},
		},
	}
}

// GetBtrfsFullLayout returns a btrfs layout with subvolumes for root, home, root user, srv, cache, tmp, and log.
func GetBtrfsFullLayout() PartitionLayout {
	return PartitionLayout{
		Name:        "btrfs",
		Description: "BTRFS with subvolumes: @, @home, @root, @srv, @cache, @tmp, @log",
		Partitions: []LayoutPartition{
			{
				Name:       "EFI",
				SizeMB:     128,
				Fstype:     "vfat",
				MountPoint: "/boot",
			},
			{
				Name:       "Root",
				SizeMB:     0,
				Fstype:     "btrfs",
				MountPoint: "/",
			},
		},
		Subvolumes: []BtrfsSubvol{
			{Name: "@", MountPoint: "/", Properties: map[string]string{"compression": "zstd"}},
			{Name: "@home", MountPoint: "/home", Properties: map[string]string{"compression": "zstd", "nodatacow": ""}},
			{Name: "@root", MountPoint: "/root", Properties: map[string]string{"compression": "zstd"}},
			{Name: "@srv", MountPoint: "/srv", Properties: map[string]string{"compression": "zstd"}},
			{Name: "@cache", MountPoint: "/var/cache", Properties: map[string]string{"compression": "zstd", "nodatacow": ""}},
			{Name: "@tmp", MountPoint: "/var/tmp", Properties: map[string]string{"compression": "zstd", "nodatacow": ""}},
			{Name: "@log", MountPoint: "/var/log", Properties: map[string]string{"nodatacow": ""}},
		},
	}
}

// GetLayout returns a partition layout by name and root filesystem type.
func GetLayout(layoutName, rootFstype string) (PartitionLayout, error) {
	switch layoutName {
	case "ext4":
		return GetDefaultLayout(rootFstype), nil
	case "btrfs":
		if rootFstype != "btrfs" {
			return PartitionLayout{}, &LayoutError{
				Layout: "btrfs",
				Reason: "requires btrfs filesystem",
			}
		}
		return GetBtrfsFullLayout(), nil
	default:
		return PartitionLayout{}, &LayoutError{
			Layout: layoutName,
			Reason: "unknown layout",
		}
	}
}

// LayoutError represents an error with layout selection.
type LayoutError struct {
	Layout string
	Reason string
}

func (e *LayoutError) Error() string {
	return "layout error: " + e.Layout + " - " + e.Reason
}
