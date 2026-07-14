<script lang="ts">
	import Button from '$components/Button.svelte';

	let { currentPage, onBack, onNext }: { currentPage: number; onBack: () => void; onNext: () => void } = $props();

	const isLastPage = $derived(currentPage === 4);
	const showBack = $derived(currentPage > 0);
</script>

<footer class="footer">
	<div class="footer-left">
		{#if showBack}
			<Button variant="ghost" onClick={onBack}>← Back</Button>
		{/if}
	</div>
	<div class="footer-right">
		<Button variant="secondary" onClick={onNext}>{isLastPage ? 'Finish' : 'Next →'}</Button>
	</div>
</footer>

<style>
	.footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 24px;
		background-color: #f3f3f3;
		border-top: 1px solid #e5e5e5;
		min-height: 52px;
	}

	.footer-left,
	.footer-right {
		display: flex;
		align-items: center;
		gap: 8px;
	}
</style>
