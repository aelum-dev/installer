import { describe, it, expect } from 'vitest';
import type { DiskInfo, PartitionEntry } from '$lib/types';
import {
	contentSummary,
	destinationChain,
	destinationSegments,
	disabledReason,
	diskSegments,
	fsLabel,
	manualMapSegments,
	manualPlanLabel,
	statusChip,
} from './disk-layout';

// Fixtures mirror the shapes backend/shared/disks_fake.go produces.
function disk(over: Partial<DiskInfo> = {}): DiskInfo {
	return {
		name: 'sda',
		path: '/dev/sda',
		size: 1000 * 1024 * 1024, // 1000 MiB
		sizeFormatted: '1000.0 MB',
		type: 'disk',
		model: 'Test Disk',
		vendor: '',
		rotational: false,
		readonly: false,
		fstype: null,
		mountpoint: null,
		serial: '',
		trans: 'sata',
		isRemovable: false,
		partitionTable: 'gpt',
		inUse: false,
		isLiveDevice: false,
		...over,
	};
}

function part(over: Partial<PartitionEntry> = {}): PartitionEntry {
	return {
		path: '/dev/sda1',
		name: 'sda1',
		type: 'part',
		size: 500 * 1024 * 1024,
		fstype: 'ext4',
		uuid: '',
		mountpoints: [],
		...over,
	};
}

describe('fsLabel', () => {
	it('names known filesystems and passes unknown ones through uppercased', () => {
		expect(fsLabel('fat32')).toBe('FAT32');
		expect(fsLabel('ntfs')).toBe('NTFS');
		expect(fsLabel('f2fs')).toBe('F2FS');
		expect(fsLabel(null)).toBe('');
	});
});

describe('diskSegments', () => {
	it('draws one segment per partition plus the free remainder', () => {
		const segs = diskSegments(
			disk({ size: 1000 * 1024 * 1024 }),
			[
				part({ path: '/dev/sda1', size: 512 * 1024 * 1024, fstype: 'fat32' }),
				part({ path: '/dev/sda2', size: 448 * 1024 * 1024, fstype: 'ext4' }),
			],
		);
		expect(segs.map((s) => s.fs)).toEqual(['fat32', 'ext4', '']);
		expect(segs.map((s) => Math.round(s.pct))).toEqual([51, 45, 4]);
		expect(segs[2].free).toBe(true);
	});

	it('keeps a tiny ESP visible through its own pct, not dropped', () => {
		const segs = diskSegments(disk({ size: 500 * 1024 ** 3 }), [
			part({ size: 512 * 1024 * 1024, fstype: 'fat32' }),
		]);
		expect(segs[0].pct).toBeGreaterThan(0);
		expect(segs[0].pct).toBeLessThan(0.2);
	});

	it('renders an unpartitioned disk with a whole-disk filesystem as one full segment', () => {
		const segs = diskSegments(disk({ fstype: 'ntfs' }), []);
		expect(segs).toHaveLength(1);
		expect(segs[0].fs).toBe('ntfs');
		expect(segs[0].pct).toBe(100);
	});

	it('renders a fully empty disk as the bare track (no segments)', () => {
		expect(diskSegments(disk(), [])).toEqual([]);
	});

	it('drops a rounding sliver of free space', () => {
		const segs = diskSegments(disk({ size: 1000 * 1024 * 1024 }), [
			part({ size: 999.5 * 1024 * 1024, fstype: 'ext4' }),
		]);
		expect(segs).toHaveLength(1);
		expect(segs[0].free).toBe(false);
	});

	it('carries mounts into the tooltip so the track can be read on hover', () => {
		const segs = diskSegments(disk({ size: 1000 * 1024 * 1024 }), [
			part({ size: 1000 * 1024 * 1024, fstype: 'swap', mountpoints: ['[SWAP]'] }),
		]);
		expect(segs[0].tooltip).toContain('[SWAP]');
	});
});

describe('contentSummary', () => {
	it('counts partitions and lists their filesystems once', () => {
		const line = contentSummary(
			disk({ size: 1000 * 1024 * 1024 }),
			[
				part({ path: '/dev/sda1', size: 10, fstype: 'fat32' }),
				part({ path: '/dev/sda2', size: 10, fstype: 'ext4' }),
				part({ path: '/dev/sda3', size: 10, fstype: 'ext4' }),
			],
		);
		expect(line).toBe('3 partitions · FAT32, ext4');
	});

	it('uses the singular for one partition and names its mount', () => {
		const line = contentSummary(
			disk({ size: 1000 * 1024 * 1024 }),
			[part({ size: 1000 * 1024 * 1024, fstype: 'btrfs', mountpoints: ['/home'] })],
		);
		expect(line).toBe('1 partition · btrfs · mounted at /home');
	});

	it('answers with the boot source when the system booted from this disk', () => {
		expect(contentSummary(disk({ mountpoint: '/' }), [])).toBe(
			'mounted at / — the system booted from',
		);
		expect(
			contentSummary(disk({}), [part({ fstype: 'ext4', mountpoints: ['/', '/home'] })]),
		).toBe('mounted at / — the system booted from');
	});

	it('says empty for an empty disk and names a whole-disk filesystem', () => {
		expect(contentSummary(disk(), [])).toBe('Empty — no partitions');
		expect(contentSummary(disk({ fstype: 'ntfs' }), [])).toBe('NTFS');
	});

	it('names the booted disk even when no partition claims "/" (a live ISO)', () => {
		// On a booted ISO the USB stick's only mount is /run/archiso/bootmnt
		// and the root is an overlay — the backend's live bit is all that
		// marks the stick, and it outranks the mount list.
		expect(
			contentSummary(disk({ isLiveDevice: true }), [
				part({ fstype: 'iso9660', mountpoints: ['/run/archiso/bootmnt'] }),
			]),
		).toBe('mounted at / — the system booted from');
	});
});

describe('statusChip', () => {
	it('marks the booted-from disk as in use', () => {
		expect(statusChip(disk({ mountpoint: '/' }), [])).toEqual({ tone: 'danger', text: 'In use' });
	});

	it('marks a merely mounted disk before removable media', () => {
		const d = disk({ isRemovable: true });
		expect(statusChip(d, [part({ mountpoints: ['/mnt/data'] })])).toEqual({
			tone: 'warning',
			text: 'Mounted',
		});
	});

	it('marks removable media, and nothing for a plain disk', () => {
		expect(statusChip(disk({ isRemovable: true }), [])).toEqual({
			tone: 'warning',
			text: 'Removable',
		});
		expect(statusChip(disk(), [])).toBeNull();
	});

	it('does not count "[SWAP]" as a mount', () => {
		expect(statusChip(disk(), [part({ fstype: 'swap', mountpoints: ['[SWAP]'] })])).toBeNull();
	});

	it('trusts the backend flags that no mount list can show', () => {
		// The live ISO's stick: no partition is mounted at "/", the root is an
		// overlay — the live bit is what makes it the danger chip.
		expect(statusChip(disk({ isLiveDevice: true }), [])).toEqual({
			tone: 'danger',
			text: 'In use',
		});
		// Active swap or a hidden btrfs mount: the backend saw it, the
		// partition mountpoints cannot show it.
		expect(statusChip(disk({ inUse: true }), [])).toEqual({
			tone: 'warning',
			text: 'Mounted',
		});
	});
});

describe('disabledReason', () => {
	it('states the floor for a too-small disk and nothing for a big one', () => {
		expect(disabledReason(disk({ size: 7 * 1024 ** 3 }))).toBe(
			'Too small — Aelum needs at least 8.0 GB',
		);
		expect(disabledReason(disk({ size: 240 * 1024 ** 3 }))).toBeNull();
	});

	it('refuses a disk the running system is using', () => {
		const big = { size: 240 * 1024 ** 3 };
		expect(disabledReason(disk({ ...big, inUse: true }))).toBe(
			'In use — the system is using this disk',
		);
		expect(disabledReason(disk({ ...big, isLiveDevice: true }))).toBe(
			'In use — the system is using this disk',
		);
		// Size is the older, harder fact — it is named first.
		expect(disabledReason(disk({ size: 7 * 1024 ** 3, inUse: true }))).toBe(
			'Too small — Aelum needs at least 8.0 GB',
		);
	});
});

describe('destination', () => {
	const big = disk({ size: 931.5 * 1024 ** 3, sizeFormatted: '931.5 GB' });
	const plain = { fileSystem: 'btrfs' as const, luks: false, lvm: false };

	it('spells ESP + root, and adds each wrapping in order', () => {
		expect(destinationChain(big, plain)).toBe('FAT32 512.0 MB · btrfs root 931.0 GB');
		expect(destinationChain(big, { ...plain, luks: true })).toBe(
			'FAT32 512.0 MB · LUKS2 · btrfs root 931.0 GB',
		);
		expect(destinationChain(big, { fileSystem: 'ext4', luks: true, lvm: true })).toBe(
			'FAT32 512.0 MB · LUKS2 · LVM · ext4 root 931.0 GB',
		);
	});

	it('draws exactly two segments — the root one wears the LUKS hatch', () => {
		const segs = destinationSegments(big, { fileSystem: 'ext4', luks: true, lvm: false });
		expect(segs.map((s) => s.fs)).toEqual(['fat32', 'ext4']);
		expect(segs.map((s) => s.hatch)).toEqual([false, true]);
		expect(segs[0].pct + segs[1].pct).toBeCloseTo(100);
		expect(segs[1].tooltip).toBe(destinationChain(big, { fileSystem: 'ext4', luks: true, lvm: false }));
	});
});

describe('manual map', () => {
	it('labels roles, outlines them, and keeps the remainder FREE', () => {
		const segs = manualMapSegments({
			disk: '/dev/nvme0n1',
			diskSizeMB: 483143,
			diskSizeBytes: 483143 * 1024 * 1024,
			partitions: [
				{ index: 1, path: '/dev/nvme0n1p1', sizeMB: 512, sizeBytes: 512 * 1024 * 1024, type: 'esp', existingFstype: 'fat32' },
				{ index: 2, path: '/dev/nvme0n1p2', sizeMB: 300000, sizeBytes: 300000 * 1024 * 1024, type: 'system', existingFstype: 'btrfs' },
			],
		});
		expect(segs.map((s) => s.label)).toEqual(['Boot', 'System', 'Free']);
		expect(segs.map((s) => s.role)).toEqual([true, true, false]);
		expect(segs[2].free).toBe(true);
	});

	it('a "use remaining" row eats the rest of the disk, leaving no FREE segment', () => {
		const segs = manualMapSegments({
			disk: '/dev/sda',
			diskSizeMB: 1000,
			partitions: [
				{ index: 1, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * 1024 * 1024, type: 'esp', existingFstype: 'fat32' },
				{ index: 2, path: '', sizeMB: 0, type: 'system', existingFstype: '' },
			],
		});
		expect(segs.some((s) => s.free)).toBe(false);
		expect(segs[1].pct).toBeCloseTo(48.8, 0);
	});

	it('names the plan the same way the chain does, without sizes', () => {
		expect(manualPlanLabel({ fileSystem: 'ext4', luks: false, lvm: false })).toBe('ext4 root');
		expect(manualPlanLabel({ fileSystem: 'btrfs', luks: true, lvm: true })).toBe(
			'btrfs root on LUKS2 on LVM',
		);
	});
});
