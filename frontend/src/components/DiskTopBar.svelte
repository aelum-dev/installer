<script lang="ts">
	import type { DiskInfo, FileSystem, InstallMode } from '$lib/types';
	import { FILESYSTEMS } from '$lib/types';
	import { mbToGB } from '$lib/format';
	import { t } from '$lib/i18n';
	import ToggleSwitch from '$components/ToggleSwitch.svelte';
	import './DiskTopBar.css';

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
</script>

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
							<span>{t('manual.allocated')} {mbToGB(spaceUsedMB)} {t('unit.gb')}</span>
							<span class="manual-space-of">{t('manual.of')} {mbToGB(spaceTotalMB)} {t('unit.gb')}</span>
							<span class="manual-space-free">{t('manual.free')} {mbToGB(spaceFreeMB)} {t('unit.gb')}</span>
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
		<!-- Installation mode: automatic vs manual (preserve other partitions) -->
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
			<ToggleSwitch
				checked={luksEnabled}
				label={t('disk.encryptLabel')}
				title={t('disk.encryptHint')}
				onToggle={() => {
					if (luksEnabled) {
						onDisableLuks?.();
					} else {
						onEnableLuks?.();
					}
				}}
			/>
			{#if onToggleLvm}
				<div class="toggle-group">
					<span class="control-row-label">{t('disk.lvmLabel')}</span>
					<ToggleSwitch
						checked={lvmEnabled}
						label={t('disk.lvmLabel')}
						title={t('disk.lvmHint')}
						onToggle={() => onToggleLvm?.()}
					/>
				</div>
			{/if}
		</div>
	{:else if onToggleLvm}
		<span class="control-row-label">{t('disk.lvmLabel')}</span>
		<div class="toggle-row">
			<ToggleSwitch
				checked={lvmEnabled}
				label={t('disk.lvmLabel')}
				title={t('disk.lvmHint')}
				onToggle={() => onToggleLvm?.()}
			/>
		</div>
	{/if}
</div>
</div>
