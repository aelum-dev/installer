import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import RoleSelect from './RoleSelect.svelte';
import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
import type { PartitionType } from '$lib/types';

// Opening the menu focuses it on the next animation frame; await one frame so
// focus has landed before asserting.
async function nextFrame(): Promise<void> {
	await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
}

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(value: PartitionType = '', onChange = vi.fn()): HTMLDivElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(RoleSelect, { target, props: { value, onChange } });
	flushSync();
	return target;
}

function trigger(root: HTMLElement): HTMLButtonElement {
	return root.querySelector<HTMLButtonElement>('.role-trigger')!;
}

function menu(root: HTMLElement): HTMLElement | null {
	return root.querySelector<HTMLElement>('.role-menu');
}

function options(root: HTMLElement): HTMLButtonElement[] {
	return Array.from(root.querySelectorAll<HTMLButtonElement>('.role-option'));
}

function pressKey(el: Element, key: string): void {
	el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
	flushSync();
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
});

describe('RoleSelect keyboard navigation', () => {
	it('exposes a collapsed listbox on the trigger', () => {
		const root = render();
		const btn = trigger(root);
		expect(btn.getAttribute('aria-haspopup')).toBe('listbox');
		expect(btn.getAttribute('aria-expanded')).toBe('false');
		expect(menu(root)).toBeNull();
	});

	it('opens on ArrowDown and focuses the menu', async () => {
		const root = render();
		pressKey(trigger(root), 'ArrowDown');
		await nextFrame();

		expect(trigger(root).getAttribute('aria-expanded')).toBe('true');
		expect(document.activeElement).toBe(menu(root));
	});

	it('keeps the options out of the tab order', () => {
		const root = render();
		pressKey(trigger(root), 'ArrowDown');

		const tabbable = options(root).filter((o) => o.tabIndex >= 0);
		expect(tabbable).toHaveLength(0);
	});

	it('moves the active option with the arrow keys', async () => {
		const root = render();
		pressKey(trigger(root), 'ArrowDown');
		await nextFrame();

		const m = menu(root)!;
		const opts = options(root);
		expect(m.getAttribute('aria-activedescendant')).toBe(opts[0].id);

		pressKey(m, 'ArrowDown');
		expect(m.getAttribute('aria-activedescendant')).toBe(opts[1].id);

		pressKey(m, 'ArrowUp');
		expect(m.getAttribute('aria-activedescendant')).toBe(opts[0].id);

		// Clamped at the ends.
		pressKey(m, 'ArrowUp');
		expect(m.getAttribute('aria-activedescendant')).toBe(opts[0].id);
	});

	it('starts on the current selection when opened', async () => {
		const root = render(TYPE_SYSTEM);
		pressKey(trigger(root), 'ArrowDown');
		await nextFrame();

		const opts = options(root);
		expect(menu(root)!.getAttribute('aria-activedescendant')).toBe(opts[2].id);
	});

	it('selects the active option with Enter', async () => {
		const onChange = vi.fn();
		const root = render('', onChange);
		pressKey(trigger(root), 'ArrowDown');
		await nextFrame();

		const m = menu(root)!;
		pressKey(m, 'ArrowDown'); // active -> ESP
		pressKey(m, 'Enter');

		expect(onChange).toHaveBeenCalledWith(TYPE_ESP);
		expect(menu(root)).toBeNull();
		expect(document.activeElement).toBe(trigger(root));
	});

	it('closes and returns focus to the trigger on Escape', async () => {
		const root = render();
		pressKey(trigger(root), 'ArrowDown');
		await nextFrame();

		pressKey(menu(root)!, 'Escape');

		expect(trigger(root).getAttribute('aria-expanded')).toBe('false');
		expect(menu(root)).toBeNull();
		expect(document.activeElement).toBe(trigger(root));
	});

	it('closes and returns focus to the trigger on Tab (single tab stop)', async () => {
		const root = render();
		pressKey(trigger(root), 'ArrowDown');
		await nextFrame();

		pressKey(menu(root)!, 'Tab');

		expect(menu(root)).toBeNull();
		expect(document.activeElement).toBe(trigger(root));
	});
});
