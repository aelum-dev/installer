<script lang="ts">
	import { GetDisks, GetDiskPartitions } from '$lib/backend';
	import PageInfo from '$components/PageInfo.svelte';
	import StorageOptionsCard from '$components/StorageOptionsCard.svelte';
	import DiskList from '$components/DiskList.svelte';
	import StateMessage from '$components/StateMessage.svelte';
	import ManualPartitionPage from '$pages/ManualPartitionPage.svelte';
	import LuksPassphraseDialog from '$components/LuksPassphraseDialog.svelte';
	import ConfirmDialog from '$components/ConfirmDialog.svelte';
	import { installState, requestEnableLuks, disableLuks, toggleLvm, setFileSystem } from '$lib/store.svelte';
	import { uiState } from '$lib/ui.svelte';
	import { computeUsedMB, computeFreeMB } from '$lib/manual-layout';
	import { inferDiskFileSystem } from '$lib/disks';
	import { formatBytes, mbToGB } from '$lib/format';
	import { type DiskInfo, type InstallMode, PAGE_IDS } from '$lib/types';
	import { DefaultFileSystem, MinDiskSizeBytes } from '$lib/constants';
	import { t } from '$lib/i18n';
	import { glyph, STEP_ICONS, diskGlyph } from '$lib/icons';
	import './DiskSelectionPage.css';

	function setMode(mode: InstallMode): void {
		installState.manualMode = mode === 'manual';
		// The pending "add partition" preview is not part of the plan; clear it
		// when leaving/entering manual mode. The layout itself is kept so the
		// EFI/System roles survive automatic <-> manual toggles.
		uiState.pendingAddMB = 0;
	}

	const pageIcon = glyph(STEP_ICONS.diskSelection);

	// Allocated / free space summary for the manual-mode rail. Derived from the
	// shared manual layout, which ManualPartitionPage keeps in sync.
	const spaceSummary = $derived.by(() => {
		const layout = installState.manualLayout;
		if (!layout) return { used: 0, total: 0, free: 0, pct: 0 };
		const used = computeUsedMB(layout);
		const total = layout.diskSizeMB;
		// Fold in the pending "add partition" (from the manual page's size slider)
		// so the bar previews the change before the partition is committed.
		const pending = Math.max(0, Math.min(uiState.pendingAddMB, computeFreeMB(layout)));
		const usedPreview = used + pending;
		const freePreview = Math.max(computeFreeMB(layout) - pending, 0);
		return { used: usedPreview, total, free: freePreview, pct: total > 0 ? Math.min(100, Math.round((usedPreview / total) * 100)) : 0 };
	});

	// The info rail's head is mode-aware: automatic mode shows the page's
	// identity, manual mode shows the selected disk's — the disk IS the page's
	// subject once its partitions are being edited. The options card below never
	// moves, so toggling modes changes only the head and the content column.
	const manualDisk = $derived(installState.manualMode ? installState.disk : null);
	const railIcon = $derived(manualDisk ? diskGlyph(manualDisk) : pageIcon);
	const railTitle = $derived(manualDisk ? manualDisk.model || manualDisk.name || '—' : t('page.diskSelection.title'));
	const railSubtitle = $derived(
		manualDisk
			? `${manualDisk.path} · ${manualDisk.sizeFormatted}`
			: t('page.diskSelection.subtitle')
	);

	const minDiskSizeFormatted = $derived(formatBytes(MinDiskSizeBytes));

	let disks = $state<DiskInfo[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Identifies the latest partition request. A slower response for a disk the
	// user has already navigated away from must not overwrite the current one.
	let partitionsRequest = 0;

	// Disabling encryption discards the passphrase, so confirm first.
	let showDisableConfirm = $state(false);

	async function loadDisks(): Promise<void> {
		loading = true;
		error = null;
		try {
			disks = (await GetDisks()) ?? [];
		} catch (err) {
			error = `Failed to load disks: ${err instanceof Error ? err.message : String(err)}`;
		} finally {
			loading = false;

			// If a disk was already selected (navigated back from a later page),
			// restore it. Use adoptDisk, not selectDisk: selectDisk toggles the
			// current disk off, which would clear the selection we're restoring.
			if (installState.diskId && disks.length > 0) {
				const selected = disks.find((d) => d.path === installState.diskId);
				if (selected) await adoptDisk(selected);
			}
		}
	}

	// adoptDisk loads a disk into the wizard state and fetches its partitions.
	// It never toggles: restoring a selection on remount must keep the disk.
	// The partitions themselves are only read here (inferDiskFileSystem) — the
	// row list never renders them.
	async function adoptDisk(disk: DiskInfo): Promise<void> {
		// A saved manual plan belongs to one disk. Keep it when restoring the
		// same disk; discard it when the user picks a different one.
		if (installState.manualLayout && installState.manualLayout.disk !== disk.path) {
			installState.manualLayout = null;
			uiState.pendingAddMB = 0;
		}
		installState.diskId = disk.path;
		installState.disk = disk;

		const request = ++partitionsRequest;
		try {
			const parts = (await GetDiskPartitions(disk.path)) ?? [];
			// A newer selection (or a deselect) happened while this request was
			// in flight — drop the stale result instead of clobbering current state.
			if (request !== partitionsRequest) return;

			const existingFstype = inferDiskFileSystem(disk, parts);
			if (existingFstype && existingFstype !== disk.fstype) {
				disk.fstype = existingFstype;
				installState.disk = disk;
			}
		} catch (err) {
			console.error(`[DiskSelectionPage] Failed to load partitions: ${err}`);
		}
	}

	async function selectDisk(disk: DiskInfo): Promise<void> {
		// Toggle: clicking the already-selected disk deselects it again.
		if (installState.diskId === disk.path) {
			deselectDisk();
			return;
		}
		await adoptDisk(disk);
	}

	function deselectDisk(): void {
		partitionsRequest++;
		installState.diskId = "";
		installState.disk = null;
		installState.fileSystem = DefaultFileSystem;
		installState.manualLayout = null;
		uiState.pendingAddMB = 0;
		installState.manualMode = false;
	}

	loadDisks();

	// Never get stuck in manual mode with no disk selected. Manual mode is only
	// reachable with a disk (the Manual segment is disabled otherwise), so a
	// diskless manual state is stale — e.g. after deselecting the disk, or
	// navigating back to the user page and forward again. Fall back to automatic
	// mode so the disk list shows instead of the dead-end "select a disk" hint.
	$effect(() => {
		if (installState.manualMode && !installState.disk) {
			installState.manualMode = false;
			installState.manualLayout = null;
		}
	});
</script>

<div class="page-layout">
	<PageInfo icon={railIcon} title={railTitle} subtitle={railSubtitle}>
		{#snippet info()}
			{#if manualDisk && spaceSummary.total > 0}
				<!-- One honest number: the free space. The total is already in the
					 identity line above, so it is not repeated here. -->
				<div class="rail-space">
					<div
						class="rail-space-bar"
						role="progressbar"
						aria-valuenow={spaceSummary.pct}
						aria-valuemin={0}
						aria-valuemax={100}
					>
						<div class="rail-space-used" style="--pct: {spaceSummary.pct}%"></div>
					</div>
					<div class="rail-space-label">
						{t('manual.free')} {mbToGB(spaceSummary.free)} {t('unit.gb')}
					</div>
				</div>
			{/if}

			<StorageOptionsCard
				disk={installState.disk}
				fileSystem={installState.fileSystem}
				manualMode={installState.manualMode}
				luksEnabled={installState.luksEnabled}
				onEnableLuks={requestEnableLuks}
				onDisableLuks={() => { showDisableConfirm = true; }}
				lvmEnabled={installState.lvmEnabled}
				onToggleLvm={toggleLvm}
				onSelectFileSystem={(fs) => { setFileSystem(fs); }}
				onToggleMode={setMode}
			/>

			{#if !installState.manualMode}
				<p class="page-hint">{t('page.diskSelection.minDiskSize', { minSize: minDiskSizeFormatted })}</p>
			{/if}
		{/snippet}
	</PageInfo>

	<div class="page-content">
		{#if installState.manualMode}
			<!-- Manual mode: full-width partition editor -->
			{#if installState.disk}
				<ManualPartitionPage disk={installState.disk} />
			{:else}
				<div class="manual-select-hint">
					<StateMessage>
						<p>{t('page.diskSelection.selectDiskManual')}</p>
					</StateMessage>
				</div>
			{/if}
		{:else}
			<!-- Automatic mode: the disk list, and nothing else. -->
			<DiskList
				loading={loading}
				{error}
				{disks}
				diskId={installState.diskId}
				persistKey={PAGE_IDS.DISK_SELECTION}
				onSelectDisk={selectDisk}
				onRetry={loadDisks}
			/>
		{/if}
	</div>
</div>

	<!-- LUKS passphrase entry popup (shown in both automatic and manual mode) -->
	<LuksPassphraseDialog bind:show={uiState.luksDialogOpen} />

	<!-- Confirm before discarding encryption -->
	<ConfirmDialog
		bind:show={showDisableConfirm}
		title={t('luks.disable.title')}
		message={t('luks.disable.message')}
		onConfirm={disableLuks}
	/>
