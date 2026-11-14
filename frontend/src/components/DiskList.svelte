<script lang="ts">
	import type { DiskInfo } from '$lib/types';
	import { t } from '$lib/i18n';
	import Button from '$components/Button.svelte';
	import ScrollBox from '$components/ScrollBox.svelte';
	import DiskCard from '$components/DiskCard.svelte';
	import StateMessage from '$components/StateMessage.svelte';
	import './DiskList.css';

	let {
		loading,
		error,
		disks,
		diskId,
		persistKey,
		onSelectDisk,
		onRetry
	}: {
		loading: boolean;
		error: string | null;
		disks: DiskInfo[];
		diskId: string;
		persistKey?: string | number;
		onSelectDisk: (disk: DiskInfo) => void;
		onRetry: () => void;
	} = $props();
</script>

{#if loading}
	<StateMessage>
		<p>{t('page.diskSelection.loading')}</p>
	</StateMessage>
{:else if error}
	<StateMessage error>
		<p>{error}</p>
		<Button variant="secondary" onclick={onRetry}>{t('action.retry')}</Button>
	</StateMessage>
{:else if disks.length === 0}
	<StateMessage>
		<p>{t('page.diskSelection.noDevices')}</p>
	</StateMessage>
{:else}
	<div class="right-content">
		<!-- The same kind of list label the language list carries. -->
		<div class="list-label">{t('page.diskSelection.devices')}</div>
		<ScrollBox {persistKey}>
			<div class="disk-list">
				{#each disks as disk (disk.path)}
					<DiskCard
						{disk}
						selected={diskId === disk.path}
						onSelect={onSelectDisk}
					/>
				{/each}
			</div>
		</ScrollBox>
	</div>
{/if}
