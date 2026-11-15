<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * The wizard's left info rail: page icon, title, subtitle, and optional extra
	 * content. Shared by PageLayout and the disk-selection sidebar so the rail
	 * markup/styles live in one place.
	 *
	 * `compact` lays the same three facts out as a horizontal head — icon tile
	 * beside title and subtitle — and top-aligns the rail below it. The stacked
	 * centered identity costs a fixed window ~150px of column it cannot spend
	 * twice, which is why the disk page (the only rail that carries cards below
	 * its identity) uses it. Every other page keeps the centered stack.
	 */
	let {
		icon,
		title,
		subtitle,
		info,
		compact = false,
	}: {
		icon?: string;
		title: string;
		subtitle?: string;
		/** Extra content under the identity (cards, hints, a space bar). */
		info?: Snippet;
		/** Horizontal head + top-aligned rail, for a rail that carries cards. */
		compact?: boolean;
	} = $props();
</script>

{#if compact}
	<div class="page-info page-info--compact">
		<div class="rail-head">
			{#if icon}
				<span class="nf-icon page-icon head-glyph" aria-hidden="true">{icon}</span>
			{/if}
			<div class="rail-head-text">
				<h2 class="page-info-title">{title}</h2>
				{#if subtitle}
					<p class="page-info-subtitle">{subtitle}</p>
				{/if}
			</div>
		</div>
		{@render info?.()}
	</div>
{:else}
	<div class="page-info">
		{#if icon}
			<span class="nf-icon page-icon" aria-hidden="true">{icon}</span>
		{/if}
		<h2 class="page-info-title">{title}</h2>
		{#if subtitle}
			<p class="page-info-subtitle">{subtitle}</p>
		{/if}
		{@render info?.()}
	</div>
{/if}

<style>
	.page-info {
		/* The icon/title/subtitle stack is one block of writing, so it is spaced at
		   the rhythm of a paragraph. Anything a page adds below it (a location card,
		   a hint list) opens its own margin from here rather than widening this gap.
		   The rail is centred in the row: it is the page's reading matter, and it
		   sits at the height of the window rather than at the top of the column. */
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: var(--spacing-lg);
		flex: 1;
	}

	/* A card-carrying rail reads top-to-bottom like the cards below it, and it is
	   too tall to centre: the fixed window's overflow would split into a glyph
	   clipped at the top and a card under the footer. */
	.page-info--compact {
		justify-content: flex-start;
		gap: var(--spacing-sm);
		min-height: 0;
		overflow-y: auto;
		scrollbar-width: thin;
		text-align: start;
	}

	.rail-head {
		display: flex;
		align-items: center;
		gap: var(--gap-sm);
		min-width: 0;
		text-align: start;
	}

	.head-glyph {
		width: 44px;
		height: 44px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border-radius: var(--radius-lg);
		background: var(--color-primary-light);
		color: var(--color-primary);
		font-size: 1.9rem;
		line-height: 1;
		flex-shrink: 0;
	}

	.rail-head-text {
		min-width: 0;
	}

	.rail-head .page-info-title {
		font-size: var(--font-size-base-lg);
	}

	/* The head's subtitle belongs to the text block beside the glyph, not under a
	   centered heading — the global 30ch measure and auto centering are for the
	   stacked rail. */
	.page-info--compact .page-info-subtitle {
		max-width: none;
		margin-inline: 0;
		font-size: var(--font-size-sm);
	}
</style>
