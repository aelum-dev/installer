<script lang="ts">
	import { PAGE_NAMES } from '$lib/types';

	let { currentPage, canGoBack, onNext }: { currentPage: number; canGoBack: boolean; onNext: () => void } = $props();

	$effect(() => {
		document.title = `${PAGE_NAMES[currentPage]} — Arch Installer`;
	});
</script>

<header class="header">
	<div class="header-left">
		{#if canGoBack}
			<button class="btn-back" onclick={() => void 0}>← Back</button>
		{/if}
	</div>
	<h1 class="header-title">{PAGE_NAMES[currentPage]}</h1>
	<div class="header-right">
		{#if currentPage < 4}
			<button class="btn-next" onclick={onNext}>Next →</button>
		{/if}
	</div>
</header>

<style>
	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 24px;
		background-color: #e8e8e8;
		border-bottom: 1px solid #ccc;
		min-height: 56px;
	}

	.header-left,
	.header-right {
		width: 120px;
		display: flex;
		align-items: center;
	}

	.header-left {
		justify-content: flex-start;
	}

	.header-right {
		justify-content: flex-end;
	}

	.header-title {
		font-size: 1.2rem;
		font-weight: 600;
		color: #333;
		text-align: center;
		flex: 1;
		margin: 0;
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

	.btn-back:disabled,
	.btn-next:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
</style>
