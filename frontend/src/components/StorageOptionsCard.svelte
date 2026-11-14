<script lang="ts">
	import type { DiskInfo, FileSystem, InstallMode } from '$lib/types';
	import { FILESYSTEMS } from '$lib/types';
	import { t } from '$lib/i18n';
	import ToggleSwitch from '$components/ToggleSwitch.svelte';
	import './StorageOptionsCard.css';

	/**
	 * The storage-options card in the page's info rail. It replaces the old
	 * DiskTopBar: filesystem, encryption, LVM, and the automatic/manual mode
	 * now live beside the page's identity instead of above the disk list.
	 *
	 * The row order is FIXED — Filesystem, Encrypt, LVM, Partitioning last —
	 * in both modes: a control that persists across modes never moves, so
	 * toggling changes only its own selected state. The first three rows are
	 * properties of the install plan; the last row chooses how the plan is
	 * edited, so it sits after and apart from them.
	 */
	let {
		disk,
		fileSystem,
		manualMode,
		luksEnabled = false,
		lvmEnabled = false,
		onEnableLuks,
		onDisableLuks,
		onToggleLvm,
		onSelectFileSystem,
		onToggleMode,
	}: {
		/** Selected disk, or null: the Manual segment stays disabled without one. */
		disk: DiskInfo | null;
		fileSystem: FileSystem | '';
		manualMode: boolean;
		luksEnabled?: boolean;
		lvmEnabled?: boolean;
		onEnableLuks?: () => void;
		onDisableLuks?: () => void;
		onToggleLvm?: () => void;
		onSelectFileSystem: (fs: FileSystem) => void;
		onToggleMode: (mode: InstallMode) => void;
	} = $props();

	const showLuks = $derived(Boolean(onEnableLuks || onDisableLuks));
	const showLvm = $derived(Boolean(onToggleLvm));

	// Info popup when the user reaches for manual mode without a disk selected.
	let showDiskHint = $state(false);

	function handleModeHover(isManual: boolean, entering: boolean): void {
		if (isManual && !disk) {
			showDiskHint = entering;
		}
	}
</script>

<div class="options-card">
	<div class="options-label">{t('page.diskSelection.options')}</div>

	<!-- Filesystem -->
	<div class="option-row">
		<span class="option-label">{t('page.diskSelection.fileSystem')}</span>
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

	<!-- Encryption (LUKS2) on its own row. LUKS always sits below LVM, so
		enabling both encrypts the single physical volume. -->
	{#if showLuks}
		<div class="option-row">
			<span class="option-label">{t('disk.encryptLabel')}</span>
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
		</div>
	{/if}

	<!-- LVM is independent of encryption. In manual mode the designated
		System partition becomes the physical volume. -->
	{#if showLvm}
		<div class="option-row">
			<span class="option-label">{t('disk.lvmLabel')}</span>
			<ToggleSwitch
				checked={lvmEnabled}
				label={t('disk.lvmLabel')}
				title={t('disk.lvmHint')}
				onToggle={() => onToggleLvm?.()}
			/>
		</div>
	{/if}

	<!-- Installation mode: a two-state choice, not a feature toggle — so a
		segmented control, and always the card's last row. -->
	<div class="option-row mode-row">
		<span class="option-label">{t('page.diskSelection.partitioning')}</span>
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
				disabled={!disk}
				onmouseenter={() => handleModeHover(true, true)}
				onmouseleave={() => handleModeHover(true, false)}
				onclick={() => onToggleMode('manual')}
			>
				{t('manual.modeManual')}
			</button>
		</div>
		<!-- Tooltip: "Select a disk first" -->
		{#if showDiskHint}
			<div class="disk-hint-tooltip">
				<span class="disk-hint-tooltip-text">{t('page.diskSelection.selectDiskFirst')}</span>
			</div>
		{/if}
	</div>
</div>
