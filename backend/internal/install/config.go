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
