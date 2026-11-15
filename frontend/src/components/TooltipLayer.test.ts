import { describe, it, expect, afterEach } from 'vitest';
import { mount, unmount, flushSync } from 'svelte';
import TooltipLayer from './TooltipLayer.svelte';
import { tooltipState } from '$lib/tooltip.svelte';

/**
 * The bubble's placement, against the window main.go fixes the app at.
 * The browser can only ever show the below-the-cursor branch — every tooltip
 * host in the wizard leaves room under itself — so the flip and the two edge
 * clamps are pinned here, where the geometry can be set deliberately.
 *
 * jsdom lays nothing out, so the bubble's box and the window's size are
 * declared rather than measured: 200x40 is what the CSS gives a wrapped hint,
 * 900x600 what the app ships. Vitest gives every file its own environment, so
 * these stand for the run.
 */
const BUBBLE_W = 200;
const BUBBLE_H = 40;
Object.defineProperty(HTMLElement.prototype, 'offsetWidth', {
	configurable: true,
	get: () => BUBBLE_W,
});
Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
	configurable: true,
	get: () => BUBBLE_H,
});
Object.defineProperty(window, 'innerWidth', { configurable: true, value: 900 });
Object.defineProperty(window, 'innerHeight', { configurable: true, value: 600 });

/** A 40x20 anchor whose top-left corner sits at (x, top). */
function anchorAt(x: number, top: number, w = 40): HTMLElement {
	const el = document.createElement('div');
	el.getBoundingClientRect = () => ({
		x,
		y: top,
		left: x,
		top,
		right: x + w,
		bottom: top + 20,
		width: w,
		height: 20,
		toJSON: () => ({}),
	});
	return el;
}

let app: ReturnType<typeof mount> | null = null;
let target: HTMLDivElement;

function render(): void {
	target = document.createElement('div');
	document.body.appendChild(target);
	app = mount(TooltipLayer, { target });
	flushSync();
}

function open(anchor: HTMLElement, cursor: { x: number; y: number } | null): HTMLElement {
	tooltipState.text = 'a hint';
	tooltipState.anchor = anchor;
	tooltipState.cursor = cursor;
	flushSync();
	const tip = target.querySelector<HTMLElement>('[role="tooltip"]');
	if (!tip) throw new Error('the layer rendered no bubble');
	return tip;
}

function arrowOf(tip: HTMLElement): string {
	const arrow = tip.querySelector<HTMLElement>('.app-tip-arrow');
	return arrow?.getAttribute('style') ?? '';
}

afterEach(() => {
	tooltipState.text = '';
	tooltipState.anchor = null;
	tooltipState.cursor = null;
	flushSync();
	if (app) {
		unmount(app);
		app = null;
	}
	document.body.replaceChildren();
});

describe('TooltipLayer placement', () => {
	it('hangs centred under the cursor that waited the hint out', () => {
		render();
		const tip = open(anchorAt(100, 100), { x: 400, y: 250 });

		expect(tip.style.left).toBe('300px'); // cursor 400 − half the bubble
		expect(tip.style.top).toBe('264px'); // cursor 250 + the 14px hang
		expect(tip.className).not.toContain('app-tip--above');
		expect(arrowOf(tip)).toContain('left: 100px'); // aims back up at the cursor
	});

	it('hangs from the anchor when keyboard focus opened it', () => {
		render();
		const tip = open(anchorAt(500, 300, 120), null);

		expect(tip.style.left).toBe('460px'); // anchor centre 560 − half the bubble
		expect(tip.style.top).toBe('334px'); // anchor bottom 320 + the 14px hang
		expect(tip.className).not.toContain('app-tip--above');
	});

	it('flips above and points down when there is no room below', () => {
		render();
		const tip = open(anchorAt(400, 555), { x: 420, y: 560 });

		// 560 + 14 + 40 would pass the window's bottom margin, so the bubble
		// hangs above the cursor instead and the arrow turns to point down.
		expect(tip.style.top).toBe('506px'); // 560 − 14 − the bubble's height
		expect(tip.className).toContain('app-tip--above');
	});

	it('clamps inside the viewport and keeps the arrow on the bubble', () => {
		render();

		const right = open(anchorAt(800, 100), { x: 880, y: 120 });
		expect(right.style.left).toBe('692px'); // window 900 − 8 margin − the bubble's width
		// The cursor now lies outside the clamped bubble: the arrow stays on it.
		expect(arrowOf(right)).toContain('left: 186px');

		const left = open(anchorAt(0, 100), { x: 10, y: 120 });
		expect(left.style.left).toBe('8px');
		expect(arrowOf(left)).toContain('left: 14px');
	});
});
