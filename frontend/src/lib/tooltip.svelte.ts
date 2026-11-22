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
 * Count of times the app itself ended the hint it was showing — a dialog
 * taking the screen. Not UI state: armed timers compare against it, so a
 * dismiss between arming and firing cancels the show. The hover or focus a
 * hint belongs to is over the moment a dialog opens, and a bubble that
 * still opened behind the overlay would resurface above the page — stuck,
 * with no pointer on its anchor and no blur coming — when the dialog hides
 * itself. Only a dismiss can cancel other nodes' pending shows; leaving a
 * node cancels that node's own through hide().
 */
let dismissals = 0;

/** Ends the open hint now and voids every pending show. Dialogs call it. */
export function dismissTooltip(): void {
	dismissals += 1;
	tooltipState.text = '';
	tooltipState.anchor = null;
	tooltipState.cursor = null;
}

/**
 * True only while the app moves focus by itself — a dialog handing focus
 * back to the control that opened it (Dialog.svelte). A hint must not
 * reopen for that focus: the interaction it would answer for was the
 * dialog's, and it just ended. A bubble waking a second after the dialog
 * hid itself, hung from an element no key is on its way to, has no
 * pointer to carry it off and no blur until the person clicks something —
 * a tooltip stuck in place. A focus the person brings still gets its hint.
 */
let focusRestored = false;

/** Runs a programmatic focus() so it cannot wake a tooltip. */
export function withRestoredFocus(restore: () => void): void {
	focusRestored = true;
	try {
		restore();
	} finally {
		focusRestored = false;
	}
}

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
		const armedAt = dismissals;
		timer = setTimeout(() => {
			timer = null;
			// current is only ever emptied through hide(), which cancels this —
			// the guard is the cheap guarantee, not the mechanism. A dismiss
			// since arming means a dialog took the screen: the wait this hint
			// was the end of is over, and opening now would only hide this
			// bubble under the overlay to outlive it.
			if (current && armedAt === dismissals) open();
		}, TOOLTIP_SHOW_DELAY_MS);
	}

	function showFromPointer(event: MouseEvent): void {
		remember(event);
		arm();
	}

	function showFromKeyboard(): void {
		// A focus the app handed over — a dialog closing — carries no new
		// interaction, so it arms nothing (see withRestoredFocus).
		if (focusRestored) return;
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
