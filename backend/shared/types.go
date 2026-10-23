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

	// Lvm reports whether the root partition becomes an LVM physical volume in
	// the `aelum` volume group (with root + home logical volumes). When set, the
	// root filesystem lives on a logical volume and the initramfs gains the
	// lvm2 hook. Composes with Luks: the PV is the decrypted LUKS mapper.
	Lvm bool
	// LvmVolumes describes the logical volumes to create when Lvm is set (root,
	// then home). Never serialized; the sizing is fixed by the lvm package.
	LvmVolumes []LvmVolume

	Hostname   string
	Region     Region
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
	// RootLV is the LVM root logical volume path (e.g. /dev/mapper/aelum-root).
	// Empty when LVM is disabled; otherwise the device mounted as root.
	RootLV string
	// HomeLV is the LVM home logical volume path (e.g. /dev/mapper/aelum-home).
	// Empty when LVM is disabled; mounted at /home.
	HomeLV string
	// HomeUUID is the filesystem UUID of the home logical volume (LVM only).
	HomeUUID    string
	MountPoints map[string]string // Additional mount points (e.g., "/home" → "/dev/sda2")
}

// LvmVolume describes one logical volume the installer creates in the volume
// group. The root volume is sized from a percentage of the VG (with a floor);
// the home volume takes the rest.
type LvmVolume struct {
	Name       string // "root" | "home"
	Percent    int    // root: percentage of the VG (ignored when Rest is true)
	Rest       bool   // home: consume 100% of the remaining space
	FileSystem string // "btrfs" | "ext4"
	MountPoint string // "/" | "/home"
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
	// Lvm enables LVM on the designated System partition: the System partition
	// becomes a PV in the `aelum` VG with root + home LVs.
	Lvm bool `json:"lvm,omitempty"`
	// Luks is a validation hint: true when the System partition is encrypted.
	// Apply reconciles it from ctx.Luks; the GUI/CLI set it before validation so
	// the LUKS header reserve is included in the System-partition floor.
	Luks bool `json:"luks,omitempty"`
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
