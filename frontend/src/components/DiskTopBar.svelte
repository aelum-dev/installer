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
		luksEnabled = false,
		onEnableLuks,
		onDisableLuks,
		lvmEnabled = false,
		onToggleLvm,
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
		luksEnabled?: boolean;
		onEnableLuks?: () => void;
		onDisableLuks?: () => void;
		lvmEnabled?: boolean;
		onToggleLvm?: () => void;
		onSelectFileSystem: (fs: FileSystem) => void;
		onToggleMode: (mode: InstallMode) => void;
	} = $props();

	// Show info popup when user tries to switch to manual without a disk selected.
	let showDiskHint = $state(false);

	function handleModeClick(mode: InstallMode): void {
		onToggleMode(mode);
	}

	function handleModeHover(isManual: boolean, entering: boolean): void {
		if (isManual && !disk) {
			showDiskHint = entering;
		}
	}

	// Convert MB to GB and format with one decimal (e.g. 1536 MB -> "1.5 GB").
	function formatGB(mb: number): number {
		const gb = mb / 1024;
		return Math.round(gb * 10) / 10;
	}
</script>

<!-- LUKS2 root encryption toggle. Off → opens the passphrase popup; on → opens
	the disable confirm. Reflects luksEnabled only. -->
{#snippet luksSwitch()}
	<button
		type="button"
		class={['luks-switch', { on: luksEnabled }]}
		role="switch"
		aria-checked={luksEnabled}
		aria-label={t('disk.encryptLabel')}
		title={t('disk.encryptHint')}
		onclick={() => {
			if (luksEnabled) {
				onDisableLuks?.();
			} else {
				onEnableLuks?.();
			}
		}}
	>
		<span class="luks-slider"></span>
	</button>
{/snippet}

<!-- LVM toggle: place root and home on logical volumes in the `aelum` volume
	group. No passphrase (unlike LUKS). -->
{#snippet lvmSwitch()}
	<button
		type="button"
		class={['luks-switch', { on: lvmEnabled }]}
		role="switch"
		aria-checked={lvmEnabled}
		aria-label={t('disk.lvmLabel')}
		title={t('disk.lvmHint')}
		onclick={() => onToggleLvm?.()}
	>
		<span class="luks-slider"></span>
	</button>
{/snippet}

<div class={['top-bar', { 'top-bar--auto': !manualMode }]}>
	<!-- Left slot: disk title in manual mode; nothing in automatic mode. -->
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
			<p class="manual-title-hint">{t('manual.continueHint')}</p>
		</div>
	{/if}

	<!-- Right slot: mode + fs + encryption controls.
		Laid out as a two-column grid so every label lines up on the left and every
		pill/toggle lines up on one vertical column (grid synchronises the column
		widths across rows). -->
	<div class="top-bar-controls">
		<!-- Installation mode: automatic vs manual (dualboot) -->
		<span class="control-row-label">{t('page.diskSelection.mode')}</span>
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
				onmouseenter={() => handleModeHover(true, true)}
				onmouseleave={() => handleModeHover(true, false)}
				onclick={() => handleModeClick('manual')}
			>
				{t('manual.modeManual')}
			</button>
		<!-- Tooltip: "Select a disk first" -->
		{#if showDiskHint}
			<div class="disk-hint-tooltip">
				<span class="disk-hint-tooltip-text">{t('page.diskSelection.selectDiskFirst')}</span>
			</div>
		{/if}
	</div>

	<span class="control-row-label">{t('page.diskSelection.fileSystem')}</span>
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

	<!-- Encryption (LUKS2) and LVM toggles share the control column (column 2),
		so their switches line up with the mode/filesystem pills above. The label
		for the first toggle occupies column 1; the LVM label sits inline before its
		switch. Shown in both automatic and manual mode. -->
	{#if onEnableLuks || onDisableLuks}
		<span class="control-row-label">{t('disk.encryptLabel')}</span>
		<div class="toggle-row">
			{@render luksSwitch()}
			{#if onToggleLvm}
				<div class="toggle-group">
					<span class="control-row-label">{t('disk.lvmLabel')}</span>
					{@render lvmSwitch()}
				</div>
			{/if}
		</div>
	{:else if onToggleLvm}
		<span class="control-row-label">{t('disk.lvmLabel')}</span>
		<div class="toggle-row">
			{@render lvmSwitch()}
		</div>
	{/if}
</div>
</div>
