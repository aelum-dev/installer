import { describe, it, expect, afterEach } from 'vitest';
import { getPageValidationReasons, isPageValid } from './page-validation';
import { installState } from './store.svelte';
import { PAGE_IDS } from './types';
import type { DiskInfo, ManualLayout } from './types';

const mib = 1024 * 1024;

const NOT_GPT_REASON =
	'Manual mode needs a GPT partition table. This disk has a different one; automatic mode rewrites it.';

function diskInfo(over: Partial<DiskInfo> = {}): DiskInfo {
	return {
		name: 'vda',
		path: '/dev/vda',
		size: 20480 * mib,
		sizeFormatted: '20.0 GiB',
		type: 'disk',
		model: 'Test Disk',
		vendor: '',
		rotational: false,
		readonly: false,
		fstype: null,
		mountpoint: null,
		serial: '',
		trans: 'virtio',
		isRemovable: false,
		partitionTable: 'gpt',
		...over,
	};
}

// A manual plan that is valid in every other respect: an existing 512 MB Boot
// partition and an existing 8192 MB System partition on a 20 GiB disk.
function validManualLayout(): ManualLayout {
	return {
		disk: '/dev/vda',
		diskSizeMB: 20480,
		diskSizeBytes: 20480 * mib,
		partitions: [
			{ index: 0, path: '/dev/vda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
			{ index: 1, path: '/dev/vda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
		],
	};
}

function resetState(): void {
	installState.locale = 'en-US';
	installState.username = '';
	installState.password = '';
	installState.confirmPassword = '';
	installState.diskId = '';
	installState.disk = null;
	installState.fileSystem = '';
	installState.manualMode = false;
	installState.manualLayout = null;
	installState.luksEnabled = false;
}

function setManual(layout: ManualLayout): void {
	installState.manualMode = true;
	installState.diskId = layout.disk;
	installState.fileSystem = 'ext4';
	installState.manualLayout = layout;
}

afterEach(resetState);

describe('getPageValidationReasons — new-partition free space', () => {
	it('accepts new partitions that fit the headroom left by existing rows', () => {
		// 100000 MB disk, 512 MB ESP already on disk, system deleted. New 80000 MB
		// system fits in the ~99488 MB headroom. Under the old bug, free would be
		// 100000-512-80000=19488 and 80000>19488 would (wrongly) reject it.
		setManual({
			disk: '/dev/sda',
			diskSizeMB: 100000,
			diskSizeBytes: 100000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 80000, type: 'system' },
			],
		});
		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).toEqual([]);
	});

	it('rejects new partitions that exceed the headroom left by existing rows', () => {
		setManual({
			disk: '/dev/sda',
			diskSizeMB: 100000,
			diskSizeBytes: 100000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 99500, type: 'system' },
			],
		});
		// Over-allocation is still rejected (this also trips the disk-size check,
		// since new + existing > disk), so just assert it does not silently pass.
		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).not.toEqual([]);
	});
});

describe('getPageValidationReasons — manual mode partition table', () => {
	// A manual plan is written with sgdisk, which refuses to save any change on a
	// disk whose partition table is not a GPT. The wizard must say so before the
	// user designs a plan that cannot be applied.
	it('refuses a manual plan on a disk with an MBR table', () => {
		setManual(validManualLayout());
		installState.disk = diskInfo({ partitionTable: 'dos' });

		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).toEqual([NOT_GPT_REASON]);
		expect(isPageValid(PAGE_IDS.DISK_SELECTION)).toBe(false);
	});

	it('accepts a GPT disk', () => {
		setManual(validManualLayout());
		installState.disk = diskInfo({ partitionTable: 'gpt' });

		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).toEqual([]);
		expect(isPageValid(PAGE_IDS.DISK_SELECTION)).toBe(true);
	});

	// No table reported means either an unpartitioned disk (sgdisk creates a GPT)
	// or a device the backend could not read. Neither may block the wizard: Apply
	// re-checks as root before it writes anything.
	it('accepts a disk whose table could not be read', () => {
		setManual(validManualLayout());
		installState.disk = diskInfo({ partitionTable: '' });

		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).toEqual([]);
	});

	// Automatic mode zaps the table first (`sgdisk -Z`), so any table is fine.
	it('does not gate automatic mode', () => {
		installState.diskId = '/dev/vda';
		installState.fileSystem = 'ext4';
		installState.disk = diskInfo({ partitionTable: 'dos' });

		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).toEqual([]);
		expect(isPageValid(PAGE_IDS.DISK_SELECTION)).toBe(true);
	});
});

describe('getPageValidationReasons — user account', () => {
	it('reports a reserved username (the old modal path never checked it)', () => {
		installState.username = 'root';
		installState.password = 'secret1';
		installState.confirmPassword = 'secret1';

		const reasons = getPageValidationReasons(PAGE_IDS.USER_ACCOUNT);
		expect(reasons).toEqual(["Username 'root' is reserved."]);
	});

	it('reports the first failing username rule only', () => {
		installState.username = '123';
		installState.password = 'secret1';
		installState.confirmPassword = 'secret1';

		expect(getPageValidationReasons(PAGE_IDS.USER_ACCOUNT)).toEqual([
			'Username cannot consist only of digits.',
		]);
	});

	it('does not report a confirmation mismatch when no password is set', () => {
		installState.username = 'alice';

		expect(getPageValidationReasons(PAGE_IDS.USER_ACCOUNT)).toEqual([
			'Password is required.',
		]);
	});

	it('accepts a fully valid account', () => {
		installState.username = 'alice';
		installState.password = 'secret1';
		installState.confirmPassword = 'secret1';

		expect(getPageValidationReasons(PAGE_IDS.USER_ACCOUNT)).toEqual([]);
		expect(isPageValid(PAGE_IDS.USER_ACCOUNT)).toBe(true);
	});
});

describe('isPageValid — agrees with the modal reasons', () => {
	it('gates the user-account page on the same rules as the modal', () => {
		installState.username = 'alice';
		installState.password = 'secret1';
		installState.confirmPassword = 'different';

		expect(isPageValid(PAGE_IDS.USER_ACCOUNT)).toBe(false);
		expect(getPageValidationReasons(PAGE_IDS.USER_ACCOUNT)).not.toEqual([]);
	});

	it('gates the manual disk page on the full layout check', () => {
		setManual({
			disk: '/dev/sda',
			diskSizeMB: 100000,
			diskSizeBytes: 100000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 99500, type: 'system' },
			],
		});

		expect(isPageValid(PAGE_IDS.DISK_SELECTION)).toBe(false);
		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).not.toEqual([]);
	});

	it('accepts a valid manual plan', () => {
		setManual({
			disk: '/dev/sda',
			diskSizeMB: 100000,
			diskSizeBytes: 100000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 80000, type: 'system' },
			],
		});

		expect(isPageValid(PAGE_IDS.DISK_SELECTION)).toBe(true);
		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).toEqual([]);
	});

	it('rejects a sub-8-GiB disk in manual mode', () => {
		installState.disk = { size: 7 * 1024 * mib } as unknown as typeof installState.disk;
		setManual({
			disk: '/dev/sda',
			diskSizeMB: 100000,
			diskSizeBytes: 100000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 80000, type: 'system' },
			],
		});

		const reasons = getPageValidationReasons(PAGE_IDS.DISK_SELECTION);
		expect(reasons.some((reason) => reason.includes('manual partitioning'))).toBe(true);
	});
});
