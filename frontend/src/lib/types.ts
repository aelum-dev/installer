export type PageID = 0 | 1 | 2 | 3 | 4 | 5;

export const PAGE_NAMES: Record<number, string> = {
	0: 'Welcome',
	1: 'User Account',
	2: 'Disk Selection',
	3: 'Confirmation',
	4: 'Installation',
	5: 'Finish',
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
