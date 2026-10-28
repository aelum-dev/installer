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

// MinDiskSizeBytes is the minimum whole-disk size (8 GiB). The root/system
// partition floor is derived by subtracting the ESP from this total, so the
// product requirement and the manual layout agree: 8192 MiB - 512 MiB = 7680 MiB
// for root. It is reduced from the old 16 GiB target after the on-disk swap file
// was removed in favor of zram.
const MinDiskSizeBytes = 8 * 1024 * 1024 * 1024

// MinESPSizeBytes is the smallest size an ESP partition may be (512 MiB).
// Every Boot partition is formatted, new or existing, and must leave room for
// the bootloader, theme, kernel, initramfs, and FAT32 allocation metadata
// without squeezing the ESP.
const MinESPSizeBytes = 512 * 1024 * 1024

// MinSystemPartitionBytes is the root/system partition floor: 7680 MiB, i.e.
// the 8 GiB disk minimum minus the 512 MiB ESP. A smaller root cannot hold the
// base + desktop packages, so the manual layout rejects it before install. A
// LUKS2 root loses ~16 MiB to its header before mkfs; that is negligible (0.2%
// of the floor) and the floor is already conservative.
const MinSystemPartitionBytes = MinDiskSizeBytes - MinESPSizeBytes

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

// BootMountPoint is where the installed system's /boot lives during install.
// It is a bind mount of this installation's directory on its ESP, so its kernel
// and initramfs stay isolated from any other install.
const BootMountPoint = "/mnt/boot"

// ESPMountPoint is where the EFI System Partition itself is mounted during
// install. rEFInd and the per-install boot directories live here.
const ESPMountPoint = "/mnt/efi"

// InstalledESPMountPoint is where the installed system mounts the ESP. /boot is
// a bind mount of InstalledESPMountPoint/<EspInstallDirName>/<id>.
const InstalledESPMountPoint = "/efi"

// EspInstallDirName is the directory at the ESP root that holds one boot
// directory per Aelum installation (kernel + initramfs).
const EspInstallDirName = "aelum"

// LogDir is the directory that holds the installation log.
const LogDir = "/var/log"

// LogMirrorDir is an optional shared folder (the QEMU host share). When it
// exists, the installer writes a second copy of the log there.
const LogMirrorDir = "/shared"

// LogFileNamePrefix is the installation log filename prefix.
const LogFileNamePrefix = "aelum-install-"

// LogFileTimeLayout is the timestamp appended to the log filename.
const LogFileTimeLayout = "20060102-150405"

// LogPathEnv is the environment variable the GUI uses to hand the CLI the same
// log path it will display on failure.
const LogPathEnv = "AELUM_LOG_PATH"

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

// PartitionTableGPT is the partition-table name util-linux reports for a GPT
// disk (blkid/lsblk PTTYPE). util-linux calls an MBR table "dos". Manual
// partitioning compares against it: sgdisk only writes to a GPT.
const PartitionTableGPT = "gpt"

// EFIPartitionSizeMB is the EFI partition size in mebibytes.
const EFIPartitionSizeMB = 512

// GPTPartitionStartSector is the first usable sector on a GPT disk with 512-byte
// logical sectors (1 MiB). On a 4Kn disk the equivalent boundary is 256 sectors;
// partition.go scales its GPT prefix by the disk's real logical sector size.
const GPTPartitionStartSector = 2048

// MinManualDiskSizeBytes is the overall disk floor for manual mode: the same
// 8 GiB product minimum. The individual ESP/System floors are checked
// separately; a 512 MiB ESP + 7680 MiB System needs a little more than 8 GiB
// once the GPT prefix/backup is counted.
const MinManualDiskSizeBytes = MinDiskSizeBytes

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

// BtrfsSnapshotsDirName is the directory snapper keeps its snapshots in. It is a
// subvolume of its own, not a directory: snapper refuses a plain one, and as a
// subvolume it stays out of the snapshots of the parent it belongs to.
const BtrfsSnapshotsDirName = ".snapshots"

// BtrfsSnapshotSubvolumeName is the name snapper gives the subvolume holding one
// snapshot, inside .snapshots/<number>/.
const BtrfsSnapshotSubvolumeName = "snapshot"

// BtrfsInitialSnapshotNumber is the number of the snapshot the system is installed
// into. snapper's rollback names the running system by the filesystem's default
// subvolume and only accepts it as one of its own numbered snapshots
// (Btrfs::getDefault -> idToNum requires a path ending <number>/snapshot), so the
// installer creates that snapshot, points the default subvolume at it, and installs
// into it - the sequence snapper's own installation-helper runs (step1, and the
// step_filesystem fallback that creates .snapshots/1/snapshot and sets it as the
// default before anything is installed).
const BtrfsInitialSnapshotNumber = 1

// BtrfsSnapshotsMountPoint is where the installed system mounts the root config's
// .snapshots subvolume. That mount is what keeps the history reachable when / is a
// snapshot: a snapshot never contains the subvolumes that were inside its parent,
// so without it the restored system has an empty .snapshots and snapper stops
// working in it.
const BtrfsSnapshotsMountPoint = "/.snapshots"

// --- Kernel / boot entry -----------------------------------------------------

// KernelFilePrefix is the filename prefix for vmlinuz kernel files.
const KernelFilePrefix = "vmlinuz-"

// InitrdFilePrefix is the filename prefix for initramfs images.
const InitrdFilePrefix = "initramfs-"

// InitrdFileExtension is the file extension for initramfs images.
const InitrdFileExtension = ".img"

// KernelPresetName is the mkinitcpio preset used for regeneration.
const KernelPresetName = "linux-lts"

// BootEntryTitle is the human-readable title shown in the boot menu.
const BootEntryTitle = "Aelum"

// RefindDir is rEFInd's installation directory on the ESP, relative to the ESP
// root (the ESP is mounted at ESPMountPoint during install).
const RefindDir = "EFI/refind"

// RefindBinaryName is the x86_64 rEFInd EFI binary shipped by the refind
// package.
const RefindBinaryName = "refind_x64.efi"

// RefindBinaryPath is the ESP-relative path of the installed rEFInd binary.
const RefindBinaryPath = RefindDir + "/" + RefindBinaryName

// RefindConfigFileName is the name of rEFInd's configuration file.
const RefindConfigFileName = "refind.conf"

// RefindConfigPath is the ESP-relative path of rEFInd's configuration file.
const RefindConfigPath = RefindDir + "/" + RefindConfigFileName

// RefindEntriesDirName is the directory under EFI/refind that holds one menu
// fragment per Aelum installation. Keeping each installation's menuentry in
// its own file means installing Aelum a second time on the same ESP adds an
// entry instead of overwriting the first one's configuration.
const RefindEntriesDirName = "aelum"

// RefindEntriesDir is the ESP-relative directory of the per-install menu
// fragments.
const RefindEntriesDir = RefindDir + "/" + RefindEntriesDirName

// RefindSourceDir is the read-only asset directory shipped by the refind
// package inside the live ISO. The whole tree (binary, icons, drivers, tools)
// is copied onto the ESP, so no network access is needed during install.
const RefindSourceDir = "/usr/share/refind"

// RefindNvramLoader is the firmware loader path registered in NVRAM. UEFI
// firmware expects backslash-separated paths relative to the ESP root.
const RefindNvramLoader = "\\EFI\\refind\\refind_x64.efi"

// RefindTimeoutSeconds is how long the rEFInd menu waits before booting the
// default entry, giving the user a window to pick another OS (dualboot).
const RefindTimeoutSeconds = 5

// RefindThemeName is the directory name of the branded Aelum boot-menu theme.
const RefindThemeName = "aelum"

// RefindThemeDir is the ESP-relative directory of the branded rEFInd theme.
const RefindThemeDir = RefindDir + "/themes/" + RefindThemeName

// RefindThemeSourceDir is the read-only theme directory shipped inside the
// live ISO (/usr/share/aelum/...). ConfigureBootloader copies it onto the ESP,
// so the branded menu needs no network access.
const RefindThemeSourceDir = "/usr/share/aelum/refind/themes/" + RefindThemeName

// RefindThemeConfigFileName is the name of the theme's option file.
const RefindThemeConfigFileName = "theme.conf"

// RefindThemeConfigPath is the ESP-relative path of the theme's option file.
const RefindThemeConfigPath = RefindThemeDir + "/" + RefindThemeConfigFileName

// RefindThemeInclude is the path refind.conf passes to the "include" token.
// rEFInd resolves included files relative to its own directory (EFI/refind),
// so this deliberately omits the RefindDir prefix.
const RefindThemeInclude = "themes/" + RefindThemeName + "/" + RefindThemeConfigFileName

// RefindThemeFont2x is the 2x font sheet, relative to EFI/refind. rEFInd does
// not scale custom fonts, so the installer selects this sheet on HiDPI
// displays (the same threshold at which rEFInd doubles icon sizes).
const RefindThemeFont2x = "themes/" + RefindThemeName + "/font-2x.png"

// RefindThemeBackgroundPath is the ESP-relative full-screen boot background
// referenced by theme.conf.
const RefindThemeBackgroundPath = RefindThemeDir + "/background.png"

// RefindIconPath is the ESP-relative Aelum mark shown for Aelum's own menu
// entries. It lives in the branded theme so the menu icon matches the boot
// background; rEFInd selects the sibling os_*.png icons (systemd-boot, Windows,
// Ventoy, generic Linux) for loaders it auto-detects.
const RefindIconPath = RefindThemeDir + "/icons/os_arch.png"

// RefindSnapshotIconPath is the ESP-relative icon for an installation's snapshot
// menu entry: the history glyph of the branded theme, drawn on the same cream
// tile as the Aelum mark so it reads as part of this system rather than as a
// second copy of it. live-iso/scripts/gen-refind-theme.py renders it.
const RefindSnapshotIconPath = RefindThemeDir + "/icons/os_snapshot.png"

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

// --- LVM volume management ---------------------------------------------------

// The volume group name is the install id derived from the PV's PARTUUID, built
// by install/lvm.VolumeGroupName: 8 lowercase hex characters with no hyphen, on
// purpose, so a logical volume's device-mapper name has a single separator
// ("1a2b3c4d-root"). There is no "aelum-" prefix — LVM doubles every hyphen
// inside both names, and a prefixed group read as "aelum--1a2b3c4d-root".

// LvmRootVolumeName is the logical volume mounted as "/".
const LvmRootVolumeName = "root"

// LvmHomeVolumeName is the logical volume mounted as "/home". It is created for
// ext4 only: btrfs keeps /home on the @home subvolume of the single root LV.
const LvmHomeVolumeName = "home"

// LvmRootPercent is the root LV's share of the volume group (ext4 layouts
// only), clamped to the root floor and to leaving room for home.
const LvmRootPercent = 30

// LvmInitramfsHook is the mkinitcpio hook that activates the volume group in the
// initramfs. It must follow sd-encrypt (LUKS sits below LVM) and precede
// filesystems.
const LvmInitramfsHook = "lvm2"

// MinLvmVolumeGroupBytes is the smallest usable volume group (16 GiB). With the
// 8 GiB root floor that leaves room for an equally sized home LV on ext4, and
// keeps one uniform floor for both filesystems.
const MinLvmVolumeGroupBytes = 16 * 1024 * 1024 * 1024

// LvmMetadataReserveBytes covers the PV header (~1 MiB), the VG metadata and
// the extent alignment that lvcreate rounds up to.
const LvmMetadataReserveBytes = 16 * 1024 * 1024

// LuksHeaderReserveBytes is the space a LUKS2 header takes before the PV starts.
// The header is 16 MiB; doubling it leaves margin for header growth.
const LuksHeaderReserveBytes = 32 * 1024 * 1024

// MinLvmRootVolumeBytes is the floor for the root LV. It reuses the plain-root
// partition floor so an LVM install and a non-LVM install are rejected below the
// same size.
const MinLvmRootVolumeBytes = MinSystemPartitionBytes

// MinLvmSystemPartitionBytes is the System-partition floor when LVM is enabled:
// the smallest usable VG plus the LVM metadata reserve.
const MinLvmSystemPartitionBytes = MinLvmVolumeGroupBytes + LvmMetadataReserveBytes

// MinLvmLuksSystemPartitionBytes is the System-partition floor when LVM sits on
// top of LUKS: the LVM floor plus the encrypted header.
const MinLvmLuksSystemPartitionBytes = MinLvmSystemPartitionBytes + LuksHeaderReserveBytes

// SystemPartitionFloorBytes returns the System-partition size floor for a manual
// layout. LVM needs a whole volume group, and LUKS adds its header on top of it;
// a plain root keeps the (smaller) base floor.
func SystemPartitionFloorBytes(lvm, luks bool) int64 {
	switch {
	case lvm && luks:
		return MinLvmLuksSystemPartitionBytes
	case lvm:
		return MinLvmSystemPartitionBytes
	default:
		return MinSystemPartitionBytes
	}
}

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
