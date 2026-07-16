<script lang="ts">
	import Button from '$components/Button.svelte';
	import { PAGE_IDS } from '$lib/types';

	let { currentPage }: { currentPage: number } = $props();

	const isLastPage = $derived(currentPage === PAGE_IDS.FINISH);
	const isWelcomePage = $derived(currentPage === PAGE_IDS.WELCOME);
	const isInstallationPage = $derived(currentPage === PAGE_IDS.INSTALLATION);
	const showBack = $derived(currentPage > PAGE_IDS.WELCOME && !isLastPage && !isInstallationPage);
	const showNext = $derived(!isLastPage && !isWelcomePage && !isInstallationPage);

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
			<Button variant="secondary" onClick={buttonHandler}>Next →</Button>
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
		padding: var(--spacing-sm) var(--spacing-xl);
		background-color: var(--color-bg-footer);
		border-top: 1px solid var(--color-border-light);
		min-height: var(--footer-height);
	}

	.footer-left,
	.footer-right {
		display: flex;
		align-items: center;
		gap: 8px;
	}
</style>
