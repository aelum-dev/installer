package shared

// ============================================================================
// Global Configuration Constants
// All tunable values, thresholds, defaults, and string literals live here.
// ============================================================================

// --- Validation thresholds ---------------------------------------------------

// MinUsernameLength is the shortest name useradd accepts (one character). The
// required-name check is what actually blocks an empty username.
const MinUsernameLength = 1

// MaxUsernameLength is the longest name shadow's useradd accepts. Linux
// LOGIN_NAME_MAX is 256 including the NUL terminator, so names may be at most
// 255 characters; useradd rejects a 256-character name.
const MaxUsernameLength = 255

// MinPasswordLength is the shortest non-empty password PAM accepts. Arch's
// default PAM stack has no password-quality policy, so even one character is
// valid (chpasswd accepts it).
const MinPasswordLength = 1

// MaxPasswordLength is the longest password PAM accepts, in UTF-8 bytes.
// pam_unix rejects a PAM response of 512 bytes or larger (PAM_MAX_RESP_SIZE),
// so 511 bytes is the last accepted length.
const MaxPasswordLength = 511

// ReservedAccountNames is the comma-separated set of user and group names that
// exist on a stock Aelum install. This is not a policy list: useradd fails when
// its private group name already exists, and an existing system account would
// otherwise be reused (and have its password changed) instead of creating the
// requested user. A single const keeps the Go backend and the generated
// frontend constants.ts (and therefore the inline validator) in sync.
//
// Source: the sysusers output of a full base + desktop install. users.Run also
// re-checks the actual target's /etc/passwd and /etc/group, so package drift
// is caught even if this list becomes stale.
const ReservedAccountNames = "adm,alpm,audio,bin,clock,daemon,dbus,disk,empower,floppy,ftp,games,greeter,http,input," +
	"kmem,kvm,lock,log,lp,mail,mem,network,nobody,optical,pcscd,polkitd,power,proc,render,rfkill,root," +
	"scanner,sgx,smmsp,storage,sys,systemd-coredump,systemd-imds,systemd-journal,systemd-journal-remote," +
	"systemd-network,systemd-oom,systemd-resolve,systemd-timesync,tss,tty,users,utmp,uucp,uuidd,video,wheel"

// MinDiskSizeGB is the minimum disk size in gigabytes.
const MinDiskSizeGB = 8

// MinDiskSizeBytes is the minimum disk size in bytes (8 GiB).
// Reduced from 16 GiB once the on-disk swap file was removed in favor of zram
// (compressed RAM-based swap). Accounts for: base system (~1.5GB) + desktop
// packages (~4GB) + overhead (~1GB).
const MinDiskSizeBytes = MinDiskSizeGB * 1024 * 1024 * 1024

// MinESPSizeBytes is the smallest size an ESP partition may be (512 MiB). This
// matches the automatic installer's EFI partition and leaves room for firmware,
// Windows Boot Manager, multiple kernels/initrds, and FAT32 allocation metadata
// without squeezing the ESP.
const MinESPSizeBytes = 512 * 1024 * 1024

// MinExistingESPSizeBytes is the smallest *existing* ESP the manual installer
// will reuse (100 MiB). Existing ESPs are commonly 100-300 MiB because Windows
// created them; requiring the new-ESP 512 MiB floor would reject every dual-boot
// machine with a Windows-made ESP. The bootloader step still has to fit the
// kernel and initrd, so a full existing ESP can fail later with a clear error.
const MinExistingESPSizeBytes = 100 * 1024 * 1024

// MinSystemPartitionBytes is the smallest size the OS partition (Boot/System) may
// be: the same 8 GiB floor as the disk itself. A smaller root cannot hold the
// base + desktop packages, so the manual layout rejects it before install.
// Note: a LUKS2 root loses ~16 MiB to its header before mkfs, so the *usable*
// root floor is marginally lower; that is not adjusted for because it is
// negligible (0.2% of 8 GiB) and the floor is already conservative. When LVM is
// enabled the System partition becomes a PV and uses the higher
// MinLvmSystemPartitionBytes floor instead; MinSystemPartitionSize selects it.
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

// EFIPartitionSizeMB is the EFI partition size in mebibytes.
const EFIPartitionSizeMB = 512

// GPTPartitionStartSector is the first usable sector on a GPT disk with 512-byte
// logical sectors (1 MiB). On a 4Kn disk the equivalent boundary is 256 sectors;
// partition.go scales its GPT prefix by the disk's real logical sector size.
const GPTPartitionStartSector = 2048

// GPTPrefixBytes is the leading space a GPT reserves for the protective MBR and
// the main partition table + header: the first usable 1 MiB, which no partition
// may occupy. Expressed in bytes so it scales with the disk's logical sector
// size (2048 sectors at 512 bytes/sector, 256 at 4096). Single source for both
// the partition package and the frontend (via constants.ts).
const GPTPrefixBytes = 1024 * 1024

// GPTBackupBytes is the trailing space a GPT reserves for its backup partition
// table and header. The real default is 33 logical sectors (a 32-entry table
// plus one header), but a generous 128 KiB is reserved so sgdisk never rejects a
// partition whose end sits just before the last usable sector. Expressed in
// bytes so it scales with the logical sector size.
const GPTBackupBytes = 128 * 1024

// MinManualDiskSizeBytes is the smallest disk that can hold the smallest valid
// manual layout: an 8 GiB System partition, an existing 100 MiB ESP, and the
// GPT prefix/backup. A disk below this can pass the automatic 8 GiB disk check
// but can never satisfy the manual partition floors, so manual mode rejects it
// up front with a specific message.
const MinManualDiskSizeBytes = MinSystemPartitionBytes + MinExistingESPSizeBytes + GPTPrefixBytes + GPTBackupBytes

// GPTTypeGUIDs are the GPT partition type GUIDs used by the installer.
var GPTTypeGUIDs = struct {
	ESP                    string
	LinuxRootX86_64        string
	LinuxLVM               string
	LinuxFilesystemGeneric string
}{
	ESP:                    "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
	LinuxRootX86_64:        "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709",
	LinuxLVM:               "E6D6D379-F507-44C2-A23C-238F2A3DF928",
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

// --- Pacstrap progress bands ------------------------------------------------
// Each pacstrap step (base + desktop) reports a step-relative 0-100 percentage
// assembled from ordered phase bands rather than a single unit ratio. The
// download band is deliberately tiny: an offline install copies packages from
// the ISO's embedded mirror in well under a second, so counting those lines
// equally with installs front-loads the bar (downloads would consume half the
// budget in the first second). The install band tops out below 100 so pacman's
// post-transaction hooks — the initramfs rebuild, font and icon caches — always
// have room to move the bar after the last package is extracted, instead of
// being hidden behind an already-full bar.
const (
	// PacstrapDownloadBandEnd is where the package-download phase tops out.
	PacstrapDownloadBandEnd = 5
	// PacstrapInstallBandEnd is where the package-install phase tops out.
	PacstrapInstallBandEnd = 80
)

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

// LuksHeaderReserveBytes is the space a LUKS2 header can consume before the
// decrypted mapper is exposed. The default metadata area is 16 MiB; 32 MiB is
// reserved so the mapper still meets an LVM floor even after alignment slack.
const LuksHeaderReserveBytes = 32 * 1024 * 1024

// --- LVM (logical volume management) ---------------------------------------
// LVM is an opt-in alternative to putting the filesystem directly on the root
// partition: the root partition becomes a physical volume (PV) in a volume group
// (VG), which holds two logical volumes — a root LV and a home LV. Both use the
// selected filesystem. For btrfs the root LV carries every subvolume except
// @home, which lives on its own LV so /home gets its own checksums/compression/
// snapshots; for ext4 the home LV is simply a separate filesystem at /home.

// LvmVolumeGroup is the name of the volume group the installer creates.
const LvmVolumeGroup = "aelum"

// LvmRootVolume is the name of the root logical volume.
const LvmRootVolume = "root"

// LvmHomeVolume is the name of the home logical volume.
const LvmHomeVolume = "home"

// LvmRootPercent is the percentage of the volume group given to the root LV
// before the minimum-size floor is applied.
const LvmRootPercent = 30

// LvmRootMinBytes is the minimum size of the root LV (8 GiB) — the same floor
// as the whole-disk minimum. The root LV is max(LvmRootPercent% of the VG,
// LvmRootMinBytes).
const LvmRootMinBytes = 8 * 1024 * 1024 * 1024

// LvmVGMinBytes is the smallest volume group the installer will create
// (16 GiB): enough for a 8 GiB root LV and a 8 GiB home LV.
const LvmVGMinBytes = 16 * 1024 * 1024 * 1024

// MinLvmSystemPartitionBytes is the System-partition floor when LVM is enabled:
// the partition becomes the PV, so it must be large enough to build the 16 GiB
// volume group the LVM sizing rule requires.
const MinLvmSystemPartitionBytes = LvmVGMinBytes

// MinLvmLuksSystemPartitionBytes is the System-partition floor for LUKS + LVM.
// The mapper is smaller than the raw partition by the LUKS2 header, so the raw
// partition must include the header reserve on top of the LVM floor.
const MinLvmLuksSystemPartitionBytes = LvmVGMinBytes + LuksHeaderReserveBytes

// MinSystemPartitionSize returns the System-partition floor for the chosen
// storage stack. LVM turns the partition into a PV and needs a 16 GiB VG; a
// LUKS2 layer underneath that PV reduces the mapper by the header reserve.
func MinSystemPartitionSize(lvm, luks bool) int64 {
	if !lvm {
		return MinSystemPartitionBytes
	}
	if luks {
		return MinLvmLuksSystemPartitionBytes
	}
	return MinLvmSystemPartitionBytes
}

// LvmRootLvPath is the device-mapper path of the root logical volume.
const LvmRootLvPath = "/dev/mapper/" + LvmVolumeGroup + "-" + LvmRootVolume

// LvmHomeLvPath is the device-mapper path of the home logical volume.
const LvmHomeLvPath = "/dev/mapper/" + LvmVolumeGroup + "-" + LvmHomeVolume

// LvmInitramfsHook is the mkinitcpio hook that activates the volume group in
// the initramfs. It is `lvm2` (shipped by mkinitcpio), not `sd-lvm2`, and goes
// after `block` and before `filesystems`, after `sd-encrypt` when LUKS is on.
const LvmInitramfsHook = "lvm2"

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
