// Page IDs for the installer wizard.
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

// Filesystem type options for root partition.
export const FILESYSTEMS = ['ext4', 'btrfs'] as const;
export type FileSystem = (typeof FILESYSTEMS)[number];

// DiskInfo represents a block device discovered on the system.
export interface DiskInfo {
	name: string;
	path: string;
	size: string;            // raw byte count as string
	sizeFormatted: string;   // human-readable (e.g. "476.9 GiB")
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

// PartitionDetail represents a partition with full information for display.
export interface PartitionDetail {
	path: string;
	name: string;
	type: string;
	size: string;            // raw byte count as string
	sizeFormatted: string;   // human-readable (e.g. "476.9 GiB")
	fstype: string | null;
	uuid: string;
	mountpoints: string[];
}
