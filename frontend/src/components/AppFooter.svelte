<script lang="ts">
	let { currentPage, onBack, onNext }: { currentPage: number; onBack: () => void; onNext: () => void } = $props();

	const isLastPage = $derived(currentPage === 4);
	const showBack = $derived(currentPage > 0);
</script>

<footer class="footer">
	<div class="footer-left">
		{#if showBack}
			<button class="btn-back" onclick={onBack}>← Back</button>
		{/if}
	</div>
	<div class="footer-right">
		<button class="btn-next" onclick={onNext}>{isLastPage ? 'Finish' : 'Next →'}</button>
	</div>
</footer>

<style>
	.footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 24px;
		background-color: #e8e8e8;
		border-top: 1px solid #ccc;
		min-height: 44px;
	}

	.footer-left,
	.footer-right {
		display: flex;
		align-items: center;
	}

	.btn-back,
	.btn-next {
		padding: 6px 16px;
		border: 1px solid #999;
		border-radius: 4px;
		background-color: #fff;
		color: #333;
		cursor: pointer;
		font-size: 0.9rem;
	}

	.btn-back:hover,
	.btn-next:hover {
		background-color: #f0f0f0;
	}
</style>
