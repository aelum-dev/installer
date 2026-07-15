<script lang="ts">
	import Button from '$components/Button.svelte';

	let { currentPage }: { currentPage: number } = $props();

	const isLastPage = $derived(currentPage === 5);
	const showBack = $derived(currentPage > 0 && !isLastPage);

	const buttonHandler = $derived(isLastPage ? () => {
		document.dispatchEvent(new CustomEvent('finish'));
	} : () => {
		document.dispatchEvent(new CustomEvent('go-next'));
	});
</script>

<footer class="footer">
	<div class="footer-left">
		{#if showBack}
			<Button variant="ghost" onClick={() => document.dispatchEvent(new CustomEvent('go-back'))}>← Back</Button>
		{/if}
	</div>
	<div class="footer-right">
		<Button variant="secondary" onClick={buttonHandler}>{isLastPage ? 'Reboot' : 'Next →'}</Button>
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
