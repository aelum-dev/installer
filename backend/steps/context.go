package steps

import "aesthetic-os-installer/backend/shared"

// Logger is the shared logging interface used by installation steps.
// Both CLI and GUI provide implementations that write to the console.
type Logger interface {
	// Info logs an informational message
	Info(format string, args ...interface{})
	// Error logs an error message
	Error(format string, args ...interface{})
	// Command logs a line of command output
	Command(line string)
}

// InstallContext holds all data needed by installation steps.
// PartitionInfo is populated by FormatDisk and consumed by subsequent steps.
type InstallContext struct {
	DiskID      string
	ESP         string // ESP mount point, e.g. "/mnt/boot"
	Partitions  *shared.PartitionInfo
	Layout      shared.PartitionLayout // full layout schema (partitions + subvolumes)
	Helper      string
	Username    string
	Password    string
	Hostname    string
	Region      shared.Region
	FileSystem  string // root filesystem: "ext4" or "btrfs"
	LayoutName  string // partition layout name (e.g., "ext4", "btrfs")
	Logger      Logger // injected by Installer; nil if not set
}
