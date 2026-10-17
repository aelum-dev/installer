// Manual (dualboot-safe) partition-plan math and validation.
//
// Pure helpers only — no Svelte state, no i18n. The store owns the selected
// plan; this module derives free space and reports structural problems as
// machine-readable issues. `page-validation.ts` turns those into translated
// strings, and the UI renders them.

import type { ManualLayout, ManualPartition, PartitionType } from '$lib/types';
import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
import { GPTBackupBytes, GPTPrefixBytes, MinDiskSizeBytes, MinESPSizeBytes } from '$lib/constants';

const BYTES_PER_MB = 1024 * 1024;

// The two regions a GPT reserves and the backend never allocates: the trailing
// backup header and the leading header + protective MBR. Sourced from the
// backend (shared.GPTBackupBytes / shared.GPTPrefixBytes via the generated
// constants.ts) so the frontend's free-space math can never drift from
// partition.go. The backend reserves both before it places a brand-new
// partition, so the frontend must too — otherwise it overstates usable space
// (by the prefix) or rejects a plan that fills the disk (by the backup).
const GPT_RESERVED_BYTES = GPTPrefixBytes + GPTBackupBytes;

/** Sum of the space the plan has reserved, in MiB. */
export function computeUsedMB(layout: ManualLayout): number {
	return layout.partitions.reduce((sum, p) => sum + Math.max(p.sizeMB, 0), 0);
}

/**
 * Sum of the space the plan has reserved, in exact bytes: existing partitions
 * by their real byte size, brand-new partitions by their whole-MiB size.
 * Deleted rows live in deletedPartitions and are excluded.
 */
export function computeUsedBytes(layout: ManualLayout): number {
	return layout.partitions.reduce((sum, p) => {
		const existing = p.path ? (p.sizeBytes ?? 0) : 0;
		const newly = p.path === '' ? Math.max(p.sizeMB, 0) * BYTES_PER_MB : 0;
		return sum + existing + newly;
	}, 0);
}

/**
 * Remaining unallocated space in whole MiB, derived from byte-exact totals so
 * that rounding existing partitions down to MiB can't overstate how much room
 * a brand-new partition may take. A layout without byte geometry falls back to
 * MiB totals.
 */
export function computeFreeMB(layout: ManualLayout): number {
	const diskBytes = layout.diskSizeBytes ?? layout.diskSizeMB * BYTES_PER_MB;
	const free = Math.max(diskBytes - computeUsedBytes(layout), 0);
	// Flooring to whole MiB matches the backend's 1-MiB-aligned gap, so "use
	// remaining" proposes exactly what a brand-new partition can be placed at.
	return Math.floor(Math.max(free - GPT_RESERVED_BYTES, 0) / BYTES_PER_MB);
}

/** How many partitions carry the given type. */
export function typeCount(layout: ManualLayout | null, type: PartitionType): number {
	if (!layout) return 0;
	return layout.partitions.filter((p) => p.type === type).length;
}

/** True when the layout has exactly one ESP and one System. */
export function hasRequiredTypes(layout: ManualLayout | null): boolean {
	if (!layout) return false;
	return typeCount(layout, TYPE_ESP) === 1 && typeCount(layout, TYPE_SYSTEM) === 1;
}

/**
 * A designated partition's size in MiB. A sizeMB of 0 means "use all
 * remaining space", reported as -1 so callers can exempt it from a fixed floor.
 */
function partitionSizeMB(p: ManualPartition): number {
	if (p.sizeMB === 0) return -1;
	if (p.path && p.sizeBytes) return Math.floor(p.sizeBytes / BYTES_PER_MB);
	return p.sizeMB;
}

/** A structural problem with a manual plan, resolved to text by the caller. */
export type ManualLayoutIssue =
	| { kind: 'espCount'; count: number }
	| { kind: 'systemCount'; count: number }
	| { kind: 'espTooSmall'; sizeMB: number }
	| { kind: 'systemTooSmall'; sizeMB: number }
	| { kind: 'invalidType'; type: string }
	| { kind: 'multipleRemaining' }
	| { kind: 'partitionAfterRemaining' }
	| { kind: 'exceedsFreeSpace'; sizeMB: number; freeMB: number }
	| { kind: 'exceedsDisk'; usedMB: number; diskMB: number };

/**
 * Every reason the plan can't be applied, mirroring partition.Validate in the
 * backend. Empty means valid. Kept as data (not translated strings) so the
 * same checks drive both the boolean gate and the explanatory modal.
 */
export function manualLayoutIssues(layout: ManualLayout): ManualLayoutIssue[] {
	const issues: ManualLayoutIssue[] = [];

	const espCount = typeCount(layout, TYPE_ESP);
	const sysCount = typeCount(layout, TYPE_SYSTEM);
	if (espCount !== 1) issues.push({ kind: 'espCount', count: espCount });
	if (sysCount !== 1) issues.push({ kind: 'systemCount', count: sysCount });

	for (const p of layout.partitions) {
		const sizeMB = partitionSizeMB(p);
		if (sizeMB < 0) continue; // "use all remaining space" — exempt from the floor

		if (p.type === TYPE_ESP && sizeMB < MinESPSizeBytes / BYTES_PER_MB) {
			issues.push({ kind: 'espTooSmall', sizeMB });
		}
		if (p.type === TYPE_SYSTEM && sizeMB < MinDiskSizeBytes / BYTES_PER_MB) {
			issues.push({ kind: 'systemTooSmall', sizeMB });
		}
		if (p.type && p.type !== TYPE_ESP && p.type !== TYPE_SYSTEM && p.type !== 'preserve') {
			issues.push({ kind: 'invalidType', type: p.type });
		}
	}

	const remainingCount = layout.partitions.filter((p) => p.sizeMB === 0 && p.type).length;
	if (remainingCount > 1) issues.push({ kind: 'multipleRemaining' });

	const remainingIndex = layout.partitions.findIndex((p) => p.sizeMB === 0);
	if (remainingIndex >= 0 && remainingIndex < layout.partitions.length - 1) {
		issues.push({ kind: 'partitionAfterRemaining' });
	}

	// New rows are what we are fitting, so they must be checked against the
	// headroom left by partitions already on the disk — never against the free
	// space after subtracting them (that double-counts them and rejects any
	// plan whose new partitions make up a large share of the disk).
	const usedExistingBytes = layout.partitions.reduce(
		(sum, p) => sum + (p.path ? (p.sizeBytes ?? 0) : 0),
		0,
	);
	const diskBytes = layout.diskSizeBytes ?? layout.diskSizeMB * BYTES_PER_MB;
	const freeForNewMB = Math.max(
		Math.floor((diskBytes - usedExistingBytes - GPT_RESERVED_BYTES) / BYTES_PER_MB),
		0,
	);
	const newPartsSize = layout.partitions
		.filter((p) => !p.path)
		.reduce((sum, p) => sum + Math.max(p.sizeMB, 0), 0);
	if (newPartsSize > freeForNewMB) {
		issues.push({ kind: 'exceedsFreeSpace', sizeMB: newPartsSize, freeMB: freeForNewMB });
	}

	const usedMB = computeUsedMB(layout);
	if (usedMB > layout.diskSizeMB) {
		issues.push({ kind: 'exceedsDisk', usedMB, diskMB: layout.diskSizeMB });
	}

	return issues;
}

/**
 * True when the plan is applicable. This is `manualLayoutIssues(layout).length
 * === 0`; it intentionally includes the free-space and disk-size checks so the
 * UI gate can never let the backend reject a plan it showed as valid.
 */
export function isManualLayoutValid(layout: ManualLayout | null): boolean {
	if (!layout) return false;
	return manualLayoutIssues(layout).length === 0;
}

/** Set a partition's type by row index. */
export function setPartitionType(
	partitions: ManualPartition[],
	index: number,
	type: PartitionType,
): void {
	const part = partitions.find((p) => p.index === index);
	if (part) part.type = type;
}
