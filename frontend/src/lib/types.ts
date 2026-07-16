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

// Page-specific error messages for validation failures
export const PAGE_ERROR_MESSAGES: Record<PageID, string> = {
	[PAGE_IDS.WELCOME]: 'Please select a language to continue.',
	[PAGE_IDS.USER_ACCOUNT]: 'Please complete all required fields to continue.',
	[PAGE_IDS.DISK_SELECTION]: 'Please select a disk to continue.',
	[PAGE_IDS.CONFIRMATION]: 'Please review your installation details.',
	[PAGE_IDS.INSTALLATION]: 'Please wait for the installation to complete.',
	[PAGE_IDS.FINISH]: 'The installation is complete.',
};

// Transition direction for animations.
export type TransitionDirection = 'forward' | 'backward';

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
}

// Format bytes to human-readable string.
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

// Get a display label for the disk.
export function diskLabel(disk: DiskInfo): string {
	const model = disk.model || 'Unknown';
	const size = formatSize(disk.size);
	const indicator = disk.rotational ? 'HDD' : 'SSD';
	return `${model} (${size}, ${indicator})`;
}
