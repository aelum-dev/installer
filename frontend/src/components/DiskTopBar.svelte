<script lang="ts">
	import type { DiskInfo, FileSystem } from '$lib/types';
	import { FILESYSTEMS } from '$lib/types';
	import { t } from '$lib/i18n';
	import './DiskTopBar.css';

	type InstallMode = 'automatic' | 'manual';

	let {
		pageIcon,
		disk,
		fileSystem,
		manualMode,
		spaceUsedMB = 0,
		spaceTotalMB = 0,
		spaceFreeMB = 0,
		spaceUsedPct = 0,
		onSelectFileSystem,
		onToggleMode
	}: {
		pageIcon: string;
		disk: DiskInfo | null;
		fileSystem: FileSystem | '';
		manualMode: boolean;
		spaceUsedMB?: number;
		spaceTotalMB?: number;
		spaceFreeMB?: number;
		spaceUsedPct?: number;
		onSelectFileSystem: (fs: FileSystem) => void;
		onToggleMode: (mode: InstallMode) => void;
	} = $props();

	// Group digits with a space every 3 (e.g. 1500 -> "1 500") for readability.
	function formatMB(mb: number): string {
		return String(mb).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
	}

	// Convert MB to GB and format with one decimal (e.g. 1536 MB -> "1.5 GB").
	function formatGB(mb: number): string {
		const gb = mb / 1024;
		return `${Math.round(gb * 10) / 10} GB`;
	}
</script>

<div class="top-bar">
	{#if manualMode && disk}
		<div class="manual-title">
			<span class="nf-icon page-icon" aria-hidden="true">{pageIcon}</span>
			<div class="manual-title-content">
				<h3>{disk.model || disk.name || '—'}</h3>
				{#if spaceTotalMB > 0}
					<div class="manual-space">
						<div
							class="manual-space-bar"
							role="progressbar"
							aria-valuenow={spaceUsedPct}
							aria-valuemin={0}
							aria-valuemax={100}
						>
							<div class="manual-space-used" style="--pct: {spaceUsedPct}%"></div>
						</div>
						<div class="manual-space-labels">
							<span>{t('manual.allocated')} {formatGB(spaceUsedMB)} {t('unit.gb')}</span>
							<span class="manual-space-of">{t('manual.of')} {formatGB(spaceTotalMB)} {t('unit.gb')}</span>
							<span class="manual-space-free">{t('manual.free')} {formatGB(spaceFreeMB)} {t('unit.gb')}</span>
						</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}

	<div class="top-bar-controls">
		<div class="fs-bar">
			<span class="fs-label">{t('page.diskSelection.fileSystem')}</span>
			<div class="fs-pills">
				{#each FILESYSTEMS as fs (fs)}
					<button
						type="button"
						class={['fs-pill', { selected: fileSystem === fs }]}
						onclick={() => onSelectFileSystem(fs)}
					>
						{t('fs.' + fs)}
					</button>
				{/each}
			</div>
		</div>

		<!-- Installation mode: automatic vs manual (dualboot) -->
		<div class="mode-bar">
			<span class="mode-label">{t('page.diskSelection.mode')}</span>
			<div class="mode-pills">
				<button
					type="button"
					class={['mode-pill', { selected: !manualMode }]}
					onclick={() => onToggleMode('automatic')}
				>
					{t('manual.modeAutomatic')}
				</button>
				<button
					type="button"
					class={['mode-pill', { selected: manualMode }]}
					onclick={() => onToggleMode('manual')}
				>
					{t('manual.modeManual')}
				</button>
			</div>
		</div>
	</div>
</div>
