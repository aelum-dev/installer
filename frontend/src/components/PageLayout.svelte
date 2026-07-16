<script lang="ts">
	import type { Snippet } from 'svelte';

	let { title, children }: { title: string; children: Snippet } = $props();
</script>

<div class="page-layout">
	<h2 class="page-title">{title}</h2>
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
		gap: var(--spacing-lg, 20px);
		position: relative;
	}

	.page-title {
		margin: 0;
		font-size: var(--font-size-lg, 1.5rem);
		color: var(--color-text, #333);
		font-weight: var(--font-weight-700, 700);
	}

	.page-content {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--spacing-xl, 24px);
		width: 100%;
		max-width: 520px;
	}
</style>
