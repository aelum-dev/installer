<script lang="ts">
	/**
	 * The app's single tooltip bubble: fixed to the viewport under the hover-
	 * or focus-anchored element (right-aligned to it, flipped above when the
	 * anchor sits near the window's bottom edge, clamped inside the viewport),
	 * with the arrow tracking the anchor. One instance, mounted by App; every
	 * `use:tooltip` host drives it through tooltipState.
	 */
	import { tooltipState } from '$lib/tooltip.svelte';

	let bubble = $state<HTMLElement | null>(null);
	let arrow = $state<HTMLElement | null>(null);
	// Flipped placement: the bubble sits above the anchor when there is no
	// room below, and the arrow turns to point down at it.
	let above = $state(false);

	// Placed imperatively in the effect: the bubble measures itself the same
	// frame it mounts (offsetWidth is a sync layout), so it never paints at a
	// default corner and jumps.
	$effect(() => {
		const { text, anchor } = tooltipState;
		if (!text || !anchor || !bubble || !arrow) return;
		const r = anchor.getBoundingClientRect();
		const bw = bubble.offsetWidth;
		const bh = bubble.offsetHeight;
		const margin = 8;
		const gap = 6;
		let top = r.bottom + gap;
		if (top + bh > window.innerHeight - margin) {
			top = Math.max(margin, r.top - gap - bh);
		}
		above = top < r.top;
		bubble.style.top = `${top}px`;
		let left = r.right - bw;
		left = Math.min(Math.max(margin, left), window.innerWidth - bw - margin);
		bubble.style.left = `${left}px`;
		// The arrow aims at the anchor (wide anchors: its leading side, near
		// where the cursor rests), staying inside the bubble's rounded ends.
		const aim = Math.min(Math.max(r.left + Math.min(r.width, 64) / 2, left + 14), left + bw - 14);
		arrow.style.left = `${aim - left}px`;
	});
</script>

{#if tooltipState.text}
	<div class={['app-tip', { 'app-tip--above': above }]} role="tooltip" bind:this={bubble}>
		<span class="app-tip-text">{tooltipState.text}</span>
		<span class="app-tip-arrow" bind:this={arrow}></span>
	</div>
{/if}

<style>
	/* The surface of the storage rail's card hint: quiet text on the card
	   material, small and wrapped — never a band wider than the reading rail. */
	.app-tip {
		position: fixed;
		z-index: 999;
		pointer-events: none;
		animation: tipFade 150ms ease;
	}

	.app-tip-text {
		display: block;
		padding: 6px 12px;
		background: var(--color-bg-white);
		border: 1.5px solid var(--color-border);
		border-radius: var(--radius-md);
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
		width: max-content;
		max-width: 230px;
	}

	.app-tip-arrow {
		position: absolute;
		top: -10px;
		margin-left: -5px;
		border: 5px solid transparent;
		border-bottom-color: var(--color-bg-white);
	}

	.app-tip--above .app-tip-arrow {
		top: auto;
		bottom: -10px;
		border-bottom-color: transparent;
		border-top-color: var(--color-bg-white);
	}

	@keyframes tipFade {
		from {
			opacity: 0;
			translate: 0 4px;
		}
		to {
			opacity: 1;
			translate: 0 0;
		}
	}
</style>
