<script lang="ts">
	import type { DiskInfo } from '$lib/types';
	import { MinDiskSizeBytes } from '$lib/constants';
	import { t } from '$lib/i18n';
	import { diskGlyph } from '$lib/icons';
	import './DiskCard.css';

	let {
		disk,
		selected,
		onSelect
	}: {
		disk: DiskInfo;
		selected: boolean;
		onSelect: (disk: DiskInfo) => void;
	} = $props();

	// The type line at the trailing edge: bus (when the device reports one) and
	// the media word, or "removable" for USB/SD/MMC.
	const typeText = $derived(
		disk.isRemovable
			? t('disk.removable')
			: (disk.trans ? disk.trans.toUpperCase() + ' ' : '') + t(disk.rotational ? 'disk.hdd' : 'disk.ssd')
	);
</script>

<button
	type="button"
	class={['disk-card', { selected }]}
	onclick={() => onSelect(disk)}
	disabled={disk.size < MinDiskSizeBytes}
>
	<span class="disk-glyph nf-icon" aria-hidden="true">{diskGlyph(disk)}</span>
	<span class="disk-main">
		<span class="disk-name">{disk.model || disk.name}</span>
		<span class="disk-path">{disk.path}</span>
	</span>
	<span class="disk-specs">
		<span class="disk-size">{disk.sizeFormatted}</span>
		<span class="disk-type">{typeText}</span>
	</span>
	<span class="disk-check" aria-hidden="true">
		<svg viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
			<path d="M3.5 8.5L6.5 11.5L12.5 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
		</svg>
	</span>
</button>
