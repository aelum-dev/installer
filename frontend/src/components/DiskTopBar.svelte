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

	// Show info popup when user tries to switch to manual without a disk selected.
	let showDiskHint = $state(false);

	function handleModeClick(mode: InstallMode): void {
		if (mode === 'manual' && !disk) {
			showDiskHint = true;
			return;
		}
		onToggleMode(mode);
	}

	// Group digits with a space every 3 (e.g. 1500 -> "1 500") for readability.
	function formatMB(mb: number): string {
		return String(mb).replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
	}

	// Convert MB to GB and format with one decimal (e.g. 1536 MB -> "1.5 GB").
	function formatGB(mb: number): number {
		const gb = mb / 1024;
		return Math.round(gb * 10) / 10;
	}
</script>

<div class="top-bar">
	{#if manualMode && disk}
		<div class="manual-title">
			<span class="nf-icon page-icon" aria-hidden="true">{pageIcon}</span>
			<div class="manual-title-content">
				<h2>{disk.model || disk.name || '—'}</h2>
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
					onclick={() => handleModeClick('automatic')}
				>
					{t('manual.modeAutomatic')}
				</button>
				<button
					type="button"
					class={['mode-pill', { selected: manualMode }]}
					disabled={!disk}
					onclick={() => handleModeClick('manual')}
				>
					{t('manual.modeManual')}
				</button>
			</div>
		</div>
	</div>

	<!-- Info popup: "Select a disk first" -->
	{#if showDiskHint}
		<div
			class="disk-hint-overlay"
			onclick={() => { showDiskHint = false; }}
			onkeydown={(e) => { if (e.key === 'Escape') showDiskHint = false; }}
		>
			<div
				class="disk-hint"
				onclick={(e) => e.stopPropagation()}
				onkeydown={(e) => e.stopPropagation()}
			>
				<div class="disk-hint-icon" aria-hidden="true">
					<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
						<circle cx="12" cy="12" r="10" stroke="var(--color-primary)" stroke-width="1.5"/>
						<line x1="12" y1="8" x2="12" y2="13" stroke="var(--color-primary)" stroke-width="1.5" stroke-linecap="round"/>
						<circle cx="12" cy="16.5" r="1" fill="var(--color-primary)"/>
					</svg>
				</div>
				<h3 class="disk-hint-title">{t('page.diskSelection.selectDiskFirst')}</h3>
				<p class="disk-hint-message">{t('page.diskSelection.selectDiskFirst')}</p>
				<button
					type="button"
					class="disk-hint-close"
					onclick={() => { showDiskHint = false; }}
				>
					{t('dialog.confirm.cancel')}
				</button>
			</div>
		</div>
	{/if}
</div>
