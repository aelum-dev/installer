<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		variant = 'primary',
		size = 'medium',
		disabled = false,
		fullWidth = false,
		onclick,
		children
	}: {
		variant?: 'primary' | 'secondary' | 'ghost';
		size?: 'small' | 'medium' | 'large';
		disabled?: boolean;
		fullWidth?: boolean;
		onclick?: (e: MouseEvent) => void;
		children: Snippet;
	} = $props();

	const btnClass = $derived(`btn btn-${variant} btn-${size}${fullWidth ? ' btn-fullWidth' : ''}`);
</script>

<button
	type="button"
	class={btnClass}
	disabled={disabled}
	onclick={onclick}
>
	{@render children()}
</button>

<style>
	.btn {
		font-family: inherit;
		font-weight: var(--font-weight-600);
		border: none;
		border-radius: var(--radius-xl);
		cursor: pointer;
		transition: all 0.2s ease;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: var(--gap-xs);
		letter-spacing: 0.01em;
	}


	.btn-primary {
		background-color: var(--color-primary);
		color: var(--color-bg-white);
	}

	.btn-primary:hover:not(:disabled) {
		background-color: var(--color-primary-hover);
		transform: translateY(-1px);
		box-shadow: var(--shadow-primary);
	}

	.btn-primary:active:not(:disabled) {
		transform: translateY(0) scale(0.98);
		box-shadow: var(--shadow-sm);
	}

	.btn-secondary {
		background-color: var(--color-bg-white);
		color: var(--color-text);
		border: 1.5px solid var(--color-border-light);
	}

	.btn-secondary:hover:not(:disabled) {
		background-color: var(--color-bg-light);
		border-color: var(--color-border);
		transform: translateY(-1px);
		box-shadow: var(--shadow-sm);
	}

	.btn-secondary:active:not(:disabled) {
		transform: translateY(0) scale(0.98);
		background-color: var(--color-bg-hover);
	}

	.btn-ghost {
		background-color: transparent;
		color: var(--color-text-muted);
		padding: var(--spacing-xs) var(--spacing-sm);
		border: 1.5px solid var(--color-border-light);
	}

	.btn-ghost:hover:not(:disabled) {
		color: var(--color-text);
		border-color: var(--color-border);
		background-color: var(--color-bg-light);
	}

	.btn-ghost:active:not(:disabled) {
		color: var(--color-text);
		border-color: var(--color-border);
		background-color: var(--color-bg-hover);
	}

	.btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
		transform: none !important;
		box-shadow: none !important;
	}

	.btn-small {
		font-size: var(--font-size-sm);
		padding: 4px 16px;
	}

	.btn-medium {
		font-size: var(--font-size-md-lg);
		padding: 7px 20px;
	}

	.btn-large {
		font-size: var(--font-size-base);
		padding: 11px 32px;
	}

	.btn-fullWidth {
		width: 100%;
	}
</style>
