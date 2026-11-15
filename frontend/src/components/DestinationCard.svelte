<script lang="ts">
	/**
	 * The rail's DESTINATION card: the plan for exactly one disk, drawn with the
	 * same strip language as its row in the list, so the person reads the same
	 * diagram twice — once as "what is", once as "what will be". The card is
	 * permanent (the plan always has one home): before a disk is picked it asks
	 * for one instead of being absent. Automatic mode draws the installer's
	 * layout and carries the erase warning once, here; manual mode draws the
	 * person's own plan — the identical picture the disk map above the editor
	 * shows — and states the same repartition scope the map caption states.
	 */
	import type { DiskInfo, ManualLayout } from '$lib/types';
	import type { PlanChoice } from '$lib/disk-layout';
	import { destinationSegments, manualMapSegments, manualPlanLabel } from '$lib/disk-layout';
	import { formatBytes } from '$lib/format';
	import { MinDiskSizeBytes } from '$lib/constants';
	import { t } from '$lib/i18n';
	import { diskGlyph } from '$lib/icons';
	import LayoutStrip from '$components/LayoutStrip.svelte';

	let {
		disk,
		plan,
		manual = false,
		layout = null,
	}: {
		/** The selected disk, or null — the empty state then asks for one. */
		disk: DiskInfo | null;
		plan: PlanChoice;
		/** Partitioning mode: the strip shows the manual plan, not the automatic layout. */
		manual?: boolean;
		/** The manual plan being edited; null while it loads. */
		layout?: ManualLayout | null;
	} = $props();

	// In manual mode the plan is the truth: before it loads, nothing is drawn
	// rather than flashing the automatic layout it would not follow.
	const segments = $derived(
		manual
			? layout
				? manualMapSegments(layout)
				: []
			: disk
				? destinationSegments(disk, plan)
				: []
	);
	/** Automatic: the erase warning (danger). Manual: the map's own scope line,
	 *  word for word — or nothing while the plan loads. */
	const note = $derived(
		manual
			? layout
				? t('manual.mapCaption', { plan: manualPlanLabel(plan) })
				: null
			: t('disk.eraseWarning')
	);
	const minDiskSizeFormatted = $derived(formatBytes(MinDiskSizeBytes));
</script>

<div class="dest-card">
	<div class="card-label">{t('destination.title')}</div>

	<!-- The card grows into the rail's leftover height; the label stays at the
		 card's reading edge like the options card's, and everything the card
		 actually says — identity, strip, note, or the empty ask — centers in
		 what is left, so the growing height is always the equal margin above
		 and below the information, never a band of emptiness under it. -->
	<div class="dest-body">
		{#if disk}
			<div class="dest-id">
				<span class="nf-icon dest-glyph" aria-hidden="true">{diskGlyph(disk)}</span>
				<span class="dest-model">{disk.model || disk.name}</span>
				<span class="dest-size">{disk.sizeFormatted}</span>
			</div>
			<!-- No device-path line and no chain caption: the selected row beside the
				 rail shows both (its meta row carries the path, its caption the chain),
				 and the rail's 440px at the fixed 900x600 window cannot repeat them
				 and still keep the note visible. Both still live in the strip's own
				 tooltips. -->
			<LayoutStrip {segments} />
			{#if note}
				<p class={['dest-note', { 'dest-note--danger': !manual }]}>
					{#if !manual}
						<svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 1.3 15 13.8H1L8 1.3Zm0 3.2L3.3 12h9.4L8 4.5Zm-.7 2.3v3h1.4v-3H7.3Zm0 4v1.4h1.4v-1.4H7.3Z"/></svg>
					{/if}
					{note}
				</p>
			{/if}
		{:else}
			<div class="dest-empty">
				<span class="dest-msg">{t('destination.emptyTitle')}</span>
				<span class="dest-sub">
					{t('destination.emptyHint')} {t('page.diskSelection.minDiskSize', { minSize: minDiskSizeFormatted })}
				</span>
			</div>
		{/if}
	</div>
</div>

<style>
	/* The rail's card surface — the same recipe the storage-options card and the
	   Welcome page's region card use, so the rail is one material.
	   The rail's bottom block: it grows into whatever rail height the options
	   card above it leaves, so its height belongs to the rail, not to its
	   content — selecting or deselecting a disk never moves its edges. (The
	   tallest content — the manual plan's note — still fits the rail's
	   fixed-window budget; see e2e/manual.spec.ts railFits. min-height:auto,
	   the flex default, keeps the content whole if a window ever shrinks it.) */
	.dest-card {
		display: flex;
		flex-direction: column;
		flex-grow: 1;
		gap: 7px;
		background-color: var(--color-bg-white);
		border: 1px solid var(--color-border-light);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-md);
		/* The same spacing measure as the options card above it — same gap
		   between the label and its content, same inset — so the two cards of
		   one rail read as one rhythm. The rail's fixed-window budget is tight
		   at the shipped 900x600; e2e/manual.spec.ts railFits pins it. */
		padding: 10px var(--spacing-md);
		text-align: start;
	}

	.card-label {
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	/* The centered block under the label — see the markup comment. min-height
	   stays auto (the flex default): if content ever outgrows the leftover
	   space the card keeps it whole and the rail scrolls, rather than the
	   centered block being squeezed and clipped at both ends. */
	.dest-body {
		display: flex;
		flex-direction: column;
		gap: 7px;
		flex-grow: 1;
		justify-content: center;
	}

	.dest-id {
		display: flex;
		align-items: baseline;
		gap: var(--gap-sm);
		min-width: 0;
	}

	.dest-glyph {
		color: var(--color-text-muted);
		flex-shrink: 0;
		align-self: center;
	}

	.dest-model {
		font-size: var(--font-size-md-lg);
		font-weight: 600;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.dest-size {
		margin-inline-start: auto;
		font-size: var(--font-size-md);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	/* The consequence, named once, in the same place the plan is shown: the
	   erase warning in automatic mode, the map's own scope line in manual. */
	.dest-note {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 0;
		font-size: var(--font-size-xs);
		color: var(--color-text-muted);
	}

	.dest-note--danger {
		color: var(--color-danger);
	}

	.dest-note svg {
		width: 12px;
		height: 12px;
		flex-shrink: 0;
	}

	/* Honest empty state: picking a disk fills a known place instead of making
	   a card appear out of nowhere. */
	.dest-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
		padding-block: var(--spacing-sm) var(--spacing-xs);
		text-align: center;
	}

	.dest-msg {
		font-size: var(--font-size-md-lg);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
	}

	.dest-sub {
		font-size: var(--font-size-xs);
		color: var(--color-text-light);
		max-width: 32ch;
	}
</style>
