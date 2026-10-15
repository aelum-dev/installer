package shared

// ============================================================================
// Global Configuration Constants
// All tunable values, thresholds, defaults, and string literals live here.
// ============================================================================

// --- Validation thresholds ---------------------------------------------------

// MinUsernameLength is the minimum allowed username length.
const MinUsernameLength = 3

// MaxUsernameLength is the maximum allowed username length (POSIX LOGIN_NAME_MAX).
// Linux limits usernames to 32 characters; useradd will reject longer names.
const MaxUsernameLength = 32

// MinPasswordLength is the minimum allowed password length.
const MinPasswordLength = 6

// ReservedUsernames contains system usernames that cannot be used by users.
// This includes POSIX reserved names, systemd service accounts, and common
// daemon accounts found on Arch Linux systems.
var ReservedUsernames = map[string]bool{
	"root": true, // superuser

	// POSIX reserved names
	"bin":      true,
	"daemon":   true,
	"adm":      true,
	"lp":       true,
	"sync":     true,
	"shutdown": true,
	"halt":     true,
	"mail":     true,
	"news":     true,
	"uucp":     true,
	"operator": true,
	"games":    true,
	"gopher":   true,
	"ftp":      true,
	"nobody":   true,

	// systemd service accounts
	"systemd":           true,
	"systemd-journal":   true,
	"systemd-network":   true,
	"systemd-resolve":   true,
	"systemd-timesync":  true,
	"systemd-bus-proxy": true,
	"systemd-coredump":  true,

	// Common daemon accounts (Arch Linux)
	"dbus":       true,
	"tss":        true,
	"ntp":        true,
	"sshd":       true,
	"polkitd":    true,
	"colord":     true,
	"pulse":      true,
	"rtkit":      true,
	"avahi":      true,
	"cups":       true,
	"usbmux":     true,
	"kernoops":   true,
	"messagebus": true,
	"statd":      true,
	"rpcuser":    true,
	"rpc":        true,
	"nfsnobody":  true,
	"radvd":      true,
	"chrony":     true,
	"qemu":       true,
	"libvirt":    true,
	"smmsp":      true,
	"postfix":    true,
	"www-data":   true,
	"proxy":      true,
	"bind":       true,
	"staff":      true,
	"man":        true,
	"printcap":   true,
	"input":      true,
	"mailnull":   true,
	"list":       true,
	"irc":        true,
	"gnats":      true,
	"backup":     true,

	// Display manager accounts
	"gdm":                 true,
	"gnome-initial-setup": true,
	"gdm3":                true,
	"lightdm":             true,
	"greetd":              true,
	"greeter":             true,
}

// MinDiskSizeGB is the minimum disk size in gigabytes.
const MinDiskSizeGB = 8

// MinDiskSizeBytes is the minimum disk size in bytes (8 GiB).
// Reduced from 16 GiB once the on-disk swap file was removed in favor of zram
// (compressed RAM-based swap). Accounts for: base system (~1.5GB) + desktop
// packages (~4GB) + overhead (~1GB).
const MinDiskSizeBytes = MinDiskSizeGB * 1024 * 1024 * 1024

// MinESPSizeBytes is the smallest size an ESP partition may be (128 MiB). This
// matches the automatic installer's EFI partition and leaves room for firmware,
// Windows Boot Manager, and multiple entries without squeezing the FAT32
// allocation bitmap.
const MinESPSizeBytes = 128 * 1024 * 1024

// MinSystemPartitionBytes is the smallest size the OS partition (Boot/System) may
// be: the same 8 GiB floor as the disk itself. A smaller root cannot hold the
// base + desktop packages, so the manual layout rejects it before install.
// Note: a LUKS2 root loses ~16 MiB to its header before mkfs, so the *usable*
// root floor is marginally lower; that is not adjusted for because it is
// negligible (0.2% of 8 GiB) and the floor is already conservative.
const MinSystemPartitionBytes = MinDiskSizeBytes

// --- Filesystem types --------------------------------------------------------

// DefaultFileSystem is the default root filesystem when none is specified.
const DefaultFileSystem = "btrfs"

// Supported root filesystem types.
const (
	FileSystemExt4  = "ext4"
	FileSystemBtrfs = "btrfs"
	FileSystemVfat  = "vfat"
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

// AppDataDir is the XDG data directory for the installer (used for frontend log path).
const AppDataDir = ".local/share/aelum-installer"

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
const DefaultHostname = "aelum"

// DefaultCLIUsername is the default CLI installer username (test only).
const DefaultCLIUsername = "aelum"

// --- EFI / GPT ---------------------------------------------------------------

// EFIPartitionSizeMB is the EFI partition size in megabytes.
const EFIPartitionSizeMB = 128

// GPTPartitionStartSector is the first usable sector on a GPT disk with 512-byte
// logical sectors (1 MiB). On a 4Kn disk the equivalent boundary is 256 sectors;
// partition.go scales its GPT prefix by the disk's real logical sector size.
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
const BootEntryFileName = "aelum"

// BootEntryTitle is the human-readable title shown in the boot menu.
const BootEntryTitle = "Aelum"

// BootOptionDescription is the custom description passed to bootctl install.
const BootOptionDescription = "Aelum"

// BootLoaderEntriesDir is the subdirectory under the ESP for boot entries.
const BootLoaderEntriesDir = "loader/entries"

// BootLoaderTimeout is the systemd-boot selection timeout (in seconds) written
// to loader.conf, giving the user a window to choose a boot entry at startup.
const BootLoaderTimeout = 0

// --- Progress tick ----------------------------------------------------------

// DryRunStepDuration is how long each install step sleeps in dry-run mode.
// Every step lasts the same fixed duration regardless of its progress weight,
// so the dry-run wizard finishes quickly and predictably.
const DryRunStepDurationMs = 1000

// ProgressTickDelayMs is the initial delay before the progress tick starts.
const ProgressTickDelayMs = 2000

// ProgressTickIntervalMs is the interval between progress increments during
// a long-running step. Each tick advances the percentage by 1.
const ProgressTickIntervalMs = 8000

// ProgressTickMaxPercent is the maximum percentage the tick goroutine will reach.
const ProgressTickMaxPercent = 99

// --- Installation step weights ----------------------------------------------
// Relative weights for progress bar calculation. The two pacstrap steps
// (base + desktop) together account for ~90% of total installation time.

// WeightFormatDisk is the weight for the format disk step.
const WeightFormatDisk = 3

// WeightMountFilesystem is the weight for the mount filesystem step.
const WeightMountFilesystem = 2

// WeightInstallBase is the weight for the base system pacstrap step.
const WeightInstallBase = 37

// WeightInstallDesktop is the weight for the desktop environment pacstrap step.
const WeightInstallDesktop = 50

// WeightSetupUsers is the weight for the user setup step.
const WeightSetupUsers = 2

// WeightConfigureSystem is the weight for the system configuration step.
const WeightConfigureSystem = 4

// WeightClearCache is the weight for the cache clearing step.
const WeightClearCache = 1

// WeightVerifySystem is the weight for the installed-system verification step.
const WeightVerifySystem = 1

// --- Wallpapers -------------------------------------------------------------

// WallpaperSourceDark is the dark wallpaper file path shipped to the installed
// system (used by DMS after install).
const WallpaperSourceDark = "/usr/share/DankMaterialShell/wallpaper-dark.jpg"

// WallpaperSourceLight is the light wallpaper file path shipped to the installed
// system (used by DMS after install).
const WallpaperSourceLight = "/usr/share/DankMaterialShell/wallpaper-light.jpg"

// --- LUKS disk encryption --------------------------------------------------

// LuksMapperName is the dm-crypt mapper device name created for the encrypted
// root partition. The decrypted device appears at /dev/mapper/<LuksMapperName>
// and is what gets formatted, mounted, and referenced by fstab/bootloader.
const LuksMapperName = "aelum"

// LuksMapperPath is the full device path of the decrypted root mapper.
const LuksMapperPath = "/dev/mapper/" + LuksMapperName

// LUKS crypto parameters pinned to current (2026) best practices. See
// luks.md §11 for the verified rationale. These match cryptsetup 2.4.0+
// defaults and the systemd-based initramfs expectations.
const (
	// LuksType selects the LUKS version (luks2 is the current default).
	LuksType = "luks2"
	// LuksCipher is the block cipher/mode for the volume key.
	LuksCipher = "aes-xts-plain64"
	// LuksKeySizeBits is the total LUKS key size. XTS splits it into two AES
	// keys, so 512 = AES-256 (256 would be AES-128).
	LuksKeySizeBits = 512
	// LuksHash is the KDF/workfactor hash.
	LuksHash = "sha256"
	// LuksPBKDF is the password-based key derivation function.
	LuksPBKDF = "argon2id"
	// LuksIterTimeMs is the memory-hard KDF time cost in milliseconds.
	LuksIterTimeMs = 2000
)

// --- Live-ISO backgrounds (hyprpaper) ---------------------------------------
// These are the live-ISO-only backgrounds the installer toggle switches between
// at runtime. They are NOT copied to the installed system — that uses the DMS
// wallpapers above.

// LiveIsoBgLight is the light live-ISO background.
const LiveIsoBgLight = "/usr/share/DankMaterialShell/bg-light.png"

// LiveIsoBgDark is the dark live-ISO background.
const LiveIsoBgDark = "/usr/share/DankMaterialShell/bg-dark.png"

// --- Chroot ------------------------------------------------------------------

// ChrootCommand is the command used to run commands inside the installed system.
const ChrootCommand = "arch-chroot"

// --- Virtual device filters --------------------------------------------------

// VirtualDevicePrefixes are device name prefixes filtered out by GetDisks().
var VirtualDevicePrefixes = []string{"loop", "ram", "dm-", "md"}

// RemovableTransportTypes are transport types that indicate removable media.
var RemovableTransportTypes = []string{"usb", "sd", "mmc"}
