import type {
	DiskInfo,
	FileSystem,
	ManualLayout,
	ManualPartition,
	PartitionType,
} from '$lib/types';
import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
// ── Installation State (shared across pages) ──

export const installState = $state({
	// Language selection
	locale: '',

	// User account
	username: '',
	password: '',
	confirmPassword: '',

	// Disk selection
	diskId: '',
	disk: null as DiskInfo | null,
	fileSystem: 'ext4' as FileSystem | '',

	// Manual (dualboot-safe) partitioning mode
	manualMode: false as boolean,
	manualLayout: null as ManualLayout | null,

	// Pending "add partition" size (MB) not yet committed to the layout. Set live
	// by ManualPartitionPage as the user drags the size slider, so the top-bar
	// space bar can preview the change before the partition is actually added.
	pendingAddMB: 0 as number,

	// Confirmation dialog
	showConfirmDialog: false,

	// Dry-run mode (set by backend on startup)
	isDryRun: false,

	// Scroll positions (persisted across page navigation, -1 = not saved yet)
	welcomeScrollY: -1,
	diskScrollY: -1,
});

// ── Navigation State ──

export const navState = $state({
	currentPage: 0,
	direction: 'forward' as 'forward' | 'backward',
	isInstalling: false,
	isComplete: false,
});

// ── Derived Page Validity ──

export function isPageValid(page: number): boolean {
	switch (page) {
		case 0: // Welcome
			return !!installState.locale;
		case 1: // User Account
			return (
				installState.username.trim().length >= 3 &&
				installState.password.length >= 6 &&
				installState.confirmPassword === installState.password &&
				installState.confirmPassword.length > 0
			);
		case 2: // Disk Selection
			if (installState.manualMode) {
				return hasRequiredTypes(installState.manualLayout)
			}
			return !!installState.diskId && !!installState.fileSystem;
		case 3: // Confirmation
			return true; // Validated by previous pages
		case 4: // Installation
			return false; // Disable navigation during installation
		case 5: // Finish
			return true;
		default:
			return false;
	}
}

// ── Actions ──

export function setDryRun(value: boolean): void {
	installState.isDryRun = value;
}

export function getIsDryRun(): boolean {
	return installState.isDryRun;
}

export function setShowConfirmDialog(value: boolean): void {
	installState.showConfirmDialog = value;
}

// ── Manual partition helpers ───────────────────────────────────────────────

// computeUsedMB sums the space that will actually be committed to disk. An
// existing partition (path set) always occupies its space. A brand-new
// partition (path empty) only occupies space once it is designated ESP or
// System — the backend never creates or formats an undesignated (Preserve) row,
// so a blank new partition must not shrink the shown free space. This is never
// stored, so it stays correct after every edit.
export function computeUsedMB(layout: ManualLayout): number {
	return layout.partitions.reduce((sum, p) => {
		const reserved = p.path !== '' || p.type !== '';
		return sum + (reserved ? Math.max(p.sizeMB, 0) : 0);
	}, 0);
}

// computeFreeMB derives the remaining unallocated space (in MB) from the disk
// total minus the committed space.
export function computeFreeMB(layout: ManualLayout): number {
	return Math.max(layout.diskSizeMB - computeUsedMB(layout), 0);
}

// typeCount returns how many partitions carry the given type.
export function typeCount(layout: ManualLayout | null, type: PartitionType): number {
	if (!layout) return 0;
	return layout.partitions.filter((p) => p.type === type).length;
}

// hasRequiredTypes is true when the layout has exactly one ESP and one System.
export function hasRequiredTypes(layout: ManualLayout | null): boolean {
	if (!layout) return false;
	return typeCount(layout, TYPE_ESP) === 1 && typeCount(layout, TYPE_SYSTEM) === 1;
}

// setType sets a partition's type.
export function setPartitionType(
	partitions: ManualPartition[],
	index: number,
	type: PartitionType,
): void {
	const part = partitions.find((p) => p.index === index);
	if (part) part.type = type;
}
