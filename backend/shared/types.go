package shared

// InstallContext holds all data needed by installation steps.
// PartitionInfo is populated by FormatDisk and consumed by subsequent steps.
type InstallContext struct {
	DiskID     string
	ESP        string // ESP mount point, e.g. "/mnt/efi"
	Partitions *PartitionInfo
	Helper     string
	Username   string

	// ManualMode indicates the disk was laid out manually: the designated Boot
	// and System partitions are formatted, and every other partition is left
	// untouched. When set, ManualLayout describes the plan and partition.Apply
	// handles formatting instead of format.Run.
	ManualMode   bool
	ManualLayout *ManualLayout
	Password     []byte // zeroed after use; never logged or returned

	// Luks reports whether the root partition is encrypted with LUKS2. When set,
	// the format step creates a decrypted mapper device that the mount/install/
	// configure steps operate on instead of the raw partition.
	Luks bool
	// LuksPassphrase is the LUKS passphrase, fed to cryptsetup via stdin. Zeroed
	// after use; never logged or returned.
	LuksPassphrase []byte

	// Lvm reports whether the System partition (or its LUKS mapper) becomes an
	// LVM physical volume holding the root logical volume. LUKS always sits
	// below LVM: at most one encrypted PV, one passphrase.
	Lvm bool

	Hostname string
	// Locale is the installed system's locale, e.g. "en_US.UTF-8". It comes from
	// the language the person chose, never from the region: what a machine's
	// mirrors and clock should be is a separate question from what language it
	// speaks.
	Locale     string
	Region     Region // location: timezone, console keymap, mirrors
	FileSystem string // root filesystem: "ext4" or "btrfs"
	Logger     Logger // injected by Installer; nil if not set

	// GPUDriver is the --gpu-driver override: "auto" (or empty — detect from
	// the PCI bus) or one fixed vendor set ("amd", "intel", "nvidia",
	// "mesa"). It is the escape hatch for hardware the vendor table cannot
	// classify; the GUI never sets it.
	GPUDriver string

	// DriverPackages is the GPU driver set chosen for this install: recorded
	// by the desktop step (install/installdesktop/drivers.go) from the
	// detected hardware or the override, and reused by the verify step. It is
	// never empty once the desktop step has run — every set provides both the
	// vulkan-driver and lib32-vulkan-driver virtuals, which is what keeps
	// pacman's provider question from ever appearing on an installed system.
	DriverPackages []string

	// InputMethod is the --input-method override: "auto" (or empty — follow
	// the language table) or "none" (no input method addons for any language).
	// It is the escape hatch the QEMU matrix and manual testing use; the GUI
	// never sets it.
	InputMethod string

	// IMPackages is the input method addon set chosen for this install:
	// recorded by the desktop step (install/installdesktop/inputmethods.go)
	// from the chosen language, and reused by the verify step. The Fcitx5 core
	// stack is not here — it is part of DesktopPackages and installed on every
	// install. An empty (non-nil) set is a real outcome: most languages are
	// typed through keyboard layouts and need no engine addon.
	IMPackages []string

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
	Size       int64  `json:"size"`       // byte size of selected disk
	FileSystem string `json:"fileSystem"` // root filesystem: "ext4" or "btrfs"
}

// PartitionInfo holds the partition layout computed after formatting.
// Populated by FormatDisk and consumed by MountFilesystem, ConfigureBootloader, etc.
type PartitionInfo struct {
	Disk     string // e.g. "/dev/sda"
	EFIPart  string // e.g. "/dev/sda1"
	EFIUUID  string // UUID of the EFI partition
	RootPart string // e.g. "/dev/sda2" (raw partition, before LUKS)
	RootUUID string // UUID of the root filesystem (mapper FS UUID when LUKS)
	// RootMapper is the decrypted dm-crypt device path (e.g. /dev/mapper/aelum).
	// Empty when encryption is disabled; otherwise the device mounted as root.
	RootMapper string
	// LuksUUID is the LUKS header UUID of the raw root partition. Used for the
	// boot cmdline rd.luks.name= argument — NOT the filesystem UUID.
	LuksUUID string
	// VGName is the volume group holding the root LV. Empty when LVM is off.
	// Nothing in fstab or the boot entry references it: the LV filesystem UUID
	// is what identifies the root, so renaming the VG never breaks boot.
	VGName string
	// HomeDevice is the ext4 home LV's device path (e.g. /dev/1a2b3c4d/home).
	// Empty for btrfs (which keeps /home on the @home subvolume) and for
	// non-LVM installs.
	HomeDevice string
	// HomeUUID is the filesystem UUID of HomeDevice. Empty unless a separate
	// home LV exists.
	HomeUUID    string
	MountPoints map[string]string // Additional mount points (e.g., "/home" → "/dev/sda2")
}

// DiskInfo represents a block device discovered on the system.
type DiskInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Size          int64  `json:"size"`          // raw byte count
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
	// PartitionTable is the disk's partition-table type as util-linux names it
	// ("gpt", "dos", …), or "" when the disk has no partition table or the type
	// could not be read. Manual mode needs a GPT; automatic mode writes one.
	PartitionTable string `json:"partitionTable"`
}

// PartitionType is the type assigned to a partition in a manual layout plan.
type PartitionType string

const (
	// TypeESP formats the partition as vfat (ESP) and mounts it at /efi during
	// install. It is always formatted, whether the row is new or existing.
	TypeESP PartitionType = "esp"
	// TypeSystem formats the partition as the root filesystem (ext4/btrfs) and
	// mounts it at /.
	TypeSystem PartitionType = "system"
	// An empty type means the partition is untouched — not formatted, not
	// mounted, not in fstab. This is what keeps other OSes and data safe.
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
type ManualLayout struct {
	Disk       string `json:"disk"`
	DiskSizeMB int64  `json:"diskSizeMB"`
	// Luks is a validation hint: true when the System partition is encrypted.
	// Apply reconciles it from ctx.Luks so the two can never disagree.
	Luks bool `json:"luks,omitempty"`
	// Lvm is a validation hint: true when the System partition becomes an LVM
	// physical volume. Apply reconciles it from ctx.Lvm the same way.
	Lvm bool `json:"lvm,omitempty"`
	// DiskSizeBytes is the exact total disk size in bytes. Free-space math that
	// must survive existing-partitions being rounded down to MiB uses it.
	DiskSizeBytes int64 `json:"diskSizeBytes"`
	// SectorSizeBytes is the disk's logical sector size in bytes (512 for most
	// disks, 4096 for 4Kn). sgdisk numbers sectors in this unit, so all sector
	// math must use it. Zero means "unknown" and is treated as 512.
	SectorSizeBytes int64             `json:"sectorSizeBytes,omitempty"`
	Partitions      []ManualPartition `json:"partitions"`
	// DeletedPartitions holds partitions the user removed from the layout. They
	// are kept (rather than dropped) so Apply can delete them from the disk with
	// `sgdisk -d`. The frontend moves a row here when the user clicks delete.
	DeletedPartitions []ManualPartition `json:"deletedPartitions,omitempty"`
}
