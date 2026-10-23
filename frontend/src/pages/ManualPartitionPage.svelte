<script lang="ts">
	import type { DiskInfo, ManualPartition, PartitionType } from '$lib/types';
	import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
	import { GetManualLayout, SaveManualLayout } from '$lib/backend';
	import { installState } from '$lib/store.svelte';
	import { uiState } from '$lib/ui.svelte';
	import {
		addPartition as appendPartition,
		computeFreeMB,
		nextPartitionIndex,
		normalizeManualLayout,
		removePartition,
		withPartitionType,
	} from '$lib/manual-layout';
	import { t } from '$lib/i18n';
	import { glyph } from '$lib/icons';
	import Chevron from '$components/Chevron.svelte';
	import { groupDigits } from '$lib/format';
	import ConfirmDialog from '$components/ConfirmDialog.svelte';
	import RoleSelect from '$components/RoleSelect.svelte';
	import NumberStepper from '$components/NumberStepper.svelte';
	import './ManualPartitionPage.css';

	let { disk }: { disk: DiskInfo | null } = $props();

	// The plan lives in the store (single owner); this page edits it through the
	// immutable helpers in manual-layout.ts and reads it back as a derived view.
	const layout = $derived(installState.manualLayout);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let saving = $state(false);
	let saveQueued = $state(false);

	// Delete confirmation target (partition index), or null when closed.
	let deleteTargetIndex = $state<number | null>(null);

	// Visibility of the shared delete-confirmation dialog.
	let showDeleteDialog = $state(false);

	// "Add new partition" controls. New partitions start blank (Preserve) and are
	// designated Boot/Root from the row pills afterwards.
	let addSizeMB = $state<number>(1);
	let addUseRemaining = $state(false);

	// Whether the add-new-partition control is shown. It only appears once the
	// user clicks the "unallocated space" row in the partition editor, and
	// disappears again on a second click (toggling off).
	let showAddControl = $state(false);

	// Live reference to the range input so the value-reassert effect below can keep
	// it on a valid value when its `max` shifts (e.g. after adding/deleting a row).
	let sliderEl = $state<HTMLInputElement | null>(null);

	// Live reference to the scrollable partition-editor so the reveal effect can
	// scroll it to the bottom when the add-control opens (keeping the FREE row in
	// view instead of letting it scroll out behind the newly shown control).
	let editorEl = $state<HTMLElement | null>(null);

	// Re-assert a valid slider value whenever available space changes (adding or
	// deleting a partition makes `availableAddMB` jump, which moves the range's
	// `max`). A bound range left with a shifted `max` can stay stuck at its
	// browser-computed midpoint, and since `addSizeMB` itself doesn't change value
	// here Svelte's bind:value won't re-write the DOM to unstick it. Writing a
	// valid number directly fixes it; this never fires an input event, so it
	// doesn't fight user drags or the shared binding.
	$effect(() => {
		void availableAddMB;
		if (!sliderEl || addUseRemaining) return;
		// Clamp to the slider's own max (not just availableAddMB) so a large
		// typed value the slider is allowed to reach isn't pulled back down.
		const sliderMax = Math.max(availableAddMB, addSizeMB || 0);
		const next = Math.min(Math.max(addSizeMB || 0, 1), sliderMax);
		if (sliderEl.value !== String(next)) {
			sliderEl.value = String(next);
		}
	});

	// Raw unallocated space shown in the editor's unallocated-space row and fully
	// available for new partitions. "Use remaining" and the slider both consume it
	// in full — nothing is held back at the end of the disk.
	const unallocatedMB = $derived.by(() => (layout ? computeFreeMB(layout) : 0));
	const availableAddMB = $derived.by(() => (layout ? Math.max(0, unallocatedMB) : 0));

	// Whether "Add" stays enabled: usable space exists, and either "Use remaining"
	// is on, or an explicit size falls within (0, availableAddMB]. Replaces the old
	// inline too-small error with a disabled button.
	const canAdd = $derived.by(() => {
		if (availableAddMB <= 0) return false;
		if (addUseRemaining) return true;
		const size = addSizeMB || 0;
		return size > 0 && size <= availableAddMB;
	});


	// Identifies the latest layout request, so a slow response for a previous
	// disk can't overwrite the current one.
	let loadRequest = 0;

	async function loadLayout(path: string): Promise<void> {
		const request = ++loadRequest;
		loading = true;
		error = null;

		// Re-use the plan already held in the store when it belongs to this disk.
		// This keeps the user's EFI/System designations (and any added/deleted
		// rows) when switching automatic <-> manual or navigating back.
		const saved = installState.manualLayout;
		if (saved && saved.disk === path) {
			installState.manualLayout = normalizeManualLayout(saved);
			loading = false;
			return;
		}

		try {
			const fetched = normalizeManualLayout(await GetManualLayout(path));
			if (request !== loadRequest) return;
			installState.manualLayout = fetched;
		} catch (err) {
			if (request !== loadRequest) return;
			error = `Failed to load partition layout: ${err instanceof Error ? err.message : String(err)}`;
		} finally {
			if (request === loadRequest) loading = false;
		}
	}

	// Persist the current layout to the backend app state. Saves are serialized:
	// rapid edits previously fired overlapping calls, and an older call could
	// resolve last and overwrite the newest layout. While a save is in flight,
	// further edits set `saveQueued` and are picked up by the running loop. Each
	// pass reads the store again, so the newest layout always wins.
	async function persist(): Promise<void> {
		if (!installState.manualLayout) return;
		if (saving) {
			saveQueued = true;
			return;
		}

		saving = true;
		try {
			do {
				saveQueued = false;
				const current = installState.manualLayout;
				if (!current) break;
				await SaveManualLayout(current);
			} while (saveQueued);
		} catch (err) {
			error = `Failed to save layout: ${err instanceof Error ? err.message : String(err)}`;
		} finally {
			saving = false;
		}
	}

	function setType(part: ManualPartition, type: PartitionType): void {
		if (!layout) return;
		// Clicking the already-active type clears it back to Preserve, so a role can
		// be undone. An invalid layout (no ESP or no System) simply keeps "Next"
		// disabled until a valid combination is chosen again.
		const next = part.type === type ? '' : type;
		installState.manualLayout = withPartitionType(layout, part.index, next);
		void persist();
	}

	// Every partition is deletable: the delete-confirmation dialog is the safety
	// gate, so there's no need to force the user to assign a role first just to
	// remove a partition.


	function addPartition(): void {
		if (!layout || !disk) return;
		const sizeMB = addUseRemaining ? availableAddMB : (addSizeMB || 0);
		// Guarded by the disabled Add button (canAdd); kept as a silent safety net so a
		// stray value can never create a zero- or oversized partition.
		if (sizeMB <= 0 || sizeMB > availableAddMB) return;
		const newPart: ManualPartition = {
			index: nextPartitionIndex(layout),
			path: '',
			sizeMB,
			type: '',
			existingFstype: '',
		};
		installState.manualLayout = appendPartition(layout, newPart);
		void persist();
		// Hide the control once there's no space left to add from, so it doesn't
		// linger with nothing to do after a full ("Use remaining") add.
		if (availableAddMB <= 0) showAddControl = false;
		// Reset add controls back to the default size (1 MB) so the next partition
		// starts from the same baseline instead of a blank/0 field.
		addSizeMB = 1;
		addUseRemaining = false;
		// Only clear the error once a partition is actually added.
		if (sizeMB > 0) error = null;
	}

	function requestDelete(index: number): void {
		deleteTargetIndex = index;
		showDeleteDialog = true;
	}

	function confirmDelete(): void {
		if (layout === null || deleteTargetIndex === null) return;
		// Move the row into deletedPartitions (keeping its path/number) rather than
		// dropping it, so Apply can delete it from the disk at install time. The row
		// disappears from the table immediately, and its space is freed for new
		// partitions.
		installState.manualLayout = removePartition(layout, deleteTargetIndex);
		void persist();
		deleteTargetIndex = null;
	}

	// Reload whenever the selected disk changes. Depend on the path (not the
	// whole disk object) so unrelated mutations like fstype inference don't
	// trigger a reload.
	$effect(() => {
		const path = disk?.path;
		if (!path) {
			loadRequest++;
			installState.manualLayout = null;
			loading = false;
			return;
		}
		void loadLayout(path);
	});

	// When the add-control is revealed, scroll the partition editor to its bottom
	// so the FREE row the user clicked stays visible at the base of the scrollable
	// area instead of being pushed out of view by the newly shown control.
	$effect(() => {
		void showAddControl;
		if (showAddControl && editorEl) {
			editorEl.scrollTop = editorEl.scrollHeight;
		}
	});

	// Live-preview the pending "add partition" in the top-bar space bar. Drags on
	// the slider update this before the partition is committed, so used/free
	// reflect what the next Add will produce. Clamped to available free space.
	$effect(() => {
		const pending = addUseRemaining ? availableAddMB : (addSizeMB || 0);
		uiState.pendingAddMB = Math.max(0, Math.min(pending, availableAddMB));
	});
</script>

<div class="manual">
	{#if error}
		<div class="state-message error">
			<p>{error}</p>
		</div>
	{/if}

	{#if loading}
		<div class="state-message">
			<div class="spinner"></div>
			<p>{t('page.diskSelection.scanPartitions')}</p>
		</div>
	{:else if !layout}
		<div class="state-message">
			<p>{t('manual.noPartitions')}</p>
		</div>
	{:else}
		<!-- Partition table -->
		<div class="partition-editor" bind:this={editorEl}>
			<table class="editor-table">
				<thead>
					<tr>
						<th>{t('disk.name')}</th>
						<th>{t('disk.sizeMB')}</th>
						<th>{t('manual.typeCol')}</th>
						<th class="text-right"></th>
					</tr>
				</thead>
				<tbody>
					{#each layout.partitions as part (part.index)}
						<tr class={['editor-row', { designated: part.type === TYPE_ESP || part.type === TYPE_SYSTEM }]}>
							<td class="part-name">
								{#if part.path}
									<span class="part-path">{part.path}</span>
									{#if part.type === TYPE_ESP || part.type === TYPE_SYSTEM}
										<span class="format-hint">{t('manual.formatHint')}</span>
									{/if}
								{:else}
									<span class="part-new">{t('manual.newPart')}</span>
								{/if}
							</td>
							<td class="part-size">{groupDigits(part.sizeMB)}</td>
							<td class="part-type">
							<RoleSelect value={part.type} onChange={(v: PartitionType) => setType(part, v)} />
							</td>
							<td class="text-right">
								<button
									type="button"
									class="delete-btn"
												onclick={() => requestDelete(part.index)}
									aria-label={t('manual.delete')}
								>
									{glyph('ea81')}
								</button>
							</td>
						</tr>
					{/each}
				{#if unallocatedMB > 0}
					<tr
						class={['unallocated-row', { 'is-active': showAddControl }]}
						onclick={() => (showAddControl = !showAddControl)}
						role="button"
						tabindex="0"
						onkeydown={(e) => {
							if (e.key === 'Enter' || e.key === ' ') {
								e.preventDefault();
								showAddControl = !showAddControl;
							}
						}}
					>
						<td class="part-name">
							<span class="unallocated-label">{t('manual.free')}</span>
						</td>
						<td class="part-size">{groupDigits(unallocatedMB)} {t('unit.mb')}</td>
						<td></td>
						<td class="text-right">
							<Chevron class="unallocated-indicator" direction={showAddControl ? 'up' : 'down'} />
						</td>
					</tr>
				{/if}
			</tbody>
		</table>
		</div>

		<!-- Add new partition control: shown only after the user clicks the FREE
			row (showAddControl). The FREE row itself is gated on unallocatedMB > 0,
			so the control can only ever be turned on while space exists; the Add
			button stays disabled when none is left. -->
		{#if showAddControl}
		<div class="add-control">
			<input
				type="range"
				class="add-slider"
				min="1"
				max={Math.max(availableAddMB, addSizeMB || 0)}
				bind:this={sliderEl}
				step="1"
				bind:value={addSizeMB}
				disabled={addUseRemaining}
			/>
			<NumberStepper
				bind:value={addSizeMB}
				min={1}
				max={availableAddMB}
				disabled={addUseRemaining}
				placeholder={t('manual.sizeMB')}
			/>
			<span class="add-unit">{t('unit.mb')}</span>
			<label class="add-check">
				<input type="checkbox" bind:checked={addUseRemaining} />
				<span class="add-switch" aria-hidden="true"><span class="add-switch-knob"></span></span>
				<span>{t('manual.useRemaining')}</span>
			</label>
			<button
				type="button"
				class="add-btn"
				disabled={!canAdd}
				onclick={addPartition}
			>
				+ {t('manual.add')}
			</button>
		</div>
		{/if}
	{/if}

	<!-- Delete confirmation dialog (shared component, custom text) -->
	<ConfirmDialog
		bind:show={showDeleteDialog}
		title={t('manual.deleteTitle')}
		message={t('manual.deleteMessage')}
		confirmLabel={t('manual.deleteConfirm')}
		onConfirm={confirmDelete}
		onCancel={() => { deleteTargetIndex = null; }}
	/>
</div>
