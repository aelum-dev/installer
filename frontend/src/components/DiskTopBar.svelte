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
							<span>{t('manual.allocated')} {Math.round(spaceUsedMB / 1024)} {t('unit.gb')}</span>
							<span class="manual-space-of">{t('manual.of')} {Math.round(spaceTotalMB / 1024)} {t('unit.gb')}</span>
							<span class="manual-space-free">{t('manual.free')} {Math.round(spaceFreeMB / 1024)} {t('unit.gb')}</span>
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
