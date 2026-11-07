<script lang="ts">
	/**
	 * The location chooser: a filter field over the list of places Arch has mirrors
	 * for. It is a dialog rather than a page because the location is one setting of
	 * the Welcome page, not a step of its own — and because the list is Arch's own
	 * country list (70+ entries), which is a thing you search for, not a thing you
	 * scroll.
	 */
	import { t } from '$lib/i18n';
	import Dialog from '$components/Dialog.svelte';
	import OptionList from '$components/OptionList.svelte';
	import ScrollBox from '$components/ScrollBox.svelte';
	import type { RegionOption } from '$lib/types';

	let {
		show = $bindable(false),
		regions,
		selected,
		onSelect,
	}: {
		show?: boolean;
		regions: RegionOption[];
		selected: string;
		onSelect: (code: string) => void;
	} = $props();

	let filter = $state('');

	// Matches on the country name, its code, or the zone — someone looking for
	// "Kyiv" is looking for a place they know, and typing the city is how they
	// start. localeCompare so accented names sort where a reader expects them.
	const visible = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		const matches = regions.filter(
			(r) =>
				q === '' ||
				r.name.toLowerCase().includes(q) ||
				r.code === q ||
				r.timezone.toLowerCase().includes(q),
		);
		return [...matches].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
	});

	function choose(code: string): void {
		onSelect(code);
		show = false;
	}

	// The filter is the first thing anyone uses here, so it takes focus when the
	// dialog is inserted. (Dialog's own focusOnOpen focuses the dialog box.)
	function focusOnInsert(node: HTMLInputElement): void {
		node.focus();
	}
</script>

<Dialog bind:show title={t('dialog.region.title')} focusOnOpen={false} onCancel={() => (show = false)}>
	<div class="region-filter">
		<label for="region-filter">{t('dialog.region.filter')}</label>
		<input
			id="region-filter"
			type="text"
			class="line-edit"
			placeholder={t('dialog.region.filterPlaceholder')}
			bind:value={filter}
			use:focusOnInsert
		/>
	</div>

	<div class="region-list">
		<ScrollBox persistKey="region-picker">
			{#if visible.length > 0}
				<OptionList items={visible} selected={selected} onSelect={choose} label={t('dialog.region.title')} />
			{:else}
				<p class="region-empty">{t('dialog.region.noMatch')}</p>
			{/if}
		</ScrollBox>
	</div>

	{#snippet actions()}
		<button type="button" class="btn-cancel" onclick={() => (show = false)}>
			{t('dialog.region.close')}
		</button>
	{/snippet}
</Dialog>

<style>
	/* Shared dialog chrome lives in Dialog.svelte / dialog.css; only the picker's
	   own layout lives here. */
	.region-filter {
		display: flex;
		flex-direction: column;
		text-align: start;
		margin-bottom: var(--spacing-md);
	}

	.region-filter label {
		font-size: var(--font-size-sm);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
		letter-spacing: 0.02em;
		margin-bottom: var(--spacing-xs);
	}

	/* The list scrolls inside the dialog, so the dialog itself never grows past
	   the window on a long country list. Column direction is the point:
	   ScrollBox sizes itself with flex:1/min-height:0, which only bounds the
	   height in a column — in a row it would bound the width and the list would
	   grow past the Close button instead of scrolling. max-height (not height)
	   so a short filtered list shrinks instead of leaving an empty box. */
	.region-list {
		display: flex;
		flex-direction: column;
		max-height: 46vh;
		min-height: 0;
	}

	.region-empty {
		margin: var(--spacing-sm) 0;
		font-size: var(--font-size-md);
		color: var(--color-text-light);
		text-align: start;
	}
</style>
