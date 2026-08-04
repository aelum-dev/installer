package shared

// ============================================================================
// Configuration Constants
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
	FileSystemExt4  = "ext4"
	FileSystemBtrfs = "btrfs"
)

// --- Language / region -------------------------------------------------------

// DefaultLanguageCode is the fallback language when none is selected.
const DefaultLanguageCode = "en_US"

// --- Btrfs subvolumes --------------------------------------------------------

// DefaultBtrfsCompression is the compression algorithm for btrfs subvolumes.
const DefaultBtrfsCompression = "zstd"

// --- Mount options -----------------------------------------------------------
// Note: Mount option constants used by installer steps are in install/config.go

// --- Time formats ------------------------------------------------------------

// TimestampFormatFull is the full timestamp format used in log files.
const TimestampFormatFull = "2006-01-02 15:04:05"

// TimestampFormatShort is the short timestamp format used in console output.
const TimestampFormatShort = "15:04:05"

// --- Path separators ---------------------------------------------------------

// MountRoot is the root mount point for the installed system.
const MountRoot = "/mnt"

// LogFileName is the default installation log filename.
const LogFileName = "installation.log"

// --- File permissions --------------------------------------------------------

// LogFileMode is the permission mode for the installation log file.
const LogFileMode = 0644

// SudoersFileMode is the permission mode for sudoers drop-in files.
const SudoersFileMode = 0440

// MirrorListDirMode is the permission mode for mirrorlist directories.
const MirrorListDirMode = 0755

// --- Mirrorlist --------------------------------------------------------------
// Note: Mirrorlist string constants used by installer steps are in install/config.go

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
// A minimal fstab with one root entry is ~80 bytes; 50 is a lenient lower bound
// to catch truly empty or corrupted output without rejecting valid fstabs.
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
	LinuxFilesystemGeneric: "8300", // GPT type code for Linux filesystem (standard)
}

// --- Virtual device filters --------------------------------------------------

// VirtualDevicePrefixes are device name prefixes filtered out by GetDisks().
var VirtualDevicePrefixes = []string{"loop", "ram", "dm-", "md"}

// RemovableTransportTypes are transport types that indicate removable media.
var RemovableTransportTypes = []string{"usb", "sd", "mmc"}
