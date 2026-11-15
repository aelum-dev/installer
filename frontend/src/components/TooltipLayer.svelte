<script lang="ts">
	/**
	 * The app's single tooltip bubble: fixed to the viewport and hung from the
	 * cursor — centred under the pointer where a pointer opened it, from the
	 * anchor's own bottom edge where keyboard focus did — flipped above when
	 * there is no room below, clamped inside the viewport, with the arrow
	 * pointing back at the spot it hangs from. One instance, mounted by App;
	 * every `use:tooltip` host drives it through tooltipState.
	 */
	import { tooltipState } from '$lib/tooltip.svelte';

	let bubble = $state<HTMLElement | null>(null);
	let arrow = $state<HTMLElement | null>(null);
	// Flipped placement: the bubble sits above the point it hangs from when
	// there is no room below, and the arrow turns to point down at it.
	let above = $state(false);

	// Placed imperatively in the effect: the bubble measures itself the same
	// frame it mounts (offsetWidth is a sync layout), so it never paints at a
	// default corner and jumps.
	$effect(() => {
		const { text, anchor, cursor } = tooltipState;
		if (!text || !anchor || !bubble || !arrow) return;
		const r = anchor.getBoundingClientRect();
		const bw = bubble.offsetWidth;
		const bh = bubble.offsetHeight;
		const margin = 8;
		/* 14 rather than a bare 6: the arrow's tip stands 10px above the box,
		   so this is the measure that puts the tip just under the cursor. */
		const gap = 14;
		// The point the bubble hangs from: the cursor when a pointer asked for
		// the hint; the anchor's bottom centre when focus did, there being no
		// pointer to hang from.
		const aim = cursor ? cursor.x : r.left + r.width / 2;
		const edge = cursor ? cursor.y : r.bottom;
		let top = edge + gap;
		if (top + bh > window.innerHeight - margin) {
			top = Math.max(margin, edge - gap - bh);
		}
		above = top < edge;
		bubble.style.top = `${top}px`;
		let left = aim - bw / 2;
		left = Math.min(Math.max(margin, left), window.innerWidth - bw - margin);
		bubble.style.left = `${left}px`;
		// The arrow points back at that spot — staying inside the bubble's
		// rounded ends once the bubble itself was pushed in from a viewport
		// edge, where the spot it aims at lies outside the bubble.
		const tip = Math.min(Math.max(aim, left + 14), left + bw - 14);
		arrow.style.left = `${tip - left}px`;
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
