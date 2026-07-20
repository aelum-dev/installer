package steps

import "aesthetic-os-installer/backend/shared"

// InstallContext holds all data needed by installation steps.
// PartitionInfo is populated by FormatDisk and consumed by subsequent steps.
type InstallContext struct {
	DiskID     string
	ESP        string // ESP mount point, e.g. "/mnt/boot"
	Partitions *shared.PartitionInfo
	Helper     string
	Username   string
	Password   string
	Locale     string
}
