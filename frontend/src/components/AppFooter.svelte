<script lang="ts">
	import Button from '$components/Button.svelte';

	let { currentPage, onBack, onNext, onFinish }: { currentPage: number; onBack: () => void; onNext: () => void; onFinish?: () => void } = $props();

const buttonHandler = $derived(onFinish ?? onNext);

	const isLastPage = $derived(currentPage === 4);
	const showBack = $derived(currentPage > 0 && !isLastPage);
</script>

<footer class="footer">
	<div class="footer-left">
		{#if showBack}
			<Button variant="ghost" onClick={onBack}>← Back</Button>
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
