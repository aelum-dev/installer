<script lang="ts">
	import type { DiskInfo, FileSystem, InstallMode } from '$lib/types';
	import { FILESYSTEMS } from '$lib/types';
	import { t } from '$lib/i18n';
	import { tooltip } from '$lib/tooltip.svelte';
	import ToggleSwitch from '$components/ToggleSwitch.svelte';
	import './StorageOptionsCard.css';

	/**
	 * The storage-options card in the page's info rail. It replaces the old
	 * DiskTopBar: filesystem, encryption, LVM, and the automatic/manual mode
	 * now live beside the page's identity instead of above the disk list.
	 *
	 * The four controls sit in two honest tiers, in both modes — a control
	 * that persists across modes never moves, so toggling changes only its own
	 * selected state. The two that CHANGE THE SHAPE of the result (Filesystem,
	 * Partitioning) are pill rows; the two that only WRAP it (Encrypt, LVM —
	 * their whole visible effect is the hatch and one extra word in the
	 * destination preview) share a switches band under the pills.
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
	const showWraps = $derived(showLuks || showLvm);

	// Reaching for manual mode without a disk gets the app tooltip's answer,
	// not a click: empty text means the shared tooltip stays closed the
	// moment a disk is selected.
	const manualHint = $derived(disk ? '' : t('page.diskSelection.selectDiskFirst'));
</script>

<div class="options-card">
	<div class="options-label">{t('page.diskSelection.options')}</div>

	<!-- Filesystem -->
	<div class="option-row fs-row">
		<span class="option-label">
			{t('page.diskSelection.fileSystem')}
			<span class="option-sub">{t('page.diskSelection.fsSub')}</span>
		</span>
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

	<!-- Installation mode: a two-state choice, not a feature toggle — so a
		segmented control. It changes the shape of the layout as much as the
		filesystem does, so it belongs beside the pills, not under the switches. -->
	<div class="option-row mode-row">
		<span class="option-label">
			{t('page.diskSelection.partitioning')}
			<span class="option-sub">
				{manualMode ? t('page.diskSelection.modeSubManual') : t('page.diskSelection.modeSubAutomatic')}
			</span>
		</span>
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
				use:tooltip={manualHint}
				onclick={() => onToggleMode('manual')}
			>
				{t('manual.modeManual')}
			</button>
		</div>
	</div>

	<!-- The wrapping band: Encrypt and LVM only wrap the layout the two rows
		above chose. LUKS always sits below LVM, so enabling both encrypts the
		single physical volume; in manual mode the designated System partition
		becomes the physical volume. -->
	{#if showWraps}
		<div class="wraps">
			{#if showLuks}
				<div class="option-row">
					<span class="option-label">
						{t('disk.encryptLabel')}
						<span class="option-sub">{t('disk.encryptSub')}</span>
					</span>
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
			{#if showLvm}
				<div class="option-row">
					<span class="option-label">
						{t('disk.lvmLabel')}
						<span class="option-sub">{t('disk.lvmSub')}</span>
					</span>
					<ToggleSwitch
						checked={lvmEnabled}
						label={t('disk.lvmLabel')}
						title={t('disk.lvmHint')}
						onToggle={() => onToggleLvm?.()}
					/>
				</div>
			{/if}
		</div>
	{/if}
</div>
