import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import DiskTopBar from './DiskTopBar.svelte';
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
	manualMode?: boolean;
	luksEnabled?: boolean;
	lvmEnabled?: boolean;
	onEnableLuks?: (() => void) | undefined;
	onDisableLuks?: (() => void) | undefined;
	onToggleLvm?: (() => void) | undefined;
};

function render(props: Props = {}): HTMLElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(DiskTopBar, {
		target,
		props: {
			pageIcon: 'x',
			disk,
			fileSystem: 'btrfs' as FileSystem,
			manualMode: props.manualMode ?? false,
			luksEnabled: props.luksEnabled ?? false,
			lvmEnabled: props.lvmEnabled ?? false,
			onEnableLuks: props.onEnableLuks,
			onDisableLuks: props.onDisableLuks,
			onToggleLvm: props.onToggleLvm,
			onSelectFileSystem: vi.fn(),
			onToggleMode: vi.fn()
		}
	});
	flushSync();
	return target;
}

function switches(root: HTMLElement): HTMLButtonElement[] {
	return Array.from(root.querySelectorAll<HTMLButtonElement>('button[role="switch"]'));
}

function toggleRow(root: HTMLElement): HTMLElement | null {
	return root.querySelector<HTMLElement>('.toggle-row');
}

function click(el: Element): void {
	el.dispatchEvent(new MouseEvent('click', { bubbles: true }));
	flushSync();
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
});

describe('DiskTopBar encryption + LVM row', () => {
	it('renders both switches in a single row', () => {
		const root = render({ onEnableLuks: vi.fn(), onToggleLvm: vi.fn() });
		const row = toggleRow(root);
		const [luks, lvm] = switches(root);

		expect(row).not.toBeNull();
		expect(luks).not.toBeUndefined();
		expect(lvm).not.toBeUndefined();
		// Same row container: the LVM switch sits next to the LUKS switch.
		expect(row!.contains(luks)).toBe(true);
		expect(row!.contains(lvm)).toBe(true);
	});

	it('keeps the LVM label attached to the LVM switch', () => {
		const root = render({ onEnableLuks: vi.fn(), onToggleLvm: vi.fn() });
		const [, lvm] = switches(root);
		const field = lvm.closest<HTMLElement>('.toggle-field');

		expect(field).not.toBeNull();
		expect(field!.textContent).toContain('LVM');
	});

	it('labels the row with the encryption label when both controls are shown', () => {
		const root = render({ onEnableLuks: vi.fn(), onToggleLvm: vi.fn() });
		// The row label is the grid cell immediately before the switch container.
		const rowLabel = toggleRow(root)!.previousElementSibling!.textContent?.trim();
		const labels = Array.from(root.querySelectorAll<HTMLElement>('.control-row-label')).map(
			(el) => el.textContent?.trim()
		);

		// Encryption owns the column-1 label (aligned with the mode/fs labels),
		// LVM sits beside its own switch.
		expect(rowLabel).toBe('Encrypt');
		expect(labels).toContain('LVM');
	});

	it('falls back to the LVM label when no encryption handlers are passed', () => {
		const root = render({ onToggleLvm: vi.fn() });
		const rowLabel = toggleRow(root)!.previousElementSibling!.textContent?.trim();

		expect(switches(root)).toHaveLength(1);
		expect(rowLabel).toBe('LVM');
	});

	it('calls the LUKS callbacks from the encryption switch', () => {
		const onEnableLuks = vi.fn();
		const onDisableLuks = vi.fn();
		const root = render({ onEnableLuks, onDisableLuks });
		const [luks] = switches(root);

		click(luks);
		expect(onEnableLuks).toHaveBeenCalledTimes(1);

		const enabled = render({
			luksEnabled: true,
			onEnableLuks,
			onDisableLuks
		});
		click(switches(enabled)[0]);
		expect(onDisableLuks).toHaveBeenCalledTimes(1);
	});

	it('calls the LVM callback from the LVM switch', () => {
		const onToggleLvm = vi.fn();
		const root = render({ onEnableLuks: vi.fn(), onToggleLvm });
		const [, lvm] = switches(root);

		click(lvm);
		expect(onToggleLvm).toHaveBeenCalledTimes(1);
	});

	it('shows the row in manual mode too', () => {
		const root = render({
			manualMode: true,
			onEnableLuks: vi.fn(),
			onToggleLvm: vi.fn()
		});
		const row = toggleRow(root)!;
		const [luks, lvm] = switches(root);

		expect(row.contains(luks)).toBe(true);
		expect(row.contains(lvm)).toBe(true);
	});

	it('omits the row entirely when neither control is wired up', () => {
		const root = render();
		expect(toggleRow(root)).toBeNull();
		expect(switches(root)).toHaveLength(0);
	});

	it('leaves the mode/fs controls untouched', () => {
		const root = render({ onEnableLuks: vi.fn(), onToggleLvm: vi.fn() });
		expect(root.querySelectorAll('.mode-pill')).toHaveLength(2);
		expect(root.querySelectorAll('.fs-pill')).toHaveLength(2);
	});
});
