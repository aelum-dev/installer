<script lang="ts">
	import { t } from '$lib/i18n';
	import PageInfo from '$components/PageInfo.svelte';
	import './DiskInfoSidebar.css';

	let { pageIcon, minDiskSizeFormatted, hidden = false }: {
		pageIcon: string;
		minDiskSizeFormatted: string;
		hidden?: boolean;
	} = $props();
</script>

<PageInfo
	icon={pageIcon}
	title={t('page.diskSelection.title')}
	subtitle={t('page.diskSelection.subtitle')}
	{hidden}
>
	{#snippet info()}
		<p class="page-hint">{t('page.diskSelection.minDiskSize', { minSize: minDiskSizeFormatted })}</p>
	{/snippet}
</PageInfo>
