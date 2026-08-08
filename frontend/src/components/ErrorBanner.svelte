<script lang="ts">
	let { message }: { message: string | null } = $props();
</script>

<div class="error-banner" role="alert" class:visible={!!message}>
	<span class="error-icon">⚠</span>
	<span class="error-text">{message}</span>
	<button class="error-close" onclick={() => message = null} aria-label="Dismiss">
		✕
	</button>
</div>

<style>
	.error-banner {
		width: 100%;
		max-width: 480px;
		background-color: var(--color-danger-error-bg);
		border: 1px solid var(--color-danger);
		border-radius: var(--radius-md);
		padding: 6px 12px;
		display: flex;
		align-items: center;
		gap: 8px;
		animation: slideIn 0.2s ease;
		position: relative;
		z-index: 10;
		transition: opacity 0.2s ease;
	}

	.error-banner.visible {
		opacity: 1;
		visibility: visible;
		pointer-events: auto;
	}

	.error-banner:not(.visible) {
		opacity: 0;
		visibility: hidden;
		pointer-events: none;
	}

	@keyframes slideIn {
		from {
			opacity: 0;
			transform: translateY(-6px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.error-icon {
		font-size: 0.9rem;
		flex-shrink: 0;
	}

	.error-text {
		flex: 1;
		font-size: 0.78rem;
		color: var(--color-danger);
		margin: 0;
		line-height: 1.3;
	}

	.error-close {
		background: none;
		border: none;
		color: var(--color-danger);
		cursor: pointer;
		font-size: 0.85rem;
		padding: 2px 6px;
		border-radius: var(--radius-sm);
		transition: background-color 0.15s ease;
		flex-shrink: 0;
	}

	.error-close:hover {
		background-color: var(--color-danger-error-hover);
	}
</style>
