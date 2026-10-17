import { describe, it, expect } from 'vitest';
import {
	computeFreeMB,
	computeUsedMB,
	isManualLayoutValid,
	manualLayoutIssues,
} from './manual-layout';
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

	it('rejects new partitions that do not fit the existing headroom', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 99500, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});
});

describe('manual layout geometry', () => {
	it('sums existing byte sizes and new MiB sizes in computeUsedMB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 128, sizeBytes: 128 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 4096, type: 'system' },
			],
		});
		expect(computeUsedMB(layout)).toBe(4224);
	});

	it('reserves both GPT regions when reporting free space', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 128, sizeBytes: 128 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 0, type: 'system' },
			],
		});
		// 100000 MiB minus 128 MiB minus the 1 MiB GPT prefix and 128 KiB backup.
		expect(computeFreeMB(layout)).toBe(99870);
	});

	it('reports structured issues in the same order the modal renders them', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 64, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 1, type: 'system' },
			],
		});
		expect(manualLayoutIssues(layout).map((issue) => issue.kind)).toEqual([
			'espTooSmall',
			'systemTooSmall',
		]);
	});
});
