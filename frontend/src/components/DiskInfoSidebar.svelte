<script lang="ts">
	import { t } from '$lib/i18n';
	import './DiskInfoSidebar.css';

	let { pageIcon, minDiskSizeFormatted, hidden = false }: {
		pageIcon: string;
		minDiskSizeFormatted: string;
		hidden?: boolean;
	} = $props();
</script>

<div class={['page-info', { 'manual-hidden': hidden }]}>
	<span class="nf-icon page-icon" aria-hidden="true">{pageIcon}</span>
	<h2>{t('page.diskSelection.title')}</h2>
	<p>{t('page.diskSelection.subtitle')}</p>
	<p class="page-hint">{t('page.diskSelection.minDiskSize', { minSize: minDiskSizeFormatted })}</p>
</div>
