import type {
	DiskInfo,
	FileSystem,
	ManualLayout,
	ManualPartition,
	PartitionType,
} from '$lib/types';
import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
import { MinDiskSizeBytes, MinESPSizeBytes } from '$lib/constants';
import { t } from '$lib/i18n';
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
				return isManualLayoutValid(installState.manualLayout)
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

// bytesPerMB is one MiB in bytes.
const bytesPerMB = 1024 * 1024;

// computeUsedMB sums the space the plan has reserved, in MiB. Every row in the
// layout counts — an existing partition (path set) always occupies its space,
// and a brand-new partition counts as soon as it is added, so the space bar
// reflects the addition immediately even before the user designates it
// ESP/System. The backend still ignores undesignated (Preserve) rows when
// creating partitions; this is a display/derivation helper only (never stored),
// so it stays correct after every edit.
export function computeUsedMB(layout: ManualLayout): number {
	return layout.partitions.reduce((sum, p) => sum + Math.max(p.sizeMB, 0), 0);
}

// computeUsedBytes sums the space the plan has reserved, in exact bytes:
// existing partitions by their real byte size, brand-new partitions by their
// whole-MiB size. Deleted rows live in deletedPartitions and are excluded, so
// removing a row correctly frees its real space.
export function computeUsedBytes(layout: ManualLayout): number {
	return layout.partitions.reduce((sum, p) => {
		const existing = p.path ? (p.sizeBytes ?? 0) : 0;
		const newly = p.path === "" ? Math.max(p.sizeMB, 0) * bytesPerMB : 0;
		return sum + existing + newly;
	}, 0);
}

// computeFreeMB returns the remaining unallocated space in whole MiB. It is
// derived from byte-exact totals (disk size minus committed bytes) so that
// rounding existing partitions down to MiB — which the backend does when it
// reports them — can't overstate how much room a brand-new partition may take.
// A layout built without byte geometry falls back to the MiB totals.
export function computeFreeMB(layout: ManualLayout): number {
	const diskBytes = layout.diskSizeBytes ?? layout.diskSizeMB * bytesPerMB;
	const free = Math.max(diskBytes - computeUsedBytes(layout), 0);
	return Math.floor(free / bytesPerMB);
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

// partitionSizeMB returns a designated partition's size in MiB. A sizeMB of 0
// means "use all remaining space", reported as -1 so callers can exempt it from
// a fixed floor (its real size is whatever the disk leaves over).
function partitionSizeMB(p: ManualPartition): number {
	if (p.sizeMB === 0) return -1;
	if (p.path && p.sizeBytes) return Math.floor(p.sizeBytes / bytesPerMB);
	return p.sizeMB;
}

// isManualLayoutValid is true when the plan has exactly one ESP and one System,
// and each designated partition meets its minimum size: ESP >= 128 MiB, System
// >= 8 GiB (the disk floor). A "use all remaining space" row (sizeMB 0) is
// exempt — its size is whatever the disk leaves, which already respects the
// disk's own 8 GiB floor. Kept in sync with partition.Validate in the backend.
export function isManualLayoutValid(layout: ManualLayout | null): boolean {
	if (!layout) return false;
	if (!hasRequiredTypes(layout)) return false;
	for (const p of layout.partitions) {
		const sizeMB = partitionSizeMB(p);
		if (sizeMB < 0) continue; // "use all remaining space" — exempt from the floor
		if (p.type === TYPE_ESP && sizeMB < MinESPSizeBytes / bytesPerMB) {
			return false;
		}
		if (p.type === TYPE_SYSTEM && sizeMB < MinDiskSizeBytes / bytesPerMB) {
			return false;
		}
	}
	return true;
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

// ── Validation Reasons for Modal ─────────────────────────────────────────────

/**
 * Returns an array of human-readable validation reason strings for the given page.
 * Used by the validation error modal to tell the user why they can't proceed.
 */
export function getPageValidationReasons(page: number): string[] {
	switch (page) {
		case 0: // Welcome
			return installState.locale ? [] : [t('validation.modal.selectLanguage')];
		case 1: // User Account
			return getUserAccountValidationReasons();
		case 2: // Disk Selection
			if (installState.manualMode) {
				return getManualLayoutValidationReasons();
			}
			return getAutoDiskValidationReasons();
		case 3: // Confirmation
			return [t('validation.modal.reviewInstallation')];
		case 4: // Installation
			return [t('validation.modal.waitInstallation')];
		default:
			return [];
	}
}

function getUserAccountValidationReasons(): string[] {
	const reasons: string[] = [];
	if (!installState.username.trim()) {
		reasons.push(t('validation.modal.usernameRequired'));
	} else if (installState.username.length < 3) {
		reasons.push(t('validation.modal.usernameMinLength'));
	} else if (installState.username.length > 32) {
		reasons.push(t('validation.modal.usernameMaxLength'));
	} else if (installState.username.length > 0 && !/^[a-z_]/.test(installState.username)) {
		reasons.push(t('validation.modal.usernameInvalidFirstChar'));
	} else if (installState.username.length > 0 && !/^[a-z0-9_-]+$/.test(installState.username)) {
		reasons.push(t('validation.modal.usernameInvalidChars'));
	}
	if (!installState.password) {
		reasons.push(t('validation.modal.passwordRequired'));
	} else if (installState.password.length < 6) {
		reasons.push(t('validation.modal.passwordMinLength'));
	}
	if (installState.password && installState.confirmPassword !== installState.password) {
		reasons.push(t('validation.modal.passwordsNoMatch'));
	}
	return reasons;
}

function getAutoDiskValidationReasons(): string[] {
	const reasons: string[] = [];
	if (!installState.diskId) {
		reasons.push(t('validation.modal.selectDisk'));
	}
	if (!installState.fileSystem) {
		reasons.push(t('validation.modal.selectFileSystem'));
	}
	if (installState.disk && installState.disk.size < MinDiskSizeBytes) {
		reasons.push(t('validation.modal.diskTooSmall'));
	}
	return reasons;
}

function getManualLayoutValidationReasons(): string[] {
	const reasons: string[] = [];
	if (!installState.diskId) {
		reasons.push(t('validation.modal.selectDisk'));
		return reasons;
	}
	if (!installState.fileSystem) {
		reasons.push(t('validation.modal.selectFileSystem'));
		return reasons;
	}
	if (!installState.manualLayout) {
		return reasons;
	}

	const layout = installState.manualLayout;
	const espCount = layout.partitions.filter((p) => p.type === TYPE_ESP).length;
	const sysCount = layout.partitions.filter((p) => p.type === TYPE_SYSTEM).length;

	if (espCount !== 1) {
		reasons.push(t('validation.modal.espRequired', { n: espCount }));
	}
	if (sysCount !== 1) {
		reasons.push(t('validation.modal.systemRequired', { n: sysCount }));
	}

	for (const p of layout.partitions) {
		const sizeMB = partitionSizeMB(p);
		if (sizeMB < 0) continue; // "use all remaining space" — exempt
		if (p.type === TYPE_ESP && sizeMB < MinESPSizeBytes / bytesPerMB) {
			reasons.push(
				t('validation.modal.espTooSmall', { size: sizeMB }),
			);
		}
		if (p.type === TYPE_SYSTEM && sizeMB < MinDiskSizeBytes / bytesPerMB) {
			reasons.push(
				t('validation.modal.systemTooSmall', { size: sizeMB }),
			);
		}
		if (p.type && p.type !== TYPE_ESP && p.type !== TYPE_SYSTEM && p.type !== 'preserve') {
			reasons.push(t('validation.modal.invalidType', { type: p.type }));
		}
	}

	// Check for multiple "use remaining space" partitions
	const remainingCount = layout.partitions.filter((p) => p.sizeMB === 0 && p.type).length;
	if (remainingCount > 1) {
		reasons.push(t('validation.modal.multipleRemaining'));
	}

	// Check for partition after remaining space
	const remainingIndex = layout.partitions.findIndex((p) => p.sizeMB === 0);
	if (remainingIndex >= 0 && remainingIndex < layout.partitions.length - 1) {
		reasons.push(t('validation.modal.partitionAfterRemaining'));
	}

	// Check new partition sizes against free space
	const usedMB = computeUsedMB(layout);
	const freeMB = computeFreeMB(layout);
	const newParts = layout.partitions.filter((p) => !p.path);
	const newPartsSize = newParts.reduce((sum, p) => sum + Math.max(p.sizeMB, 0), 0);
	if (newPartsSize > freeMB) {
		reasons.push(
			t('validation.modal.exceedsFreeSpace', { size: newPartsSize, free: freeMB }),
		);
	}

	// Check partition sizes against disk size
	if (usedMB > layout.diskSizeMB) {
		reasons.push(
			t('validation.modal.exceedsDisk', { size: usedMB, disk: layout.diskSizeMB }),
		);
	}

	return reasons;
}
