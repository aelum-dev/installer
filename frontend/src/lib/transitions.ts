import type { TransitionConfig } from 'svelte/transition';
import { cubicOut } from 'svelte/easing';

export interface PageFadeParams {
	/** Opacity at the hidden end: 0 fades the page away, 1 keeps it opaque. */
	fade?: number;
	/** Stacking order to apply while transitioning (incoming above outgoing). */
	zIndex?: number;
	/**
	 * Mark the element while it transitions out. The stylesheet uses this to
	 * hide descendant scrollbars, so a native overlay scrollbar can't outlive
	 * the page and linger over the one arriving.
	 */
	exiting?: boolean;
	delay?: number;
	duration?: number;
	easing?: (t: number) => number;
}

/**
 * Wizard page transition: a plain opacity fade — the same property Svelte's
 * built-in `fade` animates. Paired with a delay on the incoming page it becomes
 * a "fade through": the outgoing page fades out, then the incoming one fades
 * in, so the two never blend into a muddy double-exposure.
 *
 * Deliberately opacity-only. WebKitGTK does not interpolate `filter` (or other
 * non-core properties) through the Web Animations API that Svelte's transitions
 * build on — it holds the first keyframe and snaps at the end — so a blur read
 * as an instant blur-to-crisp jump. `opacity` and `transform` are the two
 * properties it does animate smoothly.
 */
export function pageFade(
	node: Element,
	{ fade = 0, zIndex, exiting = false, delay = 0, duration = 220, easing = cubicOut }: PageFadeParams = {},
): TransitionConfig {
	const computed = Number.parseFloat(getComputedStyle(node).opacity);
	const base = Number.isFinite(computed) ? computed : 1;
	const decorate = zIndex !== undefined || exiting;

	return {
		delay,
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
		css: (t) => `opacity: ${base * (fade + (1 - fade) * t)};`,
	};
}
