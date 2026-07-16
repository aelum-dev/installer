<script lang="ts">
	import type { Snippet } from 'svelte';
	import ErrorBanner from '$components/ErrorBanner.svelte';

	let { title, children, errorMessage }: { title: string; children: Snippet; errorMessage?: string | null } = $props();
</script>

<div class="page-layout">
	<h2 class="page-title">{title}</h2>
	{#if errorMessage}
		<div class="error-banner-fixed">
			<ErrorBanner message={errorMessage} />
		</div>
	{/if}
	<div class="page-content">
		{@render children()}
	</div>
</div>

<style>
	.page-layout {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		gap: var(--spacing-lg, 20px);
		position: relative;
	}

	.page-title {
		margin: 0;
		font-size: var(--font-size-lg, 1.5rem);
		color: var(--color-text, #333);
		font-weight: var(--font-weight-700, 700);
	}

	@keyframes shake {
		0%, 100% { transform: translateX(-50%); }
		10%, 30%, 50%, 70%, 90% { transform: translateX(calc(-50% - 4px)); }
		20%, 40%, 60%, 80% { transform: translateX(calc(-50% + 4px)); }
	}

	.error-banner-fixed {
		animation: shake 0.5s ease-in-out;
		position: absolute;
		top: 20px;
		left: 50%;
		transform: translateX(-50%);
		z-index: 10;
		width: 100%;
		max-width: 520px;
		padding: 0 16px;
		box-sizing: border-box;
	}

	.page-content {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--spacing-xl, 24px);
		width: 100%;
		max-width: 520px;
	}
</style>
