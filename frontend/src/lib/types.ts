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
export const FILESYSTEMS = ['btrfs', 'ext4'] as const;
export type FileSystem = (typeof FILESYSTEMS)[number];

// Manual-partitioning types (dualboot-safe). Mirrors backend shared.PartitionType.
export const TYPE_ESP = 'esp';
export const TYPE_SYSTEM = 'system';
// An empty type means the partition is untouched (dualboot-safe).
export type PartitionType = 'esp' | 'system' | '';

export const PARTITION_TYPES: PartitionType[] = [TYPE_ESP, TYPE_SYSTEM, ''];

// ManualPartition mirrors backend shared.ManualPartition — one row in a manual
// layout plan.
export interface ManualPartition {
	index: number;
	path: string; // existing device (e.g. "/dev/sda1"), "" for newly created
	sizeMB: number; // 0 = consume all remaining free space
	// sizeBytes is the exact byte size of an existing partition, used for
	// byte-exact free-space accounting. 0 for brand-new partitions.
	sizeBytes?: number;
	type: PartitionType;
	existingFstype?: string;
}

// ManualLayout mirrors backend shared.ManualLayout — the user-designed plan for
// one disk. freeMB is derived, never stored.
export interface ManualLayout {
	disk: string;
	diskSizeMB: number;
	// diskSizeBytes is the exact total disk size in bytes.
	diskSizeBytes?: number;
	// sectorSizeBytes is the disk's logical sector size in bytes (512 for most
	// disks, 4096 for 4Kn). Sector math uses it; 0/unset means 512.
	sectorSizeBytes?: number;
	// freeSectors is the exact unallocated space, in the disk's logical sectors.
	freeSectors?: number;
	// lvm places root and home on LVM logical volumes. The backend reconciles
	// this with the --lvm toggle, so the frontend need not set it.
	lvm?: boolean;
	partitions: ManualPartition[];
	freeMB: number;
	// Partitions the user removed from the layout. Kept (not dropped) so the
	// backend can delete them from the disk at install time.
	deletedPartitions?: ManualPartition[];
}

// DiskInfo represents a block device discovered on the system.
export interface DiskInfo {
	name: string;
	path: string;
	size: number;            // raw byte count
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

// PartitionEntry represents a partition on a disk.
// Used by both the backend (format discovery) and frontend (display).
export interface PartitionEntry {
	path: string;
	name: string;
	type: string;
	size: number;            // raw byte count
	fstype: string | null;
	uuid: string;
	mountpoints: string[]; // expanded for btrfs, single-element slice otherwise
}
