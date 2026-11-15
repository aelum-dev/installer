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
});

/**
 * Svelte 5 action: shows the shared tooltip while the node is hovered or
 * focused, like a native title — but themed, wrapped, and unclippable.
 * An empty text (e.g. the Manual pill once a disk IS selected) means no
 * tooltip at all.
 */
export function tooltip(node: HTMLElement, text: string | null | undefined) {
	let current = text ?? '';

	function show(): void {
		if (!current) return;
		tooltipState.text = current;
		tooltipState.anchor = node;
	}

	function hide(): void {
		if (tooltipState.anchor === node) {
			tooltipState.text = '';
			tooltipState.anchor = null;
		}
	}

	node.addEventListener('mouseenter', show);
	node.addEventListener('mouseleave', hide);
	node.addEventListener('focus', show);
	node.addEventListener('blur', hide);

	return {
		update(next: string | null | undefined): void {
			current = next ?? '';
			if (!current) hide();
			else if (tooltipState.anchor === node) tooltipState.text = current;
		},
		destroy(): void {
			hide();
			node.removeEventListener('mouseenter', show);
			node.removeEventListener('mouseleave', hide);
			node.removeEventListener('focus', show);
			node.removeEventListener('blur', hide);
		},
	};
}
