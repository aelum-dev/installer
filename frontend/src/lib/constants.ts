// AUTO-GENERATED — do not edit. Run go run ./backend/scripts/gen-ts-constants.go to regenerate.
// Source: backend/internal/shared/config.go

export const MinUsernameLength = 3;
export const MinPasswordLength = 6;
export const MinDiskSizeBytes = (((16 * 1024) * 1024) * 1024);
export const DefaultFileSystem = "ext4";
export const FileSystemExt4 = "ext4";
export const FileSystemBtrfs = "btrfs";
export const DefaultLanguageCode = "en_US";
export const DefaultBtrfsCompression = "zstd";
export const TimestampFormatFull = "2006-01-02 15:04:05";
export const TimestampFormatShort = "15:04:05";
export const MountRoot = "/mnt";
export const LogFileName = "installation.log";
export const LogFileMode = 0o644;
export const SudoersFileMode = 0o440;
export const MirrorListDirMode = 0o755;
export const MirrorListURL = "https://archlinux.org/mirrorlist/all/https/";
export const RawMirrorlistFileName = "all_https.txt";
export const MirrorListsRelativeDir = "mirrorlist";
export const MirrorListCountryHeaderPrefix = "## ";
export const MirrorListCommentedServerPrefix = "#Server = ";
export const MirrorListServerPrefix = "Server = ";
export const MinFstabContentLengthBytes = 50;
export const DefaultHostname = "aestheticos";
export const DefaultCLIUsername = "aesthetic";
export const EFIPartitionSizeMB = 128;
export const GPTPartitionStartSector = 2048;
