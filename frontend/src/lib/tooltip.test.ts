import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
	tooltip,
	tooltipState,
	dismissTooltip,
	withRestoredFocus,
	TOOLTIP_SHOW_DELAY_MS,
} from '$lib/tooltip.svelte';

/**
 * The tooltip action bound to a real host node in the DOM. The action is a
 * plain function, so these tests drive it exactly the way a component's
 * `use:tooltip` does — no wrapper component needed.
 */
function host(text = 'Encrypt the root partition'): { node: HTMLElement; destroy: () => void } {
	const node = document.createElement('button');
	node.textContent = text;
	document.body.appendChild(node);
	const action = tooltip(node, text);
	return { node, destroy: () => action.destroy() };
}

function enter(node: HTMLElement): void {
	node.dispatchEvent(new MouseEvent('mouseenter'));
}

function leave(node: HTMLElement): void {
	node.dispatchEvent(new MouseEvent('mouseleave'));
}

function focus(node: HTMLElement): void {
	node.dispatchEvent(new FocusEvent('focus'));
}

function blur(node: HTMLElement): void {
	node.dispatchEvent(new FocusEvent('blur'));
}

let cleanup: (() => void) | null = null;

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	cleanup?.();
	cleanup = null;
	tooltipState.text = '';
	tooltipState.anchor = null;
	tooltipState.cursor = null;
	vi.useRealTimers();
	document.body.innerHTML = '';
});

describe('dismissTooltip', () => {
	it('ends the open hint when a dialog takes the screen', () => {
		const { node, destroy } = host();
		cleanup = destroy;
		enter(node);
		vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS);
		expect(tooltipState.text).not.toBe('');

		dismissTooltip();

		expect(tooltipState.text).toBe('');
		expect(tooltipState.anchor).toBeNull();
		expect(tooltipState.cursor).toBeNull();
	});

	it('voids a show still being waited out, so it cannot open behind the dialog and outlive it', () => {
		const { node, destroy } = host();
		cleanup = destroy;
		// The pointer rests one tick short of the wait when the dialog opens:
		// without the void the timer would fire mid-dialog and the bubble
		// would resurface above the page — anchored to a node the pointer
		// never enters again — the moment the dialog hides itself.
		enter(node);
		vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS - 1);
		dismissTooltip();
		vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS);

		expect(tooltipState.text).toBe('');
		expect(tooltipState.anchor).toBeNull();
	});

	it('leaves a later hover of its own free to open', () => {
		const { node, destroy } = host();
		cleanup = destroy;
		enter(node);
		vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS);
		dismissTooltip();

		// The person comes back to the control after the dialog: a fresh
		// wait of its own arms normally.
		enter(node);
		vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS);
		expect(tooltipState.text).not.toBe('');
	});
});

describe('withRestoredFocus', () => {
	it('a focus the dialog hands back opens no hint', () => {
		const { node, destroy } = host();
		cleanup = destroy;
		// The dialog hid itself (commit) and returned focus to its trigger:
		// no key and no pointer asked for the trigger a second later, so the
		// hint must stay closed — that bubble had nothing left to close it.
		withRestoredFocus(() => focus(node));
		vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS);

		expect(tooltipState.text).toBe('');
		expect(tooltipState.anchor).toBeNull();
	});

	it('a focus the person brings still gets its hint', () => {
		const { node, destroy } = host();
		cleanup = destroy;
		focus(node);
		vi.advanceTimersByTime(TOOLTIP_SHOW_DELAY_MS);
		expect(tooltipState.text).not.toBe('');

		blur(node);
		expect(tooltipState.text).toBe('');
	});
});
