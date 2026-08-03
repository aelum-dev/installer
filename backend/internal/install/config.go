package install

// ============================================================================
// Installer Configuration Constants
// All tunable values, command names, paths, and string literals for the
// installation steps live here.
// ============================================================================

// --- Paths -------------------------------------------------------------------

// BootMountPoint is the EFI System Partition (ESP) mount point.
const BootMountPoint = "/mnt/boot"

// Note: MountRoot is defined in shared/config.go

// --- Filesystem types --------------------------------------------------------

// Supported root filesystem types (Vfat is install-specific; Ext4/Btrfs are in shared/config.go).
const (
	FileSystemVfat = "vfat"
)

// --- EFI / GPT ---------------------------------------------------------------
// Note: EFIPartitionSizeMB, GPTPartitionStartSector, and GPTTypeGUIDs
// are defined in shared/config.go — imported via shared.GPTTypeGUIDs etc.

// --- Partition layout --------------------------------------------------------

// PartitionLayoutEFI is the name for the EFI system partition.
const PartitionLayoutEFI = "EFI"

// PartitionLayoutRoot is the name for the root partition.
const PartitionLayoutRoot = "Root"

// --- Helper commands ---------------------------------------------------------

// HelperCommand names as registered in the aestheticos-helper.
const (
	// Chroot commands
	HelperCmdChrootRun   = "chroot-run"
	HelperCmdRunChpasswd = "run-chpasswd"

	// Target filesystem ops
	HelperCmdWriteFile  = "write-file"
	HelperCmdAppendFile = "append-file"
	HelperCmdReadFile   = "read-file"
	HelperCmdSetSymlink = "set-symlink"

	// Bootctl (consolidated from 6 commands)
	HelperCmdBootctl = "bootctl"

	// Generic whitelisted command runner
	HelperCmdRun = "run"

	// Existing commands
	HelperCmdUdevSettle            = "udev-settle"
	HelperCmdBtrfsSubvolCreate     = "btrfs-subvol-create"
	HelperCmdBtrfsSubvolSetDefault = "btrfs-subvol-set-default"
	HelperCmdBtrfsSetCompression   = "btrfs-set-compression"
	HelperCmdBtrfsMkswapfile       = "btrfs-mkswapfile"
	HelperCmdParted                = "parted"
	HelperCmdFormat                = "format"
	HelperCmdMount                 = "mount"
	HelperCmdUmount                = "umount"
)

// --- Sgdisk arguments --------------------------------------------------------

// SgdiskClearFlag clears the partition table (removes GPT data).
const SgdiskClearFlag = "-Z"

// SgdiskCreatePartitionFlag creates a new partition.
const SgdiskCreatePartitionFlag = "-n"

// SgdiskSetTypeFlag sets a partition type GUID.
const SgdiskSetTypeFlag = "-t"

// SgdiskSetNameFlag sets a partition name.
const SgdiskSetNameFlag = "-c"

// --- Wipefs arguments --------------------------------------------------------

// WipefsAllFlag wipes all filesystem signatures.
const WipefsAllFlag = "--all"

// WipefsForceFlag forces overwrite even if "device busy".
const WipefsForceFlag = "-f"

// --- Mkfs arguments ----------------------------------------------------------

// MkfsFatForceFlag forces creation of FAT filesystem.
const MkfsFatForceFlag = "-F"

// MkfsFatTypeFlag sets FAT type (32-bit).
const MkfsFatTypeFlag = "32"

// MkfsFatLabelFlag sets the filesystem label.
const MkfsFatLabelFlag = "-n"

// MkfsFatLabelValue is the default label for the EFI partition.
const MkfsFatLabelValue = "EFI"

// MkfsExt4ForceFlag forces creation of ext4 filesystem.
const MkfsExt4ForceFlag = "-F"

// MkfsExt4LabelFlag sets the filesystem label.
const MkfsExt4LabelFlag = "-L"

// MkfsExt4LabelValue is the default label for the root partition.
const MkfsExt4LabelValue = "aestheticos"

// MkfsBtrfsForceFlag forces creation of btrfs filesystem.
const MkfsBtrfsForceFlag = "-f"

// MkfsBtrfsLabelFlag sets the filesystem label.
// NOTE: Same label as MkfsExt4LabelValue. Both filesystems get labeled "aestheticos".
const MkfsBtrfsLabelValue = "aestheticos"

// --- Umount arguments --------------------------------------------------------

// UmountLazyFlag performs a lazy unmount.
const UmountLazyFlag = "-l"

// UmountRecursiveFlag unmounts recursively.
const UmountRecursiveFlag = "-R"

// --- Pacstrap ----------------------------------------------------------------

// PacstrapMountPoint is the target mount point for pacstrap.
const PacstrapMountPoint = "/mnt"

// --- Time formats ------------------------------------------------------------

// Note: TimestampFormatFull and TimestampFormatShort are defined in shared/config.go

// --- Mirrorlist --------------------------------------------------------------

// Note: MirrorListCommentedServerPrefix and MirrorListServerPrefix are defined in shared/config.go

// --- Mount options -----------------------------------------------------------

// DefaultMountOptions are the standard mount options for the root filesystem.
const DefaultMountOptions = "defaults,noatime"

// EfiPartitionUmask is the umask applied to the EFI partition mount.
const EfiPartitionUmask = "umask=0077"

// BtrfsSubvolumeMountPrefix is prepended to subvolume mount options.
const BtrfsSubvolumeMountPrefix = "subvol="

// BtrfsAutodefragOption enables autodefrag on btrfs subvolume mounts.
const BtrfsAutodefragOption = "autodefrag"

// --- File permissions --------------------------------------------------------

// Note: SudoersFileMode and MinFstabContentLengthBytes are defined in shared/config.go

// --- Chroot commands ---------------------------------------------------------

// ChrootCommand is the command used to run commands inside the installed system.
// arch-chroot auto-mounts /proc, /sys, /dev (from arch-install-scripts on the live ISO).
const ChrootCommand = "arch-chroot"

// --- Bootctl arguments -------------------------------------------------------

// BootctlInstallFlag is the flag for installing systemd-boot.
const BootctlInstallFlag = "install"

// BootctlGracefulFlag enables graceful mode for bootctl.
const BootctlGracefulFlag = "--graceful"

// BootctlEspPathFlag sets the ESP path for bootctl.
const BootctlEspPathFlag = "--esp-path="

// BootctlCleanupFlag runs the ESP cleanup.
const BootctlCleanupFlag = "cleanup"

// BootctlRandomSeedFlag generates a random seed.
const BootctlRandomSeedFlag = "random-seed"

// BootctlSetDefaultFlag sets the default boot entry.
const BootctlSetDefaultFlag = "set-default"

// BootctlIsInstalledFlag checks if systemd-boot is installed.
const BootctlIsInstalledFlag = "is-installed"

// BootctlStatusFlag shows bootctl status.
const BootctlStatusFlag = "status"

// --- Kernel / boot entry -----------------------------------------------------

// KernelFilePrefix is the filename prefix for vmlinuz kernel files.
const KernelFilePrefix = "vmlinuz-"

// InitrdFilePrefix is the filename prefix for initramfs images.
const InitrdFilePrefix = "initramfs-"

// InitrdFileExtension is the file extension for initramfs images.
const InitrdFileExtension = ".img"

// KernelPresetName is the mkinitcpio preset used for regeneration.
const KernelPresetName = "linux-lts"

// BootEntryFileName is the name of the systemd-boot entry file (without .conf).
// NOTE: This is hardcoded. If the user customizes their installation name,
// the boot entry file and menu entries will still say "aestheticos".
const BootEntryFileName = "aestheticos"

// BootEntryTitle is the human-readable title shown in the boot menu.
const BootEntryTitle = "Aesthetic OS"

// BootOptionDescription is the custom description passed to bootctl install.
// NOTE: This is hardcoded. Matches BootEntryTitle by convention.
const BootOptionDescription = "Aesthetic OS"

// BootLoaderEntriesDir is the subdirectory under the ESP for boot entries.
const BootLoaderEntriesDir = "loader/entries"

// --- Heredoc delimiters ------------------------------------------------------

// FstabHeredocDelimiter is the heredoc delimiter for writing fstab.
const FstabHeredocDelimiter = "EOF"

// BootEntryHeredocDelimiter is the heredoc delimiter for writing boot entries.
const BootEntryHeredocDelimiter = "BOOTEOF"

// --- Btrfs subvolume setup ---------------------------------------------------

// BtrfsTopLevelSubvolumeID is the subvolume ID for the btrfs top-level.
const BtrfsTopLevelSubvolumeID = 5

// BtrfsSetupTmpMount is the temporary mount point for btrfs subvolume operations.
const BtrfsSetupTmpMount = "/tmp/btrfs-setup"

// --- Logging -----------------------------------------------------------------

// Note: LogFileMode is defined in shared/config.go.

// --- Simulate ---------------------------------------------------------------

// SimulateStepSleepDuration is how long simulateStep sleeps in dry-run mode (milliseconds).
// Short enough to feel responsive during testing, long enough to be visible in logs.
const SimulateStepSleepDuration = 200

// --- DMS Greeter -------------------------------------------------------------

// DmsWallpaperSource is the wallpaper file path on the live ISO.
const DmsWallpaperSource = "/usr/share/DankMaterialShell/wallpaper.jpg"

// DmsWallpaperDest is the destination path for the wallpaper inside the
// installed system's DMS configuration directory.
const DmsWallpaperDest = "/mnt/usr/share/DankMaterialShell/wallpaper.jpg"

// --- Environment file -------------------------------------------------------

// EnvironmentFileSource is the path to the environment file on the live ISO.
const EnvironmentFileSource = "/etc/environment"

// --- Progress tick ----------------------------------------------------------

// ProgressTickDelayMs is the initial delay before the progress tick starts.
// Fast steps (format, mount) finish before the first tick fires, avoiding
// unnecessary UI flicker.
const ProgressTickDelayMs = 2000

// ProgressTickIntervalMs is the interval between progress increments during
// a long-running step. Each tick advances the percentage by 1.
// Set higher (7s) so the tick animation better matches pacstrap's actual
// duration — base pacstrap (45 ticks) takes ~5min, desktop (35 ticks) ~4min.
const ProgressTickIntervalMs = 8000

// ProgressTickMaxPercent is the maximum percentage the tick goroutine will
// reach. The step's final emit always overrides this value.
const ProgressTickMaxPercent = 99
