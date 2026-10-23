import { describe, it, expect } from 'vitest';
import {
	addPartition,
	computeFreeMB,
	computeUsedMB,
	isManualLayoutValid,
	manualLayoutIssues,
	nextPartitionIndex,
	normalizeManualLayout,
	removePartition,
	withPartitionType,
} from './manual-layout';
import type { ManualLayout } from './types';

const mib = 1024 * 1024;

function baseLayout(over: Partial<ManualLayout> = {}): ManualLayout {
	return {
		disk: '/dev/sda',
		diskSizeMB: 100000,
		diskSizeBytes: 100000 * mib,
		partitions: [
			{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
			{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
		],
		...over,
	};
}

describe('isManualLayoutValid', () => {
	it('accepts an ESP >= 512 MiB and a System >= 8 GiB', () => {
		expect(isManualLayoutValid(baseLayout())).toBe(true);
	});

	it('accepts a 7680 MiB System partition (8 GiB total - 512 MiB ESP)', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 7680, sizeBytes: 7680 * mib, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(true);
	});

	it('rejects a System partition below 7680 MiB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 7679, sizeBytes: 7679 * mib, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects a 256 MiB existing ESP (Boot is always formatted)', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 256, sizeBytes: 256 * mib, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects an existing ESP below 512 MiB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 99, sizeBytes: 99 * mib, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects a brand-new ESP below 512 MiB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '', sizeMB: 256, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('accepts a brand-new System using all remaining space (sizeMB 0)', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 0, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(true);
	});

	it('rejects a brand-new System explicit size below 8 GiB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 4096, type: 'system' },
			],
		});
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects a layout with no System partition', () => {
		const layout = baseLayout({
			partitions: [{ index: 0, path: '/dev/sda1', sizeMB: 512, type: 'esp' }],
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

describe('use-all-remaining rows respect the role floor', () => {
	it('rejects a brand-new ESP whose resolved free space is below 512 MiB', () => {
		const layout = baseLayout({
			diskSizeMB: 9000,
			diskSizeBytes: 9000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda2', sizeMB: 8700, sizeBytes: 8700 * mib, type: 'system' },
				{ index: 1, path: '', sizeMB: 0, type: 'esp' },
			],
		});
		expect(manualLayoutIssues(layout)).toContainEqual({ kind: 'espTooSmall', sizeMB: 298 });
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('rejects a brand-new System whose resolved free space is below 7680 MiB', () => {
		const layout = baseLayout({
			diskSizeMB: 8000,
			diskSizeBytes: 8000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 0, type: 'system' },
			],
		});
		expect(manualLayoutIssues(layout)).toContainEqual({ kind: 'systemTooSmall', sizeMB: 7486, minMB: 7680 });
		expect(isManualLayoutValid(layout)).toBe(false);
	});

	it('accepts a use-all ESP when the resolved free space meets the floor', () => {
		const layout = baseLayout({
			diskSizeMB: 9000,
			diskSizeBytes: 9000 * mib,
			partitions: [
				{ index: 0, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
				{ index: 1, path: '', sizeMB: 0, type: 'esp' },
			],
		});
		expect(manualLayoutIssues(layout)).toEqual([]);
		expect(isManualLayoutValid(layout)).toBe(true);
	});
});

describe('manual layout geometry', () => {
	it('sums existing byte sizes and new MiB sizes in computeUsedMB', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 4096, type: 'system' },
			],
		});
		expect(computeUsedMB(layout)).toBe(4608);
	});

	it('reserves both GPT regions when reporting free space', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '', sizeMB: 0, type: 'system' },
			],
		});
		// 100000 MiB minus 512 MiB minus the 1 MiB GPT prefix and 128 KiB backup.
		expect(computeFreeMB(layout)).toBe(99486);
	});

	it('reports structured issues in the same order the modal renders them', () => {
		const layout = baseLayout({
			partitions: [
				// Both the tiny ESP and the tiny System violate their floors.
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

describe('manual layout guards', () => {
	it('rejects a brand-new partition with no role', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, sizeBytes: 512 * mib, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
				{ index: 2, path: '', sizeMB: 1024, type: '' },
			],
		});
		expect(manualLayoutIssues(layout).map((issue) => issue.kind)).toContain('newPartNoType');
	});

	it('rejects an existing sub-1-MiB ESP without treating it as use-remaining', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 0, sizeBytes: 512 * 1024, type: 'esp' },
				{ index: 1, path: '/dev/sda2', sizeMB: 8192, sizeBytes: 8192 * mib, type: 'system' },
			],
		});
		// Boot partitions are always formatted, so the floor applies. The tiny ESP
		// must not be mistaken for a brand-new "use all remaining space" row.
		expect(manualLayoutIssues(layout)).toEqual([{ kind: 'espTooSmall', sizeMB: 0 }]);
	});
});

describe('empty disk payload', () => {
	// Go marshals a nil slice as `null`, so a fresh, unpartitioned disk arrives
	// with `partitions: null` even though the type says array.
	function emptyLayout(): ManualLayout {
		return {
			disk: '/dev/vda',
			diskSizeMB: 20480,
			diskSizeBytes: 20480 * mib,
			partitions: null as unknown as ManualLayout['partitions'],
		};
	}

	it('normalizes a null partitions list to an empty array', () => {
		expect(normalizeManualLayout(emptyLayout()).partitions).toEqual([]);
	});

	it('computes used/free space instead of throwing', () => {
		expect(computeUsedMB(emptyLayout())).toBe(0);
		expect(computeFreeMB(emptyLayout())).toBeGreaterThan(20000);
	});

	it('reports missing roles instead of crashing the manual page', () => {
		expect(() => manualLayoutIssues(emptyLayout())).not.toThrow();
		expect(manualLayoutIssues(emptyLayout()).map((issue) => issue.kind)).toEqual(
			expect.arrayContaining(['espCount', 'systemCount']),
		);
	});
});

describe('immutable layout updates', () => {
	it('nextPartitionIndex returns max index + 1', () => {
		const layout = baseLayout({
			partitions: [
				{ index: 0, path: '/dev/sda1', sizeMB: 512, type: 'esp' },
				{ index: 3, path: '/dev/sda3', sizeMB: 1024, type: 'system' },
			],
		});
		expect(nextPartitionIndex(layout)).toBe(4);
	});

	it('nextPartitionIndex returns 0 for an empty plan', () => {
		expect(nextPartitionIndex(baseLayout({ partitions: [] }))).toBe(0);
	});

	it('addPartition appends without mutating the original', () => {
		const original = baseLayout({ partitions: [] });
		const next = addPartition(original, { index: 0, path: '', sizeMB: 1024, type: 'system' });
		expect(next.partitions).toHaveLength(1);
		expect(original.partitions).toHaveLength(0);
	});

	it('removePartition moves the row to deletedPartitions without mutating the original', () => {
		const original = baseLayout();
		const next = removePartition(original, 1);
		expect(next.partitions.map((p) => p.index)).toEqual([0]);
		expect(next.deletedPartitions?.map((p) => p.index)).toEqual([1]);
		expect(original.partitions.map((p) => p.index)).toEqual([0, 1]);
	});

	it('removePartition is a no-op for an unknown index', () => {
		const original = baseLayout();
		expect(removePartition(original, 99)).toBe(original);
	});

	it('withPartitionType changes only the target row', () => {
		const original = baseLayout();
		const next = withPartitionType(original, 1, '');
		expect(next.partitions[1].type).toBe('');
		expect(next.partitions[0].type).toBe('esp');
		expect(original.partitions[1].type).toBe('system');
	});
});
