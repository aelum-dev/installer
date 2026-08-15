<script lang="ts">
	import { PAGE_NAMES } from '$lib/types';
	import { getIsDryRun } from '$lib/state.svelte';
	import { t } from '$lib/i18n';

	let { currentPage }: { currentPage: number } = $props();
</script>

<header class="header">
	<span class="brand">{t('app.brand')}</span>
	{#if getIsDryRun()}
		<span class="dry-run-badge">{t('app.dryRun')}</span>
	{/if}
</header>

<style>
	.header {
		display: flex;
		justify-content: space-between;
		background-color: var(--color-bg-light);
		padding: var(--spacing-lg);
	}

	.brand {
		font-size: 0.95rem;
		font-weight: var(--font-weight-700);
		color: var(--color-text);
		text-transform: uppercase;
		letter-spacing: 1px;
		font-family: "Nunito", var(--font-family);
	}

	.dry-run-badge {
		display: inline-flex;
		align-items: center;
		padding: 4px 12px;
		background: var(--color-primary);
		color: white;
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-700);
		border-radius: var(--radius-xl);
		letter-spacing: 0.04em;
	}
</style>
