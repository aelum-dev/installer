import { describe, it, expect, afterEach, vi } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import ConfirmDialog from './ConfirmDialog.svelte';

// jsdom has no layout and the modal focuses itself on the next animation frame.
// Awaiting one frame after a state change keeps the order the browser uses:
// render -> rAF callback -> focus.
async function nextFrame(): Promise<void> {
	await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
}

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(props: Record<string, unknown> = {}): HTMLDivElement {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(ConfirmDialog, {
		target,
		props: { show: true, title: 'Confirm', message: 'Are you sure?', ...props },
	});
	flushSync();
	return target;
}

afterEach(() => {
	if (app) {
		unmount(app);
		app = null;
	}
	target?.remove();
});

// jsdom doesn't implement native Tab movement, so tests assert the trap's own
// focus moves and whether it claimed (preventDefault) the key.
function pressKey(el: Element, init: KeyboardEventInit): KeyboardEvent {
	const event = new KeyboardEvent('keydown', {
		bubbles: true,
		cancelable: true,
		...init,
	});
	el.dispatchEvent(event);
	flushSync();
	return event;
}

describe('Dialog focus trap', () => {
	it('moves focus into the dialog on open', async () => {
		const root = render();
		await nextFrame();
		expect(document.activeElement).toBe(root.querySelector('[role="dialog"]'));
	});

	it('wraps Tab from the last control back to the first', () => {
		const root = render();
		const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>('[role="dialog"] button'));
		const cancel = buttons[0];
		const confirm = buttons[buttons.length - 1];

		confirm.focus();
		const event = pressKey(confirm, { key: 'Tab' });

		expect(event.defaultPrevented).toBe(true);
		expect(document.activeElement).toBe(cancel);
	});

	it('wraps Shift+Tab from the first control to the last', () => {
		const root = render();
		const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>('[role="dialog"] button'));
		const cancel = buttons[0];
		const confirm = buttons[buttons.length - 1];

		cancel.focus();
		const event = pressKey(cancel, { key: 'Tab', shiftKey: true });

		expect(event.defaultPrevented).toBe(true);
		expect(document.activeElement).toBe(confirm);
	});

	it('sends Tab from the dialog container to the first control', () => {
		const root = render();
		const dialog = root.querySelector<HTMLElement>('[role="dialog"]')!;

		dialog.focus();
		pressKey(dialog, { key: 'Tab' });

		expect(document.activeElement).toBe(root.querySelector('.btn-cancel'));
	});

	it('leaves mid-dialog Tab to the browser (no preventDefault)', () => {
		const root = render();
		const cancel = root.querySelector<HTMLElement>('.btn-cancel')!;

		// Focus is on the first control; forward Tab is not a wrap, so the trap
		// must let the browser move on to the next control.
		cancel.focus();
		const event = pressKey(cancel, { key: 'Tab' });

		expect(event.defaultPrevented).toBe(false);
		expect(document.activeElement).toBe(cancel);
	});

	it('cancels once on Escape (does not bubble to the overlay)', () => {
		const onCancel = vi.fn();
		const root = render({ onCancel });
		const dialog = root.querySelector<HTMLElement>('[role="dialog"]')!;

		pressKey(dialog, { key: 'Escape' });

		expect(onCancel).toHaveBeenCalledTimes(1);
	});
});
