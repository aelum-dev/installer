import { describe, it, expect } from 'vitest';
import { inferDiskFileSystem } from './disks';
import type { DiskInfo, PartitionEntry } from './types';

function disk(over: Partial<DiskInfo> = {}): DiskInfo {
	return {
		name: 'sda',
		path: '/dev/sda',
		size: 100 * 1024 * 1024 * 1024,
		sizeFormatted: '100 GiB',
		type: 'disk',
		model: 'Test Disk',
		vendor: '',
		rotational: false,
		readonly: false,
		fstype: null,
		mountpoint: null,
		serial: '',
		trans: 'nvme',
		isRemovable: false,
		...over,
	};
}

function part(over: Partial<PartitionEntry>): PartitionEntry {
	return {
		path: '/dev/sda1',
		name: 'sda1',
		type: 'part',
		size: 1024,
		fstype: null,
		uuid: '',
		mountpoints: [],
		...over,
	};
}

describe('inferDiskFileSystem', () => {
	it('prefers the filesystem reported by the disk itself', () => {
		expect(inferDiskFileSystem(disk({ fstype: 'xfs' }), [part({ fstype: 'ext4' })])).toBe('xfs');
	});

	it('prefers the partition mounted at /', () => {
		const partitions = [
			part({ path: '/dev/sda1', fstype: 'ext4' }),
			part({ path: '/dev/sda2', fstype: 'btrfs', mountpoints: ['/'] }),
		];
		expect(inferDiskFileSystem(disk(), partitions)).toBe('btrfs');
	});

	it('falls back to a btrfs or ext4 partition', () => {
		const partitions = [
			part({ path: '/dev/sda1', fstype: 'vfat' }),
			part({ path: '/dev/sda2', fstype: 'ext4' }),
		];
		expect(inferDiskFileSystem(disk(), partitions)).toBe('ext4');
	});

	it('falls back to the first formatted partition', () => {
		expect(inferDiskFileSystem(disk(), [part({ fstype: 'xfs' })])).toBe('xfs');
	});

	it('returns null when nothing reports a filesystem', () => {
		expect(inferDiskFileSystem(disk(), [part({}), part({ path: '/dev/sda2' })])).toBeNull();
	});
});
