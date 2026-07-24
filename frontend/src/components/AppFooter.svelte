<script lang="ts">
	import Button from '$components/Button.svelte';
	import { PAGE_IDS, MIN_DISK_SIZE_BYTES } from '$lib/types';
	import { diskSelectionData } from '$lib/state.svelte';

	let { currentPage }: { currentPage: number } = $props();

	const isLastPage = $derived(currentPage === PAGE_IDS.FINISH);
	const isWelcomePage = $derived(currentPage === PAGE_IDS.WELCOME);
	const isInstallationPage = $derived(currentPage === PAGE_IDS.INSTALLATION);
	const showBack = $derived(currentPage > PAGE_IDS.WELCOME && !isLastPage && !isInstallationPage);
	const showNext = $derived(!isLastPage && !isWelcomePage && !isInstallationPage);
	const diskSizeOk = $derived.by(() => {
		if (!diskSelectionData.disk) return false;
		const size = parseInt(diskSelectionData.disk.size, 10);
		return !isNaN(size) && size >= MIN_DISK_SIZE_BYTES;
	});
	const nextDisabled = $derived(currentPage === PAGE_IDS.CONFIRMATION && !diskSizeOk);

	const buttonHandler = $derived(isLastPage ? () => {
		document.dispatchEvent(new CustomEvent('finish', { cancelable: true }));
	} : () => {
		document.dispatchEvent(new CustomEvent('go-next', { cancelable: true }));
	});
</script>

<footer class="footer">
	<div class="footer-left">
		{#if showBack}
			<Button variant="ghost" onClick={() => document.dispatchEvent(new CustomEvent('go-back', { cancelable: true }))}>← Back</Button>
		{/if}
	</div>
	<div class="footer-right">
		{#if showNext}
			<Button variant="secondary" onClick={buttonHandler} disabled={nextDisabled}>Next →</Button>
		{:else if isLastPage}
			<Button variant="secondary" onClick={buttonHandler}>Reboot</Button>
		{/if}
	</div>
</footer>

<style>
	.footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--spacing-md) var(--spacing-xl);
		min-height: var(--footer-height);
		background-color: var(--color-bg-footer);
	}

	.footer-left,
	.footer-right {
		display: flex;
		align-items: center;
		gap: 8px;
	}
</style>
