package shared

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
	LayoutName  string            // Name of the layout used
	EFIPart     string            // e.g. "/dev/sda1"
	EFIUUID     string            // UUID of the EFI partition
	RootPart    string            // e.g. "/dev/sda2"
	RootUUID    string            // UUID of the root partition
	MountPoints map[string]string // Additional mount points (e.g., "/home" → "/dev/sda2")
}

// DiskInfo represents a block device discovered on the system.
type DiskInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         string `json:"size"`
	Type         string `json:"type"`
	Model        string `json:"model"`
	Vendor       string `json:"vendor"`
	Rotational   bool   `json:"rotational"`
	Readonly     bool   `json:"ro"`
	Fstype       string `json:"fstype"`
	Mountpoint   string `json:"mountpoint"`
	Serial       string `json:"serial"`
	Trans        string `json:"trans"`
	IsRemovable  bool   `json:"isRemovable"`
}
