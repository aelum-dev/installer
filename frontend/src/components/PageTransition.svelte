<script lang="ts">
	import type { TransitionDirection } from '$lib/types';

	let { children, contentKey, direction, leaving, entering }: {
		children: any;
		contentKey: number;
		direction: TransitionDirection;
		leaving: boolean;
		entering: boolean;
	} = $props();

	let className = $derived('transition-content' + (leaving ? ' leaving' : '') + (entering ? ' entering' : ''));
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
		transition: transform 200ms ease, opacity 200ms ease;
	}

	/* svelte-ignore css_unused_selector */
	.transition-content.entering {
		opacity: 0;
	}

	/* svelte-ignore css_unused_selector */
	.transition-content.leaving {
		opacity: 1;
	}

	/* Forward direction */
	/* svelte-ignore css_unused_selector */
	div[data-transition-direction="forward"].leaving {
		transform: translateX(0);
	}

	/* svelte-ignore css_unused_selector */
	div[data-transition-direction="forward"].entering {
		transform: translateX(30px);
		opacity: 0;
	}

	/* Backward direction */
	/* svelte-ignore css_unused_selector */
	div[data-transition-direction="backward"].leaving {
		transform: translateX(0);
	}

	/* svelte-ignore css_unused_selector */
	div[data-transition-direction="backward"].entering {
		transform: translateX(-30px);
		opacity: 0;
	}
</style>
