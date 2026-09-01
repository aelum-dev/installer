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
	disabled={parseInt(disk.size, 10) < MinDiskSizeBytes}
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

	{#if selected && (disk.fstype || disk.mountpoint || disk.isRemovable)}
		<div class="card-tags">
			{#if disk.isRemovable}
				<span class="tag removable-tag">{t('disk.removableWarning')}</span>
			{/if}
			{#if disk.fstype}
				<span class="tag fstype-tag">{t('disk.existing')} {disk.fstype}</span>
			{/if}
			{#if disk.mountpoint}
				<span class="tag mounted-tag">{t('disk.mounted')} {disk.mountpoint}</span>
			{/if}
		</div>
	{/if}

	{#if selected}
		<div class="card-partitions">
			{#if partitionsLoading}
				<p class="partition-loading">{t('page.diskSelection.scanPartitions')}</p>
			{:else if partitions.length > 0}
				<table class="partition-table">
					<thead>
						<tr>
							<th>{t('disk.name')}</th>
							<th>{t('disk.type')}</th>
							<th class="text-right">{t('disk.size')}</th>
							<th class="text-right">{t('disk.mount')}</th>
						</tr>
					</thead>
					<tbody>
						{#each partitions as part (part.path)}
							<tr>
								<td class="part-name">{part.name}</td>
								<td>
									{#if part.fstype}
										<span class="part-fstype">{part.fstype}</span>
									{:else}
										<span class="part-unknown">—</span>
									{/if}
								</td>
								<td class="text-right">{part.sizeFormatted}</td>
								<td class="text-right">
									{#if part.mountpoints && part.mountpoints.length > 0}
										<span class="part-mount">{part.mountpoints.join(', ')}</span>
									{:else}
										<span class="part-empty">—</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{:else}
				<p class="partition-empty">{t('page.diskSelection.noPartitions')}</p>
			{/if}
		</div>
	{/if}
</button>
