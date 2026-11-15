import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import StorageOptionsCard from './StorageOptionsCard.svelte';
import { tooltipState, TOOLTIP_SHOW_DELAY_MS } from '$lib/tooltip.svelte';
import type { DiskInfo, FileSystem } from '$lib/types';

const disk: DiskInfo = {
	name: 'sda',
	path: '/dev/sda',
	size: 20_000_000_000,
	sizeFormatted: '20.0 GiB',
	type: 'disk',
	model: 'Test Disk',
	vendor: '',
	rotational: true,
	readonly: false,
	fstype: null,
	mountpoint: null,
	serial: '',
	trans: 'sata',
	isRemovable: false,
	partitionTable: 'gpt',
};

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

type Props = {
	disk?: DiskInfo | null;
	manualMode?: boolean;
	luksEnabled?: boolean;
	lvmEnabled?: boolean;
	onEnableLuks?: (() => void) | undefined;
	onDisableLuks?: (() => void) | undefined;
	onToggleLvm?: (() => void) | undefined;
	onSelectFileSystem?: (fs: FileSystem) => void;
	onToggleMode?: (mode: 'automatic' | 'manual') => void;
};

function render(props: Props = {}): HTMLElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(StorageOptionsCard, {
		target,
		props: {
			disk: props.disk === undefined ? disk : props.disk,
			fileSystem: 'btrfs' as FileSystem,
			manualMode: props.manualMode ?? false,
			luksEnabled: props.luksEnabled ?? false,
			lvmEnabled: props.lvmEnabled ?? false,
			onEnableLuks: props.onEnableLuks,
			onDisableLuks: props.onDisableLuks,
			onToggleLvm: props.onToggleLvm,
			onSelectFileSystem: props.onSelectFileSystem ?? vi.fn(),
			onToggleMode: props.onToggleMode ?? vi.fn()
		}
	});
	flushSync();
	return target;
}

function switches(root: HTMLElement): HTMLButtonElement[] {
	return Array.from(root.querySelectorAll<HTMLButtonElement>('button[role="switch"]'));
}

function rows(root: HTMLElement): HTMLElement[] {
	return Array.from(root.querySelectorAll<HTMLElement>('.option-row'));
}

function rowLabel(row: Element): string | undefined {
	const label = row.querySelector<HTMLElement>('.option-label');
	if (!label) return undefined;
	// The label's second line (.option-sub) is explanation, not the row's name.
	const clone = label.cloneNode(true) as HTMLElement;
	clone.querySelector('.option-sub')?.remove();
	return clone.textContent?.trim();
}

function click(el: Element): void {
	el.dispatchEvent(new MouseEvent('click', { bubbles: true }));
	flushSync();
}

function hover(el: Element, type: string): void {
	el.dispatchEvent(new MouseEvent(type));
	flushSync();
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
});

describe('StorageOptionsCard rows', () => {
	it('always renders the filesystem and partitioning rows', () => {
		const root = render();
		expect(root.querySelectorAll('.fs-pill')).toHaveLength(2);
		expect(root.querySelectorAll('.mode-pill')).toHaveLength(2);
	});

	it('renders one labeled row per wired switch', () => {
		const root = render({ onEnableLuks: vi.fn(), onToggleLvm: vi.fn() });
		const [luks, lvm] = switches(root);
		const luksRow = luks.closest<HTMLElement>('.option-row')!;
		const lvmRow = lvm.closest<HTMLElement>('.option-row')!;

		// Each switch owns its own row and its own label (no shared row label).
		expect(luksRow).not.toBe(lvmRow);
		expect(rowLabel(luksRow)).toBe('Encrypt');
		expect(luksRow.contains(luks)).toBe(true);
		expect(rowLabel(lvmRow)).toBe('LVM');
		expect(lvmRow.contains(lvm)).toBe(true);
	});

	it('omits the encryption and LVM rows when neither control is wired up', () => {
		const root = render();
		expect(switches(root)).toHaveLength(0);
		// Filesystem + Partitioning only.
		expect(rows(root)).toHaveLength(2);
	});

	it('keeps the row order fixed in both modes: shapers first, wrappers last', () => {
		for (const manualMode of [false, true]) {
			const root = render({
				manualMode,
				onEnableLuks: vi.fn(),
				onToggleLvm: vi.fn()
			});
			const labels = rows(root).map(rowLabel);
			// The two rows that shape the layout come first; the two wrapping
			// switches share one band under them — the order never moves.
			expect(labels).toEqual(['Filesystem', 'Partitioning', 'Encrypt', 'LVM']);
			const wraps = root.querySelector('.wraps')!;
			expect(wraps).not.toBeNull();
			expect(wraps.querySelectorAll('.option-row')).toHaveLength(2);
		}
	});

	it('calls the LUKS callbacks from the encryption switch', () => {
		const onEnableLuks = vi.fn();
		const onDisableLuks = vi.fn();
		const root = render({ onEnableLuks, onDisableLuks });
		click(switches(root)[0]);
		expect(onEnableLuks).toHaveBeenCalledTimes(1);

		const enabled = render({ luksEnabled: true, onEnableLuks, onDisableLuks });
		click(switches(enabled)[0]);
		expect(onDisableLuks).toHaveBeenCalledTimes(1);
	});

	it('calls the LVM callback from the LVM switch', () => {
		const onToggleLvm = vi.fn();
		const root = render({ onToggleLvm });
		click(switches(root)[0]);
		expect(onToggleLvm).toHaveBeenCalledTimes(1);
	});

	it('calls the filesystem callback from the filesystem pills', () => {
		const onSelectFileSystem = vi.fn();
		const root = render({ onSelectFileSystem });
		const ext4 = Array.from(root.querySelectorAll<HTMLElement>('.fs-pill')).find(
			(el) => el.textContent?.trim() === 'ext4'
		)!;
		click(ext4);
		expect(onSelectFileSystem).toHaveBeenCalledWith('ext4');
	});
});

describe('StorageOptionsCard partitioning segments', () => {
	function modePills(root: HTMLElement): HTMLButtonElement[] {
		return Array.from(root.querySelectorAll<HTMLButtonElement>('.mode-pill'));
	}

	it('disables the Manual segment until a disk is selected', () => {
		const root = render({ disk: null });
		const [, manual] = modePills(root);
		expect(manual.disabled).toBe(true);
	});

	it('shows the "select a disk first" tooltip while hovering the disabled segment', () => {
		const root = render({ disk: null });
		const [, manual] = modePills(root);

		// Timers only after the mount: the component is built with real ones.
		vi.useFakeTimers();
		try {
			// The wait is the feature: the bubble appears only once the pointer
			// has actually rested on the control.
			hover(manual, 'mouseenter');
			vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS - 1);
			flushSync();
			expect(tooltipState.text).toBe('');
			expect(tooltipState.anchor).toBeNull();

			vi.advanceTimersByTime(1);
			flushSync();
			expect(tooltipState.text).toContain('Select a disk before switching to manual mode.');
			expect(tooltipState.anchor).toBe(manual);

			hover(manual, 'mouseleave');
			expect(tooltipState.text).toBe('');
			expect(tooltipState.anchor).toBeNull();
		} finally {
			vi.useRealTimers();
		}
	});

	it('never shows a tooltip for a hover the person did not wait out', () => {
		const root = render({ disk: null });
		const [, manual] = modePills(root);

		vi.useFakeTimers();
		try {
			// Crossing a control on the way to something else must not open a
			// bubble afterwards, so leaving cancels the pending show.
			hover(manual, 'mouseenter');
			vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS - 1);
			hover(manual, 'mouseleave');
			vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS);
			flushSync();
			expect(tooltipState.text).toBe('');
			expect(tooltipState.anchor).toBeNull();
		} finally {
			vi.useRealTimers();
		}
	});

	it('enables the Manual segment once a disk is selected and switches modes', () => {
		const onToggleMode = vi.fn();
		const root = render({ onToggleMode });
		const [automatic, manual] = modePills(root);

		expect(manual.disabled).toBe(false);
		click(manual);
		expect(onToggleMode).toHaveBeenCalledWith('manual');

		click(automatic);
		expect(onToggleMode).toHaveBeenCalledWith('automatic');
	});

	it('marks the active mode segment', () => {
		const auto = render();
		expect(auto.querySelectorAll('.mode-pill.selected')).toHaveLength(1);
		expect(auto.querySelectorAll('.mode-pill')[0].className).toContain('selected');

		const manual = render({ manualMode: true });
		expect(manual.querySelectorAll('.mode-pill')[1].className).toContain('selected');
	});
});
