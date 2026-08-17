<script lang="ts">
	/**
	 * Two-column page layout constrained between header and footer.
	 * Left column: title, subtitle, hints (static).
	 * Right column: content area (pages manage their own scrolling).
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

<div class="page-layout">
	<div class="page-content">
		<div class="col col-left">
			<div class="col-inner">
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
			<slot name="right" />
		</div>
	</div>
</div>

<style>
	.page-layout {
		height: 100%;
		width: 100%;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.page-content {
		display: grid;
		grid-template-columns: 1fr 1fr;
		grid-template-rows: 1fr;
		gap: var(--gap-lg);
		flex: 1;
		width: 100%;
		min-height: 0;
		padding-inline: var(--spacing-xl);
		box-sizing: border-box;
	}

	.col {
		min-width: 0;
		min-height: 0;
		align-self: stretch;
	}

	.col-left {
		display: flex;
		flex-direction: column;
		align-self: center;
		padding-top: var(--spacing-lg);
		padding-bottom: var(--spacing-lg);
	}

	.col-inner {
		display: flex;
		flex-direction: column;
		gap: var(--spacing-lg);
		width: 100%;
		box-sizing: border-box;
	}

	.col-right {
		display: flex;
		flex-direction: column;
		flex: 1;
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}

	:global(.col-right > *) {
		flex: 1;
		min-height: 0;
		height: 100%;
		display: flex;
		flex-direction: column;
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
</style>
