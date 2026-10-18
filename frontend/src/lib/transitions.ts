import type { TransitionConfig } from 'svelte/transition';
import { cubicOut } from 'svelte/easing';

export interface PageSlideParams {
	/** Horizontal travel in px. Positive = starts/ends to the right. */
	distance?: number;
	/**
	 * Opacity at the hidden end of the curve: 0 fades the page away, 1 keeps it
	 * fully opaque. The incoming page uses 1 so it immediately covers the
	 * outgoing one (whose scrollbars would otherwise show through a crossfade).
	 */
	fade?: number;
	/** Stacking order to apply while transitioning (incoming above outgoing). */
	zIndex?: number;
	/**
	 * Mark the element while it transitions out. The stylesheet uses this to
	 * hide descendant scrollbars, so a native overlay scrollbar can't outlive
	 * the page and linger over the one arriving.
	 */
	exiting?: boolean;
	duration?: number;
	easing?: (t: number) => number;
}

/**
 * Wizard page transition: a short horizontal push. Used as both `in:` and
 * `out:`. Svelte drives the same t=0..1 curve in both directions (t=0 is fully
 * hidden), so the caller only flips the sign of `distance`.
 */
export function pageSlide(
	node: Element,
	{ distance = 48, fade = 0, zIndex, exiting = false, duration = 320, easing = cubicOut }: PageSlideParams = {},
): TransitionConfig {
	const computed = Number.parseFloat(getComputedStyle(node).opacity);
	const base = Number.isFinite(computed) ? computed : 1;
	const decorate = zIndex !== undefined || exiting;

	return {
		duration,
		easing,
		// Pinning the stacking order keeps the incoming page above the outgoing
		// one even if the DOM order of the two keyed branches ever changes.
		tick: decorate
			? () => {
					const el = node as HTMLElement;
					if (zIndex !== undefined) el.style.zIndex = String(zIndex);
					if (exiting) el.setAttribute('data-exiting', '');
				}
			: undefined,
		css: (t) => {
			const x = (1 - t) * distance;
			const opacity = base * (fade + (1 - fade) * t);
			return `transform: translateX(${x}px); opacity: ${opacity};`;
		},
	};
}
