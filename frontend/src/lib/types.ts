export const PAGE_IDS = {
	WELCOME: 0,
	USER_ACCOUNT: 1,
	DISK_SELECTION: 2,
	CONFIRMATION: 3,
	INSTALLATION: 4,
	FINISH: 5,
} as const;

export type PageID = (typeof PAGE_IDS)[keyof typeof PAGE_IDS];

export const TOTAL_PAGES = 6;

export const PAGE_NAMES: Record<PageID, string> = {
	[PAGE_IDS.WELCOME]: 'Welcome',
	[PAGE_IDS.USER_ACCOUNT]: 'User Account',
	[PAGE_IDS.DISK_SELECTION]: 'Disk Selection',
	[PAGE_IDS.CONFIRMATION]: 'Confirmation',
	[PAGE_IDS.INSTALLATION]: 'Installation',
	[PAGE_IDS.FINISH]: 'Finish',
};

// Pages that show no header/footer
export const PAGES_NO_NAV = new Set<PageID>();

// Pages that require validation before advancing
export const PAGES_NEEDING_VALIDATION = new Set<PageID>([PAGE_IDS.USER_ACCOUNT, PAGE_IDS.DISK_SELECTION]);

// MinDiskSizeBytes is the minimum disk size in bytes (16 GiB).
// Mirrors the backend constant in shared/config.go.
// Accounts for: base system (~1.5GB) + desktop packages (~4GB) + swap (4GB) + overhead (~1GB)
export const MIN_DISK_SIZE_BYTES = 16 * 1024 * 1024 * 1024;

// MinUsernameLength is the minimum allowed username length.
// Mirrors the backend constant in shared/config.go.
export const MIN_USERNAME_LENGTH = 3;

// MinPasswordLength is the minimum allowed password length.
// Mirrors the backend constant in shared/config.go.
export const MIN_PASSWORD_LENGTH = 6;

// DiskInfo represents a block device discovered on the system.
export interface DiskInfo {
	name: string;
	path: string;
	size: string;
	type: string;
	model: string;
	vendor: string;
	rotational: boolean;
	readonly: boolean;
	fstype: string | null;
	mountpoint: string | null;
	serial: string;
	trans: string;
	isRemovable: boolean;
}

// Filesystem type options for root partition.
export const FILESYSTEMS = ['ext4', 'btrfs'] as const;
export type FileSystem = (typeof FILESYSTEMS)[number];

export interface DiskSelectionData {
	diskId: string;
	disk: DiskInfo | null;
	fileSystem: FileSystem | '';
}

// PartitionDetail represents a partition with full information for display.
export interface PartitionDetail {
	path: string;
	name: string;
	type: string;
	size: string;
	fstype: string | null;
	uuid: string;
	mountpoint: string | null;
}

// Format bytes to human-readable string.
// NOTE: A Go equivalent exists in backend/internal/shared/disks.go::FormatSize.
// Both serve their respective layers (frontend display vs backend struct tags),
// but the logic is intentionally kept separate to avoid cross-language coupling.
export function formatSize(bytes: string): string {
	const n = parseInt(bytes, 10);
	if (isNaN(n) || n <= 0) return '0 B';

	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	const k = 1024;
	const i = Math.floor(Math.log(n) / Math.log(k));
	const size = n / Math.pow(k, i);

	if (i === 0) return `${Math.round(size)} ${units[0]}`;
	return `${size.toFixed(1)} ${units[i]}`;
}
