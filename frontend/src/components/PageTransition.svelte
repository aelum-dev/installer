<script lang="ts">
	import type { TransitionDirection } from '$lib/types';

	let { children, contentKey, direction, leaving, entering, enteringDone }: {
		children: any;
		contentKey: number;
		direction: TransitionDirection;
		leaving: boolean;
		entering: boolean;
		enteringDone: boolean;
	} = $props();

	let className = $derived('transition-content' + (leaving ? ' leaving' : '') + (entering ? ' entering' : '') + (enteringDone ? ' done' : ''));
</script>

<div class="page-transition">
	<div class={className} data-transition-direction={direction}>
		{@render children()}
	</div>
</div>

<style>
	.page-transition {
		position: relative;
		width: 100%;
		height: 100%;
	}

	.transition-content {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		transition: transform 250ms cubic-bezier(0.4, 0, 0.2, 1),
		            opacity 250ms cubic-bezier(0.4, 0, 0.2, 1);
		will-change: transform, opacity;
	}

	/* ── Forward direction ── */

	/* Leaving page slides left and fades out */
	div[data-transition-direction="forward"].leaving {
		transform: translateX(-40px);
		opacity: 0;
	}

	/* Entering page starts off-screen right with no opacity */
	div[data-transition-direction="forward"].entering {
		transform: translateX(40px);
		opacity: 0;
	}

	/* Entering page ends at normal position with full opacity */
	div[data-transition-direction="forward"].entering.done {
		transform: translateX(0);
		opacity: 1;
	}

	/* ── Backward direction ── */

	/* Leaving page slides right and fades out */
	div[data-transition-direction="backward"].leaving {
		transform: translateX(40px);
		opacity: 0;
	}

	/* Entering page starts off-screen left with no opacity */
	div[data-transition-direction="backward"].entering {
		transform: translateX(-40px);
		opacity: 0;
	}

	/* Entering page ends at normal position with full opacity */
	div[data-transition-direction="backward"].entering.done {
		transform: translateX(0);
		opacity: 1;
	}
</style>
