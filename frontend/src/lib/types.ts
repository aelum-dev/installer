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
// one disk.
export interface ManualLayout {
	disk: string;
	diskSizeMB: number;
	// diskSizeBytes is the exact total disk size in bytes.
	diskSizeBytes?: number;
	// sectorSizeBytes is the disk's logical sector size in bytes (512 for most
	// disks, 4096 for 4Kn). Sector math uses it; 0/unset means 512.
	sectorSizeBytes?: number;
	// lvm places root and home on LVM logical volumes. The backend reconciles
	// this with the --lvm toggle, so the frontend need not set it.
	lvm?: boolean;
	// luks is a validation hint: true when root is encrypted. The frontend
	// passes the current toggle to manualLayoutIssues so the LUKS header reserve
	// is included in the System-partition floor.
	luks?: boolean;
	partitions: ManualPartition[];
	// Partitions the user removed from the layout. Kept (not dropped) so the
	// backend can delete them from the disk at install time.
	deletedPartitions?: ManualPartition[];
}

// LanguageOption is one entry in the language picker, mirrored from the backend
// GetLanguages() binding.
export interface LanguageOption {
	code: string; // backend locale code, e.g. "en_US"
	name: string; // display name, e.g. "English (US)"
	locale: string; // full backend locale, e.g. "en_US.UTF-8"
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
