/**
 * One app-themed tooltip for the whole wizard.
 *
 * Elements attach with `use:tooltip={text}`; a single fixed-position bubble
 * lives in App's TooltipLayer. Fixed positioning is the whole point: a bubble
 * inside a card or strip gets cut by the rail's scroll box, the disk list's
 * scroll box, or its card's rounding — and an anchored-to-the-left overflow of
 * a scroll container is not only invisible but unscrollable. The layer also
 * floats over the page and stays below dialog overlays (which own z-index
 * 1000+), so a hint can never outlive the dialog it hid under.
 */
export const tooltipState = $state({
	text: '',
	anchor: null as HTMLElement | null,
	/**
	 * Where the pointer was when the hint opened, in viewport coordinates —
	 * the point the bubble hangs from. Null when keyboard focus opened it,
	 * where there is no pointer to answer for, and the anchor's own edge is
	 * the only position there is.
	 */
	cursor: null as { x: number; y: number } | null,
});

/**
 * How long the pointer must rest on a node (or a control keep focus) before
 * its hint appears. An instant tooltip fires on the way past: the disk rows,
 * the strip's segments and the editor's paths are crossed in milliseconds and
 * each crossing would flash a bubble. 1s is the native `title` measure, so the
 * app behaves the way the widget the person expects does.
 */
export const TOOLTIP_SHOW_DELAY_MS = 1000;

/**
 * Svelte 5 action: shows the shared tooltip while the node is hovered or
 * focused, like a native title — but themed, wrapped, and unclippable.
 * An empty text (e.g. the Manual pill once a disk IS selected) means no
 * tooltip at all.
 *
 * A pointer that waited the second out gets its hint under the pointer, at the
 * spot it had reached when the hint opened, and the bubble then stays put
 * rather than chasing the mouse. Keyboard focus carries no pointer, so that
 * path hangs the bubble from the element itself.
 */
export function tooltip(node: HTMLElement, text: string | null | undefined) {
	let current = text ?? '';
	let timer: ReturnType<typeof setTimeout> | null = null;
	// The pointer's latest position, held OUTSIDE the state: writing it on
	// every mousemove would re-render the tooltip layer as fast as the mouse
	// moves. It crosses into tooltipState once, when the hint opens.
	let pointer: { x: number; y: number } | null = null;

	function remember(event: MouseEvent): void {
		pointer = { x: event.clientX, y: event.clientY };
	}

	function open(): void {
		tooltipState.text = current;
		tooltipState.anchor = node;
		tooltipState.cursor = pointer;
	}

	function arm(): void {
		// One pending timer per node; an already-open tooltip keeps its text
		// instead of restarting the wait.
		if (!current || timer !== null || tooltipState.anchor === node) return;
		timer = setTimeout(() => {
			timer = null;
			// current is only ever emptied through hide(), which cancels this —
			// the guard is the cheap guarantee, not the mechanism.
			if (current) open();
		}, TOOLTIP_SHOW_DELAY_MS);
	}

	function showFromPointer(event: MouseEvent): void {
		remember(event);
		arm();
	}

	function showFromKeyboard(): void {
		// Nothing to hang the bubble from, and the last pointer position is not
		// this interaction's: focus hangs it from the element.
		pointer = null;
		arm();
	}

	function hide(): void {
		// Leaving before the wait is over cancels it: the pointer moved on, so
		// the hint the person never waited for must not appear behind them.
		if (timer !== null) {
			clearTimeout(timer);
			timer = null;
		}
		if (tooltipState.anchor === node) {
			tooltipState.text = '';
			tooltipState.anchor = null;
			tooltipState.cursor = null;
		}
	}

	node.addEventListener('mouseenter', showFromPointer);
	node.addEventListener('mousemove', remember);
	node.addEventListener('mouseleave', hide);
	node.addEventListener('focus', showFromKeyboard);
	node.addEventListener('blur', hide);

	return {
		update(next: string | null | undefined): void {
			current = next ?? '';
			if (!current) hide();
			else if (tooltipState.anchor === node) tooltipState.text = current;
		},
		destroy(): void {
			hide();
			node.removeEventListener('mouseenter', showFromPointer);
			node.removeEventListener('mousemove', remember);
			node.removeEventListener('mouseleave', hide);
			node.removeEventListener('focus', showFromKeyboard);
			node.removeEventListener('blur', hide);
		},
	};
}
