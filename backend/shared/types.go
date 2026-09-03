package shared

// InstallContext holds all data needed by installation steps.
// PartitionInfo is populated by FormatDisk and consumed by subsequent steps.
type InstallContext struct {
	DiskID     string
	ESP        string // ESP mount point, e.g. "/mnt/boot"
	Partitions *PartitionInfo
	Helper     string
	Username   string

	// ManualMode indicates the disk was laid out manually (dualboot-safe):
	// only the Boot and Root partitions are formatted/mounted; every other
	// partition is left untouched. When set, ManualLayout describes the plan
	// and partition.Apply handles formatting instead of format.Run.
	ManualMode   bool
	ManualLayout *ManualLayout
	Password     []byte // zeroed after use; never logged or returned
	Hostname     string
	Region       Region
	FileSystem   string // root filesystem: "ext4" or "btrfs"
	Logger       Logger // injected by Installer; nil if not set

	// ProgressCallback reports intermediate progress during a step.
	// Called by the installer's tick goroutine to animate long-running steps.
	// nil when no progress reporting is needed (e.g. CLI without UI).
	ProgressCallback func(percent int)

	// Cancel is closed when SIGINT is received during a long-running command.
	// Used to unblock scanner goroutines (e.g. in installPackages) so they
	// return promptly instead of waiting for the child process to finish.
	Cancel chan struct{}
}

// Language represents a selectable language.
type Language struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

// UserAccountData holds user account form fields.
type UserAccountData struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// DiskSelectionData holds disk selection info.
type DiskSelectionData struct {
	DiskID     string `json:"diskId"`
	Size       string `json:"size"`       // byte size of selected disk
	FileSystem string `json:"fileSystem"` // root filesystem: "ext4" or "btrfs"
}

// PartitionInfo holds the partition layout computed after formatting.
// Populated by FormatDisk and consumed by MountFilesystem, ConfigureBootloader, etc.
type PartitionInfo struct {
	Disk        string            // e.g. "/dev/sda"
	EFIPart     string            // e.g. "/dev/sda1"
	EFIUUID     string            // UUID of the EFI partition
	RootPart    string            // e.g. "/dev/sda2"
	RootUUID    string            // UUID of the root partition
	MountPoints map[string]string // Additional mount points (e.g., "/home" → "/dev/sda2")
}

// DiskInfo represents a block device discovered on the system.
type DiskInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Size          string `json:"size"`          // raw byte count as string
	SizeFormatted string `json:"sizeFormatted"` // human-readable (e.g. "476.9 GiB")
	Type          string `json:"type"`
	Model         string `json:"model"`
	Vendor        string `json:"vendor"`
	Rotational    bool   `json:"rotational"`
	Readonly      bool   `json:"readonly"`
	Fstype        string `json:"fstype"`
	Mountpoint    string `json:"mountpoint"`
	Serial        string `json:"serial"`
	Trans         string `json:"trans"`
	IsRemovable   bool   `json:"isRemovable"`
}

// PartitionType is the type assigned to a partition in a manual layout plan.
type PartitionType string

const (
	// TypeESP formats the partition as vfat (ESP) and mounts it at /boot.
	TypeESP PartitionType = "esp"
	// TypeSystem formats the partition as the root filesystem (ext4/btrfs) and
	// mounts it at /.
	TypeSystem PartitionType = "system"
	// An empty type means the partition is untouched — not formatted, not
	// mounted, not in fstab. This is what makes dualboot safe.
)

// ManualPartition is one row in a manual partition layout plan.
type ManualPartition struct {
	Index  int           `json:"index"`
	Path   string        `json:"path"`   // existing device (e.g. "/dev/sda1"), "" for newly created
	SizeMB int64         `json:"sizeMB"` // new-partition size in MiB; 0 = consume all remaining free space
	Type   PartitionType `json:"type"`   // esp | system | empty = untouched
	// SizeBytes is the exact byte size of an existing partition (Path set). It is
	// used for byte-exact free-space accounting so rounding existing partitions
	// down to MiB can't overstate how much room a brand-new partition may take.
	// Zero for brand-new partitions, whose size is an exact whole number of MiB.
	SizeBytes      int64  `json:"sizeBytes"`
	ExistingFstype string `json:"existingFstype,omitempty"`
}

// ManualLayout is the user-designed manual partition plan for one disk.
// FreeMB is always recomputed from the disk total, never stored, so it stays
// correct after every edit.
type ManualLayout struct {
	Disk       string `json:"disk"`
	DiskSizeMB int64  `json:"diskSizeMB"`
	// DiskSizeBytes is the exact total disk size in bytes. Free-space math that
	// must survive existing-partitions being rounded down to MiB uses it.
	DiskSizeBytes int64             `json:"diskSizeBytes"`
	Partitions    []ManualPartition `json:"partitions"`
	FreeMB        int64             `json:"freeMB"`
	// FreeSectors is the exact unallocated space in 512-byte sectors, derived
	// from the raw byte totals so it never suffers MiB truncation.
	FreeSectors int64 `json:"freeSectors"`
	// DeletedPartitions holds partitions the user removed from the layout. They
	// are kept (rather than dropped) so Apply can delete them from the disk with
	// `sgdisk -d`. The frontend moves a row here when the user clicks delete.
	DeletedPartitions []ManualPartition `json:"deletedPartitions,omitempty"`
}
