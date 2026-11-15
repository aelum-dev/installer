<script lang="ts">
	import type { DiskInfo, PartitionEntry } from '$lib/types';
	import type { PlanChoice } from '$lib/disk-layout';
	import {
		contentSummary,
		destinationChain,
		destinationSegments,
		diskSegments,
		disabledReason,
		statusChip,
	} from '$lib/disk-layout';
	import { t } from '$lib/i18n';
	import { diskGlyph } from '$lib/icons';
	import LayoutStrip from '$components/LayoutStrip.svelte';
	import './DiskCard.css';

	let {
		disk,
		partitions = [],
		selected,
		plan,
		onSelect,
	}: {
		disk: DiskInfo;
		/** What is on the disk, for the strip and the content line. */
		partitions?: PartitionEntry[];
		selected: boolean;
		/** The storage choices — a selected row previews them as the destination. */
		plan: PlanChoice;
		onSelect: (disk: DiskInfo) => void;
	} = $props();

	// The type line at the trailing edge: bus (when the device reports one) and
	// the media word, or "removable" for USB/SD/MMC.
	const typeText = $derived(
		disk.isRemovable
			? t('disk.removable')
			: (disk.trans ? disk.trans.toUpperCase() + ' ' : '') + t(disk.rotational ? 'disk.hdd' : 'disk.ssd'),
	);

	const reason = $derived(disabledReason(disk));
	const chip = $derived(statusChip(disk, partitions));

	// The strip tells the truth for the row's state: what is, or — once
	// selected — what will be. Plan and consequence live in the same element,
	// so flipping Encrypt/LVM in the options card redraws the row under the
	// cursor.
	const segments = $derived(selected ? destinationSegments(disk, plan) : diskSegments(disk, partitions));
	const chain = $derived(selected ? destinationChain(disk, plan) : '');
	const content = $derived(reason ?? contentSummary(disk, partitions));
</script>

<button
	type="button"
	class={['disk-card', { selected }]}
	onclick={() => onSelect(disk)}
	disabled={reason !== null}
	aria-pressed={selected}
	title={`${disk.model || disk.name} — ${disk.path}`}
>
	<span class="disk-row">
		<span class="disk-glyph nf-icon" aria-hidden="true">{diskGlyph(disk)}</span>
		<span class="disk-name">{disk.model || disk.name}</span>
		{#if chip}
			<span class="chip chip--{chip.tone}">{chip.text}</span>
		{/if}
		<span class="disk-size">{disk.sizeFormatted}</span>
		<span class="disk-type">{typeText}</span>
	</span>
	<span class="disk-row disk-meta">
		<span class="disk-path">{disk.path}</span>
		<span class="disk-content">{content}</span>
	</span>
	<LayoutStrip {segments} tag={t('disk.layoutTag')} caption={chain} />
</button>
