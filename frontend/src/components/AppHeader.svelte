<script lang="ts">
	import { PAGE_IDS } from '$lib/types';
	import { getIsDryRun, navState } from '$lib/store.svelte';
	import { t } from '$lib/i18n';
	import logoUrl from '../assets/images/logo.svg';
	import ThemeToggle from '$components/ThemeToggle.svelte';
</script>

<header class="header">
	<span class="brand">
		<img class="brand-logo" src={logoUrl} alt="Aelum logo" />
		<span class="brand-name">{t('app.brand')}</span>
	</span>
	<span class="header-actions">
		{#if getIsDryRun()}
			<span class="dry-run-badge">{t('app.dryRun')}</span>
		{/if}
		{#if
			navState.currentPage !== PAGE_IDS.INSTALLATION &&
			navState.currentPage !== PAGE_IDS.FINISH
		}
			<ThemeToggle />
		{/if}
	</span>
</header>

<style>
	.header {
		display: flex;
		justify-content: space-between;
		background-color: var(--color-bg-light);
		min-height: 40px;
		align-items: center;
		padding-bottom: var(--spacing-lg);
		padding-left: var(--spacing-lg);
		padding-right: var(--spacing-lg);
	}

	.brand {
		display: flex;
		align-items: center;
		gap: var(--spacing-sm);
	}

	.brand-logo {
		width: 28px;
		height: 28px;
		object-fit: contain;
	}

	.brand-name {
		font-size: var(--font-size-md-lg);
		font-weight: var(--font-weight-700);
		color: var(--color-text);
		text-transform: uppercase;
		letter-spacing: 1px;
		font-family: "Nunito", var(--font-family);
	}

	.header-actions {
		display: flex;
		align-items: center;
		gap: var(--spacing-sm);
	}

	.dry-run-badge {
		display: inline-flex;
		align-items: center;
		padding: 4px 12px;
		background: var(--color-primary);
		color: var(--color-on-primary);
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-700);
		border-radius: var(--radius-xl);
		letter-spacing: 0.04em;
	}
</style>
