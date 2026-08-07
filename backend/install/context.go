package install

import "aestheticos-installer/backend/shared"

// InstallContext holds all data needed by installation steps.
// PartitionInfo is populated by FormatDisk and consumed by subsequent steps.
type InstallContext struct {
	DiskID     string
	ESP        string // ESP mount point, e.g. "/mnt/boot"
	Partitions *shared.PartitionInfo
	Helper     string
	Username   string
	Password   []byte // zeroed after use; never logged or returned
	Hostname   string
	Region     shared.Region
	FileSystem string // root filesystem: "ext4" or "btrfs"
	Logger     Logger // injected by Installer; nil if not set

	// ProgressCallback reports intermediate progress during a step.
	// Called by the installer's tick goroutine to animate long-running steps.
	// nil when no progress reporting is needed (e.g. CLI without UI).
	ProgressCallback func(percent int)

	// Cancel is closed when SIGINT is received during a long-running command.
	// Used to unblock scanner goroutines (e.g. in installPackages) so they
	// return promptly instead of waiting for the child process to finish.
	Cancel chan struct{}
}
