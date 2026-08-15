<script lang="ts">
	import type { Snippet } from 'svelte';

	let { title, children, stretch = false }: { title: string; children: Snippet; stretch?: boolean } = $props();

	const contentClass = $derived(stretch ? 'stretch' : 'center');
</script>

<div class="page-layout">
	<h2 class="page-title">{title}</h2>
	<div class={['page-content', contentClass]}>
		{@render children()}
	</div>
</div>

<style>
	.page-layout {
		display: flex;
		flex-direction: column;
		flex: 1;
		position: relative;
	}

	.page-title {
		font-size: var(--font-size-lg, 1.5rem);
		line-height: 1.3;
		color: var(--color-text, #333);
		font-weight: var(--font-weight-700, 700);
		text-align: center;
		overflow: hidden;
	}

	.page-content {
		display: flex;
		flex: 1;
		min-height: 0;
		min-width: 480px;
		flex-direction: column;
		align-items: center;
		gap: var(--page-gap, 24px);
		width: 100%;
		max-width: 520px;
		margin: 0 auto;
	}

	.page-content.center {
		justify-content: center;
	}

	.page-content.stretch {
		justify-content: flex-start;
	}
</style>
