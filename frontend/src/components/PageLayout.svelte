<script lang="ts">
	import type { Snippet } from 'svelte';

	let { title, children }: { title: string; children: Snippet } = $props();
</script>

<div class="page-layout">
	<h2>{title}</h2>
	<div class="page-content">
		{@render children()}
	</div>
</div>

<style>
	.page-layout {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		gap: var(--spacing-md, 12px);
	}

	h2 {
		position: absolute;
		top: var(--page-heading-top, 24px);
		left: 50%;
		transform: translateX(-50%);
		font-size: var(--font-size-lg, 1.5rem);
		color: var(--color-text, #333);
		margin: 0;
	}

	.page-content {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--spacing-md, 12px);
	}
</style>
