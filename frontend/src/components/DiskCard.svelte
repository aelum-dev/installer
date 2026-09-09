<script lang="ts">
	import type { DiskInfo, PartitionDetail } from '$lib/types';
	import { MinDiskSizeBytes } from '$lib/constants';
	import { t } from '$lib/i18n';
	import { glyph } from '$lib/icons';
	import './DiskCard.css';

	let {
		disk,
		selected,
		partitions,
		partitionsLoading,
		onSelect
	}: {
		disk: DiskInfo;
		selected: boolean;
		partitions: PartitionDetail[];
		partitionsLoading: boolean;
		onSelect: (disk: DiskInfo) => void;
	} = $props();
</script>

<button
	type="button"
	class={['disk-card', { selected }]}
	onclick={() => onSelect(disk)}
	disabled={disk.size < MinDiskSizeBytes}
>
	<div class="card-header">
		<div class="disk-icon" aria-hidden="true">
			{#if disk.isRemovable}
				<span class="disk-icon-glyph removable-glyph">{glyph('f129e')}</span> <!-- removable: USB/SD/MMC flash drive (md-usb_flash_drive) -->
			{:else}
				{#if disk.rotational}
					<span class="disk-icon-glyph">{glyph('ede9')}</span> <!-- HDD: spinning disc -->
				{:else}
					<span class="disk-icon-glyph">{glyph('f061a')}</span> <!-- SSD: flash chip -->
				{/if}
			{/if}
		</div>
		<div class="card-title">
			<div class="disk-name">{disk.model || disk.name}</div>
			<div class="disk-specs">
				{disk.sizeFormatted}
				<span class="specs-sep">·</span>
				{t(disk.rotational ? 'disk.hdd' : 'disk.ssd')}
				{#if disk.trans}
					<span class="specs-sep">·</span>
					{disk.trans.toUpperCase()}
				{/if}
			</div>
		</div>
		<div class="card-radio">
			<div class={['radio', { checked: selected }]}></div>
		</div>
	</div>


</button>
