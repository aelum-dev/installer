<script lang="ts">
	import { PAGE_NAMES } from '$lib/types';
	import { getIsDryRun } from '$lib/state.svelte';

	let { currentPage }: { currentPage: number } = $props();
</script>

<header class="header">
	<span class="brand">AestheticOS</span>
	{#if getIsDryRun()}
		<span class="dry-run-badge">DRY RUN</span>
	{/if}
</header>

<style>
	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		min-height: var(--header-height);
		background-color: var(--color-bg-light);
	}

	.brand {
		font-size: 0.95rem;
		font-weight: var(--font-weight-700);
		color: var(--color-text);
		text-transform: uppercase;
		letter-spacing: 1px;
		margin-left: var(--spacing-lg);
	}

	.dry-run-badge {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 12px;
		background: var(--color-primary);
		color: white;
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-700);
		border-radius: var(--radius-xl);
		letter-spacing: 0.04em;
		margin-right: var(--spacing-lg);
	}
</style>
