<script lang="ts">
	import { GetDisks, GetDiskPartitions } from '$lib/backend';
	import PageInfo from '$components/PageInfo.svelte';
	import StorageOptionsCard from '$components/StorageOptionsCard.svelte';
	import DestinationCard from '$components/DestinationCard.svelte';
	import DiskList from '$components/DiskList.svelte';
	import StateMessage from '$components/StateMessage.svelte';
	import ManualPartitionPage from '$pages/ManualPartitionPage.svelte';
	import LuksPassphraseDialog from '$components/LuksPassphraseDialog.svelte';
	import ConfirmDialog from '$components/ConfirmDialog.svelte';
	import type { PlanChoice } from '$lib/disk-layout';
	import { installState, requestEnableLuks, disableLuks, toggleLvm, setFileSystem } from '$lib/store.svelte';
	import { uiState } from '$lib/ui.svelte';
	import { inferDiskFileSystem } from '$lib/disks';
	import { type DiskInfo, type InstallMode, type PartitionEntry, PAGE_IDS } from '$lib/types';
	import { DefaultFileSystem } from '$lib/constants';
	import { t } from '$lib/i18n';
	import { glyph, STEP_ICONS } from '$lib/icons';
	import './DiskSelectionPage.css';

	function setMode(mode: InstallMode): void {
		installState.manualMode = mode === 'manual';
		// The pending "add partition" preview is not part of the plan; clear it
		// when leaving/entering manual mode. The layout itself is kept so the
		// EFI/System roles survive automatic <-> manual toggles.
		uiState.pendingAddMB = 0;
	}

	const pageIcon = glyph(STEP_ICONS.diskSelection);

	// The info rail's head is the page's identity in both modes — only the
	// words change. The rail is the wizard's fixed frame: the same head style,
	// the same options card, the same destination card below it, in both modes;
	// the content column is what Partitioning actually changes. The chosen
	// disk's identity lives in the destination card, which is where its layout
	// is previewed.
	const railTitle = $derived(
		installState.manualMode ? t('page.diskSelection.manualTitle') : t('page.diskSelection.title')
	);
	const railSubtitle = $derived(
		installState.manualMode ? t('page.diskSelection.manualSub') : t('page.diskSelection.subtitle')
	);

	// The live storage choices, in the shape every disk picture (row strips, the
	// destination card) draws its "what will be" from. Empty filesystem means
	// "not chosen yet", which is the default.
	const plan = $derived<PlanChoice>({
		fileSystem: installState.fileSystem || DefaultFileSystem,
		luks: installState.luksEnabled,
		lvm: installState.lvmEnabled,
	});

	let disks = $state<DiskInfo[]>([]);
	let partitionsByPath = $state<Record<string, PartitionEntry[]>>({});
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Identifies the latest load. A slower response than a retry that followed it
	// must not overwrite the current list.
	let loadRequest = 0;

	// Disabling encryption discards the passphrase, so confirm first.
	let showDisableConfirm = $state(false);

	// Every disk's partitions arrive with the disk list: the rows draw them as
	// strips, and a selection then needs no second round-trip.
	async function loadDisks(): Promise<void> {
		loading = true;
		error = null;
		const request = ++loadRequest;
		try {
			const list = (await GetDisks()) ?? [];
			const results = await Promise.all(
				list.map(async (d): Promise<[string, PartitionEntry[]]> => {
					try {
						return [d.path, (await GetDiskPartitions(d.path)) ?? []];
					} catch (err) {
						console.error(`[DiskSelectionPage] Failed to load partitions for ${d.path}: ${err}`);
						return [d.path, []];
					}
				})
			);
			// A newer load (a retry) is already in flight — drop this response.
			if (request !== loadRequest) return;

			const byPath: Record<string, PartitionEntry[]> = {};
			for (const [path, parts] of results) byPath[path] = parts;

			// A whole-disk filesystem lsblk reports only on the partitions gets
			// folded onto the disk so the row's strip and summary see it.
			for (const d of list) {
				const existingFstype = inferDiskFileSystem(d, byPath[d.path] ?? []);
				if (existingFstype && existingFstype !== d.fstype) d.fstype = existingFstype;
			}

			disks = list;
			partitionsByPath = byPath;

			// If a disk was already selected (navigated back from a later page),
			// restore it. Use adoptDisk, not selectDisk: selectDisk toggles the
			// current disk off, which would clear the selection we're restoring.
			if (installState.diskId) {
				const selected = list.find((d) => d.path === installState.diskId);
				if (selected) adoptDisk(selected);
			}
		} catch (err) {
			if (request !== loadRequest) return;
			error = `Failed to load disks: ${err instanceof Error ? err.message : String(err)}`;
		} finally {
			if (request === loadRequest) loading = false;
		}
	}

	// adoptDisk loads a disk into the wizard state. It never toggles: restoring a
	// selection on remount must keep the disk. The partitions the row previews
	// were already fetched with the disk list.
	function adoptDisk(disk: DiskInfo): void {
		// A saved manual plan belongs to one disk. Keep it when restoring the
		// same disk; discard it when the user picks a different one.
		if (installState.manualLayout && installState.manualLayout.disk !== disk.path) {
			installState.manualLayout = null;
			uiState.pendingAddMB = 0;
		}
		installState.diskId = disk.path;
		installState.disk = disk;
	}

	function selectDisk(disk: DiskInfo): void {
		// Toggle: clicking the already-selected disk deselects it again.
		if (installState.diskId === disk.path) {
			deselectDisk();
			return;
		}
		adoptDisk(disk);
	}

	function deselectDisk(): void {
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

<div class={['page-layout', 'disk-page', { 'disk-page--manual': installState.manualMode }]}>
	<PageInfo compact icon={pageIcon} title={railTitle} subtitle={railSubtitle}>
		{#snippet info()}
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

			<!-- The same card in the same place in both modes: the automatic
				 destination layout, or the manual plan itself — the identical
				 picture the content column draws. -->
			<DestinationCard
				disk={installState.disk}
				{plan}
				manual={installState.manualMode}
				layout={installState.manualLayout}
			/>
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
				{partitionsByPath}
				{plan}
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
