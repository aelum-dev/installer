// The wizard's disk-picture math: one shared model behind every drawing of a
// disk — the "what is" strip on a device row, the "what will be" destination
// strip (row + rail card), and the manual-mode disk map. The components render
// segments; this module decides their sizes, kinds, labels and tooltips, so the
// three pictures can never disagree about the same disk.
//
// The strip language is monochrome by product decision: every partition wears
// the primary color whatever its filesystem — type and size live in the
// tooltip. Structure comes from texture and geometry only: the bare track is
// free space, a diagonal hatch is LUKS, hairline gaps are segment edges.

import type { DiskInfo, FileSystem, ManualLayout, PartitionEntry } from '$lib/types';
import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
import {
	LvmMetadataReserveBytes,
	LuksHeaderReserveBytes,
	MinDiskSizeBytes,
	MinESPSizeBytes,
} from '$lib/constants';
import { formatBytes } from '$lib/format';
import { t } from '$lib/i18n';

const BYTES_PER_MB = 1024 * 1024;

/** The storage choices that decide what the destination layout will be. */
export interface PlanChoice {
	fileSystem: FileSystem;
	luks: boolean;
	lvm: boolean;
}

/** One segment of a disk picture: a partition, a wrapping, or free space. */
export interface StripSegment {
	key: string;
	/** Share of the whole disk, 0–100. The renderer clamps tiny ones to a visible floor. */
	pct: number;
	/** Sanitized filesystem name for the CSS class (`seg-<fs>`); '' for free space. */
	fs: string;
	/** Plain-words content of the segment for the title attribute. */
	tooltip: string;
	/** LUKS wrapping — the renderer textures the segment with a hatch. */
	hatch: boolean;
	/** Free space — the renderer leaves the bare track. */
	free: boolean;
	/** Text drawn inside the segment (manual-mode map only, when it fits). */
	label: string;
	/** A designated Boot/System partition — the map outlines its segment. */
	role: boolean;
}

/** Free space below this share of the disk is not drawn (a rounding sliver). */
const MIN_FREE_SHARE = 0.1;

// The filesystems the wizard ships translations for; anything else lsblk
// reports is shown as its uppercase raw name.
const KNOWN_FS = new Set(['btrfs', 'ext4', 'fat32', 'ntfs', 'swap', 'vfat', 'xfs', 'zfs']);

/** Filesystem name in words (FAT32, Ext4, …), or '' for unformatted. */
export function fsLabel(fs: string | null | undefined): string {
	if (!fs) return '';
	return KNOWN_FS.has(fs) ? t(`fs.${fs}`) : fs.toUpperCase();
}

/** Filesystem name safe for a CSS class; unknown shapes collapse to 'raw'. */
function sanitizeFs(fs: string | null | undefined): string {
	if (!fs) return '';
	return /^[a-z0-9]+$/.test(fs) ? fs : 'raw';
}

/** Mountpoints of a partition's filesystems; "[SWAP]" is not a directory. */
function directories(p: PartitionEntry): string[] {
	return (p.mountpoints ?? []).filter((m) => m && m !== '[SWAP]');
}

/** True when this disk (or a partition on it) is where the session booted from. */
function isBootedFrom(disk: DiskInfo, parts: PartitionEntry[]): boolean {
	// The backend names the live system's own disk by device identity — the
	// disk behind / or behind the ISO's /run/archiso/bootmnt. On a booted ISO
	// no partition is mounted at "/" (the root is an overlay), so without this
	// bit the stick the installer is running from would only ever read as
	// "mounted".
	if (disk.isLiveDevice) return true;
	return disk.mountpoint === '/' || parts.some((p) => (p.mountpoints ?? []).includes('/'));
}

/** True when anything on this disk is mounted somewhere. */
function isMounted(disk: DiskInfo, parts: PartitionEntry[]): boolean {
	// The backend's aggregate is authoritative and counts what the per-mount
	// view cannot see: active swap, and a mount hidden behind the single
	// mountpoint lsblk prints for a filesystem mounted more than once.
	if (disk.inUse) return true;
	if (disk.mountpoint && disk.mountpoint !== '/') return true;
	return parts.some((p) => directories(p).length > 0);
}

function partTooltip(p: PartitionEntry): string {
	const fs = fsLabel(p.fstype);
	const mounts = (p.mountpoints ?? []).filter(Boolean);
	const where = mounts.length ? ` — ${mounts.join(', ')}` : '';
	return `${formatBytes(p.size)}${fs ? ` ${fs}` : ''}${where}`;
}

function freeSegment(pct: number, tooltip: string): StripSegment {
	return { key: 'free', pct, fs: '', tooltip, hatch: false, free: true, label: '', role: false };
}

/**
 * The disk as it is: one segment per partition, in on-disk order, plus the
 * free remainder. An unpartitioned disk carrying a whole-disk filesystem is
 * one full segment of it; a fully empty disk renders as the bare track.
 */
export function diskSegments(disk: DiskInfo, parts: PartitionEntry[]): StripSegment[] {
	const total = Math.max(disk.size, 1);
	if (parts.length === 0) {
		if (!disk.fstype) return [];
		return [
			{
				key: disk.path,
				pct: 100,
				fs: sanitizeFs(disk.fstype),
				tooltip: partTooltip({
					path: disk.path,
					name: disk.name,
					type: 'disk',
					size: disk.size,
					fstype: disk.fstype,
					uuid: '',
					mountpoints: disk.mountpoint ? [disk.mountpoint] : [],
				}),
				hatch: false,
				free: false,
				label: '',
				role: false,
			},
		];
	}

	const segs: StripSegment[] = parts.map((p, i) => ({
		key: p.path || `p${i}`,
		pct: (p.size / total) * 100,
		fs: sanitizeFs(p.fstype),
		tooltip: partTooltip(p),
		hatch: false,
		free: false,
		label: '',
		role: false,
	}));

	const usedBytes = parts.reduce((sum, p) => sum + Math.max(p.size, 0), 0);
	const freePct = ((disk.size - usedBytes) / total) * 100;
	if (freePct >= MIN_FREE_SHARE) {
		segs.push(freeSegment(freePct, t('manual.unallocated')));
	}
	return segs;
}

/**
 * The plain-words content line: what history this disk carries, in the fewest
 * words that still tell two identical models apart.
 */
export function contentSummary(disk: DiskInfo, parts: PartitionEntry[]): string {
	if (isBootedFrom(disk, parts)) {
		return t('disk.contentInUse');
	}
	if (parts.length === 0) {
		// A whole-disk filesystem gets a line of its own; a truly empty disk says so.
		return disk.fstype ? fsLabel(disk.fstype) : t('disk.emptyDisk');
	}

	const countWord =
		parts.length === 1
			? t('disk.onePartition', { count: parts.length })
			: t('disk.manyPartitions', { count: parts.length });
	const fss = [...new Set(parts.map((p) => fsLabel(p.fstype)).filter(Boolean))];
	let line = fss.length ? `${countWord} · ${fss.join(', ')}` : countWord;

	const mounts = [...new Set(parts.flatMap((p) => directories(p)))];
	if (disk.mountpoint && disk.mountpoint !== '/') mounts.unshift(disk.mountpoint);
	if (mounts.length) {
		line += ` · ${t('disk.contentMounted', { mount: mounts.join(', ') })}`;
	}
	return line;
}

/** The one bit of triage a name cannot carry, or null when there is nothing to warn. */
export function statusChip(
	disk: DiskInfo,
	parts: PartitionEntry[],
): { tone: 'danger' | 'warning'; text: string } | null {
	if (isBootedFrom(disk, parts)) return { tone: 'danger', text: t('disk.chipInUse') };
	if (isMounted(disk, parts)) return { tone: 'warning', text: t('disk.chipMounted') };
	if (disk.isRemovable) return { tone: 'warning', text: t('disk.chipRemovable') };
	return null;
}

/** Why this disk cannot be selected, or null. */
export function disabledReason(disk: DiskInfo): string | null {
	if (disk.size < MinDiskSizeBytes) {
		return t('disk.tooSmall', { minSize: formatBytes(MinDiskSizeBytes) });
	}
	// The running system is on this disk — as its booted-from system or as a
	// mounted volume. Automatic mode's first command rewrites the partition
	// table; nothing mounted survives that, so the row is not a choice.
	if (disk.inUse || disk.isLiveDevice) {
		return t('disk.inUse');
	}
	return null;
}

/** The root filesystem the destination layout will format, after every wrapping takes its reserve. */
export function destinationRootBytes(disk: DiskInfo, plan: PlanChoice): number {
	let bytes = disk.size - MinESPSizeBytes;
	if (plan.luks) bytes -= LuksHeaderReserveBytes;
	if (plan.lvm) bytes -= LvmMetadataReserveBytes;
	return Math.max(bytes, 0);
}

/** The chain the destination strip spells out: "FAT32 512 MB · LUKS2 · LVM · btrfs root 476.4 GB". */
export function destinationChain(disk: DiskInfo, plan: PlanChoice): string {
	return [
		`${t('fs.fat32')} ${formatBytes(MinESPSizeBytes)}`,
		plan.luks ? 'LUKS2' : '',
		plan.lvm ? 'LVM' : '',
		`${t('disk.destRoot', { fs: plan.fileSystem })} ${formatBytes(destinationRootBytes(disk, plan))}`,
	]
		.filter(Boolean)
		.join(' · ');
}

/**
 * The disk as the installer will leave it in automatic mode: one FAT32 ESP,
 * then the root filesystem — LUKS shown as the hatch on the root segment, LVM
 * named in the chain (it is structure inside the segment, not its own slice).
 */
export function destinationSegments(disk: DiskInfo, plan: PlanChoice): StripSegment[] {
	const total = Math.max(disk.size, 1);
	const espPct = Math.min((MinESPSizeBytes / total) * 100, 100);
	const chain = destinationChain(disk, plan);
	return [
		{
			key: 'esp',
			pct: espPct,
			fs: 'fat32',
			tooltip: t('disk.destEfiTip', { size: formatBytes(MinESPSizeBytes) }),
			hatch: false,
			free: false,
			label: '',
			role: false,
		},
		{
			key: 'root',
			pct: 100 - espPct,
			fs: sanitizeFs(plan.fileSystem),
			tooltip: chain,
			hatch: plan.luks,
			free: false,
			label: '',
			role: false,
		},
	];
}

/**
 * The plan label beside the manual-mode disk map: the same wrapping language
 * as the chain, without sizes ("ext4 root on LUKS2 on LVM").
 */
export function manualPlanLabel(plan: PlanChoice): string {
	return [
		t('disk.destRoot', { fs: plan.fileSystem }),
		plan.luks ? t('manual.planOnLuks') : '',
		plan.lvm ? t('manual.planOnLvm') : '',
	]
		.filter(Boolean)
		.join(' ');
}

/**
 * The manual plan drawn as one bar: every row of the plan at its share of the
 * disk, designated rows outlined and labeled, the rest of the disk as FREE. A
 * "use all remaining space" row (sizeMB 0) eats the remainder, leaving no FREE
 * segment — which is exactly what it promises.
 */
export function manualMapSegments(layout: ManualLayout): StripSegment[] {
	const total = Math.max(layout.diskSizeBytes ?? layout.diskSizeMB * BYTES_PER_MB, 1);
	const parts = layout.partitions ?? [];

	const fixedBytes = parts.reduce((sum, p) => {
		if (p.path) return sum + (p.sizeBytes ?? p.sizeMB * BYTES_PER_MB);
		return sum + Math.max(p.sizeMB, 0) * BYTES_PER_MB;
	}, 0);
	// Rows claiming "all the rest" (new rows with sizeMB 0): the remainder is
	// their size, shared when (impossibly) more than one asks for it.
	const claimers = parts.filter((p) => !p.path && p.sizeMB === 0).length;
	const claimBytes = claimers > 0 ? Math.max(total - fixedBytes, 0) / claimers : 0;

	const segs: StripSegment[] = parts.map((p, i) => {
		const bytes = p.path
			? (p.sizeBytes ?? Math.max(p.sizeMB, 0) * BYTES_PER_MB)
			: p.sizeMB === 0
				? claimBytes
				: p.sizeMB * BYTES_PER_MB;
		const role = p.type === TYPE_ESP || p.type === TYPE_SYSTEM;
		// The same words the row's role pills carry — the map and the choice
		// never name the same partition differently ("Root" vs "System").
		const roleLabel = p.type === TYPE_ESP ? t('manual.type.esp') : p.type === TYPE_SYSTEM ? t('manual.type.system') : '';
		const name = p.path || t('manual.newPart');
		const fs = p.path ? fsLabel(p.existingFstype) : '';
		const tooltip = `${name} — ${fs ? `${fs} ` : ''}${formatBytes(bytes)}${role ? ` · ${t('manual.formatsNow')}` : ''}`;
		return {
			key: p.path || `new-${p.index ?? i}`,
			pct: (bytes / total) * 100,
			fs: sanitizeFs(p.existingFstype),
			tooltip,
			hatch: false,
			free: false,
			label: roleLabel,
			role,
		};
	});

	const freePct = ((total - fixedBytes - claimBytes * claimers) / total) * 100;
	if (freePct >= MIN_FREE_SHARE) {
		segs.push({
			key: 'free',
			pct: freePct,
			fs: '',
			tooltip: `${t('manual.unallocated')} — ${formatBytes((freePct / 100) * total)}`,
			hatch: false,
			free: true,
			label: t('manual.free'),
			role: false,
		});
	}
	return segs;
}
