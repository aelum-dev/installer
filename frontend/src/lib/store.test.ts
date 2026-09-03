import { describe, it, expect } from 'vitest';
import { isManualLayoutValid } from './store.svelte';
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
