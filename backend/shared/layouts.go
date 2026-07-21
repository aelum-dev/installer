package shared

// PartitionLayout defines the partition scheme for a disk.
// Each layout specifies the number, size, filesystem, and purpose of partitions.
type PartitionLayout struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Partitions  []LayoutPartition `json:"partitions"`
}

// LayoutPartition defines a single partition in a layout.
type LayoutPartition struct {
	Name     string `json:"name"`     // Human-readable name (e.g., "EFI", "Root")
	SizeMB   int64  `json:"sizeMb"`   // Size in MB (0 = rest of disk)
	Fstype   string `json:"fstype"`   // Filesystem type (e.g., "vfat", "ext4", "btrfs")
	MountPoint string `json:"mountPoint"` // Mount point in installed system (e.g., "/boot", "/", "/home")
	Options  string `json:"options"`  // Additional mkfs options (e.g., "-f" for btrfs)
}

// GetDefaultLayout returns the standard 2-partition layout (EFI + root).
// Works with both ext4 and btrfs.
func GetDefaultLayout(rootFstype string) PartitionLayout {
	return PartitionLayout{
		Name:        "default",
		Description: "Standard layout: EFI system partition + root partition",
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

// GetBtrfsSubvolumeLayout returns a btrfs layout with subvolumes (@ and @home).
// The root partition is formatted as btrfs, then subvolumes are created.
func GetBtrfsSubvolumeLayout() PartitionLayout {
	return PartitionLayout{
		Name:        "btrfs-subvolumes",
		Description: "BTRFS with subvolumes: @ for root, @home for /home (enables snapshots)",
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
				Fstype:     "btrfs",
				MountPoint: "/",
				Options:    "-f", // Force format (btrfs refuses non-empty devices without -f)
			},
		},
		// Note: Subvolumes (@ and @home) are created after formatting, not as separate partitions
	}
}

// GetLayout returns a partition layout by name and root filesystem type.
func GetLayout(layoutName, rootFstype string) (PartitionLayout, error) {
	switch layoutName {
	case "default":
		return GetDefaultLayout(rootFstype), nil
	case "btrfs-subvolumes":
		if rootFstype != "btrfs" {
			return PartitionLayout{}, &LayoutError{
				Layout: "btrfs-subvolumes",
				Reason: "requires btrfs filesystem",
			}
		}
		return GetBtrfsSubvolumeLayout(), nil
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
