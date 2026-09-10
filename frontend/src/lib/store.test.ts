import { describe, it, expect, afterEach } from 'vitest';
import { isManualLayoutValid, getPageValidationReasons, installState } from './store.svelte';
import { PAGE_IDS } from './types';
import type { ManualLayout } from './types';

const mib = 1024 * 1024;

function baseLayout(over: Partial<ManualLayout> = {}): ManualLayout {
	return {
		disk: '/dev/sda',
		diskSizeMB: 100000,
		diskSizeBytes: 100000 * mib,
		partitions: [
			{ index: 0, path: '/dev/sda1', sizeMB: 128, sizeBytes: 128 * mib, type: 'esp' },
			{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
		],
		freeMB: 0,
		...over,
	};
}

describe('isManualLayoutValid', () => {
	it('accepts an ESP >= 128 MiB and a System >= 8 GiB', () => {
		expect(isManualLayoutValid(baseLayout())).toBe(true);
	});

	it('rejects a System partition below 8 GiB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 128, sizeBytes: 128 * mib, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 1, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects an ESP below 128 MiB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 64, sizeBytes: 64 * mib, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('accepts a brand-new System using all remaining space (sizeMB 0)', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 128, sizeBytes: 128 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 0, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(true);
	});

	it('rejects a brand-new System explicit size below 8 GiB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 128, sizeBytes: 128 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 4096, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects a layout with no System partition', () => {
		const layout = baseLayout({
			partitions: [{ index: 0, path: '/dev/sda1', sizeMB: 128, type: 'esp' }],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects null', () => {
		expect(isManualLayoutValid(null)).toBe(false);
	});
});

// Regression: the free-space check must compare new partitions against the
// headroom left by *existing* partitions, not against the space remaining after
// the new partitions are subtracted (which double-counts them and rejects any
// plan whose new partitions are a large share of the disk).
describe('getPageValidationReasons — new-partition free space', () => {
	const mib = 1024 * 1024;

	function setManual(layout: ManualLayout) {
		installState.manualMode = true;
		installState.diskId = layout.disk;
		installState.fileSystem = 'ext4';
		installState.manualLayout = layout;
	}

	afterEach(() => {
		installState.manualMode = false;
		installState.manualLayout = null;
	});

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
			freeMB: 0,
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
			freeMB: 0,
		});
		// Over-allocation is still rejected (this also trips the disk-size check,
		// since new + existing > disk), so just assert it does not silently pass.
		expect(getPageValidationReasons(PAGE_IDS.DISK_SELECTION)).not.toEqual([]);
	});
});
