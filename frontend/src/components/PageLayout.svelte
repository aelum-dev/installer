<script lang="ts">
	/**
	 * Two-column page layout with consistent padding and structure.
	 * Both left and right slots are wrapped in a content-box with uniform padding.
	 *
	 * Props:
	 *   title   - Page heading (rendered as h2 in left column)
	 *   subtitle - Descriptive text below the title (rendered as p in left column)
	 *   hint    - Small hint/help text below subtitle (rendered as p in left column)
	 *
	 * Slots:
	 *   left   - Additional content for left column (after title/subtitle/hint)
	 *   right  - Main content for right column
	 */
	export let title = '';
	export let subtitle = '';
	export let hint = '';
</script>

<div class="page-content">
	<div class="col col-left">
		<div class="content-box">
			{#if title}
				<h2 class="page-title">{title}</h2>
			{/if}
			{#if subtitle}
				<p class="page-subtitle">{subtitle}</p>
			{/if}
			{#if hint}
				<p class="page-hint">{hint}</p>
			{/if}
			<slot name="left" />
		</div>
	</div>
	<div class="col col-right">
		<div class="content-box">
			<slot name="right" />
		</div>
	</div>
</div>

<style>
	.page-content {
		display: grid;
		grid-template-columns: 1fr 1fr;
		align-items: center;
		gap: var(--gap-lg);
		flex: 1;
		width: 100%;
		box-sizing: border-box;
		min-width: 0;
	}

	.col-left,
	.col-right {
		min-width: 0;
	}

	.col-left {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: var(--spacing-lg);
	}

	.col-right {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		justify-content: center;
		overflow-y: auto;
		overflow-x: hidden;
	}

	.content-box {
		padding: var(--spacing-xl);
		display: flex;
		flex-direction: column;
		gap: var(--spacing-md);
		width: 100%;
		box-sizing: border-box;
		flex: 1;
	}

	/* Consistent title/subtitle/hint styling */
	.page-title {
		margin: 0;
		font-size: var(--font-size-xl);
		font-weight: var(--font-weight-700);
		line-height: 1.2;
		color: var(--color-text);
	}

	.page-subtitle {
		margin: 0;
		font-size: var(--font-size-base-lg);
		line-height: 1.5;
		color: var(--color-text-muted);
	}

	.page-hint {
		margin: 0;
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
	}

	.col-right::-webkit-scrollbar {
		width: 8px;
	}

	.col-right::-webkit-scrollbar-track {
		background: var(--color-border-light);
		border-radius: 4px;
	}

	.col-right::-webkit-scrollbar-thumb {
		background: var(--color-border);
		border-radius: 4px;
	}

	.col-right::-webkit-scrollbar-thumb:hover {
		background: var(--color-border-dark);
	}
</style>
