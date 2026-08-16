<script lang="ts">
	/**
	 * Full-width centered layout for pages that don't need the 2-column structure.
	 * Used for Installation and Finish pages.
	 *
	 * Props:
	 *   title   - Page heading (rendered as h2 at top)
	 *   subtitle - Descriptive text below title (rendered as p)
	 *
	 * Slots:
	 *   default - Main centered content
	 */
	export let title = '';
	export let subtitle = '';
</script>

<div class="centered-page">
	{#if title}
		<h2 class="page-title">{title}</h2>
	{/if}
	{#if subtitle}
		<p class="page-subtitle">{subtitle}</p>
	{/if}
	<slot />
</div>

<style>
	.centered-page {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--spacing-lg);
		flex: 1;
		width: 100%;
		padding: var(--spacing-xl);
		box-sizing: border-box;
	}

	.page-title {
		margin: 0;
		font-size: var(--font-size-xl);
		font-weight: var(--font-weight-700);
		line-height: 1.2;
		color: var(--color-text);
		text-align: center;
	}

	.page-subtitle {
		margin: 0;
		font-size: var(--font-size-base-lg);
		line-height: 1.5;
		color: var(--color-text-muted);
		text-align: center;
	}
</style>
