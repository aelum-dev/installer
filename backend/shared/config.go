package shared

// ============================================================================
// Global Configuration Constants
// All tunable values, thresholds, defaults, and string literals live here.
// Change a value in one place — every consumer picks it up automatically.
// ============================================================================

// --- Validation thresholds ---------------------------------------------------

// MinUsernameLength is the minimum allowed username length.
const MinUsernameLength = 3

// MinPasswordLength is the minimum allowed password length.
const MinPasswordLength = 6

// MinDiskSizeBytes is the minimum disk size in bytes (16 GiB).
// Accounts for: base system (~1.5GB) + desktop packages (~4GB) + swap (dynamic, up to 16GB) + overhead (~1GB).
const MinDiskSizeBytes = 16 * 1024 * 1024 * 1024

// --- Filesystem types --------------------------------------------------------

// DefaultFileSystem is the default root filesystem when none is specified.
const DefaultFileSystem = "ext4"

// Supported root filesystem types.
const (
	FileSystemExt4   = "ext4"
	FileSystemBtrfs  = "btrfs"
	FileSystemVfat   = "vfat"
)

// --- Language / region -------------------------------------------------------

// DefaultLanguageCode is the fallback language when none is selected.
const DefaultLanguageCode = "en_US"

// --- Btrfs subvolumes --------------------------------------------------------

// DefaultBtrfsCompression is the compression algorithm for btrfs subvolumes.
const DefaultBtrfsCompression = "zstd"

// --- Paths -------------------------------------------------------------------

// MountRoot is the root mount point for the installed system.
const MountRoot = "/mnt"

// BootMountPoint is the EFI System Partition (ESP) mount point.
const BootMountPoint = "/mnt/boot"

// LogFileName is the default installation log filename.
const LogFileName = "installation.log"

// --- Time formats ------------------------------------------------------------

// TimestampFormatFull is the full timestamp format used in log files.
const TimestampFormatFull = "2006-01-02 15:04:05"

// TimestampFormatShort is the short timestamp format used in console output.
const TimestampFormatShort = "15:04:05"

// --- File permissions --------------------------------------------------------

// LogFileMode is the permission mode for the installation log file.
const LogFileMode = 0644

// SudoersFileMode is the permission mode for sudoers drop-in files.
const SudoersFileMode = 0440

// MirrorListDirMode is the permission mode for mirrorlist directories.
const MirrorListDirMode = 0755

// --- Mirrorlist --------------------------------------------------------------

// MirrorListURL is the Arch Linux mirrorlist download URL.
const MirrorListURL = "https://archlinux.org/mirrorlist/all/https/"

// RawMirrorlistFileName is the filename for the raw downloaded mirrorlist.
const RawMirrorlistFileName = "all_https.txt"

// MirrorListsRelativeDir is the relative directory for per-region mirrorlist files.
const MirrorListsRelativeDir = "mirrorlist"

// MirrorListCountryHeaderPrefix is the prefix for country sections in the mirrorlist.
const MirrorListCountryHeaderPrefix = "## "

// MirrorListCommentedServerPrefix is the prefix for a commented-out mirror server line.
const MirrorListCommentedServerPrefix = "#Server = "

// MirrorListServerPrefix is the prefix for an active mirror server line.
const MirrorListServerPrefix = "Server = "

// --- Fstab -------------------------------------------------------------------

// MinFstabContentLengthBytes is the minimum expected fstab content length.
const MinFstabContentLengthBytes = 50

// --- Region defaults ---------------------------------------------------------

// DefaultHostname is the default system hostname.
const DefaultHostname = "aestheticos"

// DefaultCLIUsername is the default CLI installer username (test only).
const DefaultCLIUsername = "aesthetic"

// --- EFI / GPT ---------------------------------------------------------------

// EFIPartitionSizeMB is the EFI partition size in megabytes.
const EFIPartitionSizeMB = 128

// GPTPartitionStartSector is the first usable sector on a GPT disk (1 MiB).
const GPTPartitionStartSector = 2048

// GPTTypeGUIDs are the GPT partition type GUIDs used by the installer.
var GPTTypeGUIDs = struct {
	ESP                    string
	LinuxRootX86_64        string
	LinuxFilesystemGeneric string
}{
	ESP:                    "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
	LinuxRootX86_64:        "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709",
	LinuxFilesystemGeneric: "8300",
}

// --- Partition layout --------------------------------------------------------

// PartitionLayoutEFI is the name for the EFI system partition.
const PartitionLayoutEFI = "EFI"

// PartitionLayoutRoot is the name for the root partition.
const PartitionLayoutRoot = "Root"

// --- Mount options -----------------------------------------------------------

// DefaultMountOptions are the standard mount options for the root filesystem.
const DefaultMountOptions = "defaults,noatime"

// EfiPartitionUmask is the umask applied to the EFI partition mount.
const EfiPartitionUmask = "umask=0077"

// BtrfsSubvolumeMountPrefix is prepended to subvolume mount options.
const BtrfsSubvolumeMountPrefix = "subvol="

// BtrfsAutodefragOption enables autodefrag on btrfs subvolume mounts.
const BtrfsAutodefragOption = "autodefrag"

// --- Btrfs subvolume setup ---------------------------------------------------

// BtrfsTopLevelSubvolumeID is the subvolume ID for the btrfs top-level.
const BtrfsTopLevelSubvolumeID = 5

// BtrfsSetupTmpMount is the temporary mount point for btrfs subvolume operations.
const BtrfsSetupTmpMount = "/tmp/btrfs-setup"

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
const BootEntryFileName = "aestheticos"

// BootEntryTitle is the human-readable title shown in the boot menu.
const BootEntryTitle = "Aesthetic OS"

// BootOptionDescription is the custom description passed to bootctl install.
const BootOptionDescription = "Aesthetic OS"

// BootLoaderEntriesDir is the subdirectory under the ESP for boot entries.
const BootLoaderEntriesDir = "loader/entries"

// --- Simulate ---------------------------------------------------------------

// SimulateStepSleepDuration is how long simulateStep sleeps in dry-run mode (ms).
const SimulateStepSleepDuration = 200

// --- Progress tick ----------------------------------------------------------

// ProgressTickDelayMs is the initial delay before the progress tick starts.
const ProgressTickDelayMs = 2000

// ProgressTickIntervalMs is the interval between progress increments during
// a long-running step. Each tick advances the percentage by 1.
const ProgressTickIntervalMs = 8000

// ProgressTickMaxPercent is the maximum percentage the tick goroutine will reach.
const ProgressTickMaxPercent = 99

// --- DMS Greeter -------------------------------------------------------------

// DmsWallpaperSource is the wallpaper file path on the live ISO.
const DmsWallpaperSource = "/usr/share/DankMaterialShell/wallpaper.jpg"

// DmsWallpaperDest is the destination path for the wallpaper inside the
// installed system's DMS configuration directory.
const DmsWallpaperDest = "/mnt/usr/share/DankMaterialShell/wallpaper.jpg"

// --- Environment file -------------------------------------------------------

// EnvironmentFileSource is the path to the environment file on the live ISO.
const EnvironmentFileSource = "/etc/environment"

// --- Chroot ------------------------------------------------------------------

// ChrootCommand is the command used to run commands inside the installed system.
const ChrootCommand = "arch-chroot"

// --- Virtual device filters --------------------------------------------------

// VirtualDevicePrefixes are device name prefixes filtered out by GetDisks().
var VirtualDevicePrefixes = []string{"loop", "ram", "dm-", "md"}

// RemovableTransportTypes are transport types that indicate removable media.
var RemovableTransportTypes = []string{"usb", "sd", "mmc"}
