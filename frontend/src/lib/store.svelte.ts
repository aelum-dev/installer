import type {
	DiskInfo,
	FileSystem,
	ManualLayout,
	ManualPartition,
	PartitionRole,
} from '$lib/types';
import { ROLE_BOOT, ROLE_ROOT } from '$lib/types';
import { GPTOverheadMB } from '$lib/constants';

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
	fileSystem: '' as FileSystem | '',

	// Manual (dualboot-safe) partitioning mode
	manualMode: false as boolean,
	manualLayout: null as ManualLayout | null,

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
				return hasRequiredRoles(installState.manualLayout)
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

// computeFreeMB derives the remaining unallocated space (in MB) from the disk
// total minus the sum of all partition sizes minus ~1 MiB GPT overhead. It is
// never stored, so it stays correct after every edit.
export function computeFreeMB(layout: ManualLayout): number {
	const used = layout.partitions.reduce((sum, p) => sum + Math.max(p.sizeMB, 0), 0);
	return Math.max(layout.diskSizeMB - used - GPTOverheadMB, 0);
}

// roleCount returns how many partitions carry the given role.
export function roleCount(layout: ManualLayout | null, role: PartitionRole): number {
	if (!layout) return 0;
	return layout.partitions.filter((p) => p.role === role).length;
}

// hasRequiredRoles is true when the layout has exactly one Boot and one Root.
export function hasRequiredRoles(layout: ManualLayout | null): boolean {
	if (!layout) return false;
	return roleCount(layout, ROLE_BOOT) === 1 && roleCount(layout, ROLE_ROOT) === 1;
}

// designate sets a partition's role.
export function setPartitionRole(
	partitions: ManualPartition[],
	index: number,
	role: PartitionRole,
): void {
	const part = partitions.find((p) => p.index === index);
	if (part) part.role = role;
}
