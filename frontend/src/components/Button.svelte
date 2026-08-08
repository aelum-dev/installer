<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		variant = 'primary',
		size = 'medium',
		disabled = false,
		fullWidth = false,
		onClick,
		children
	}: {
		variant?: 'primary' | 'secondary' | 'ghost';
		size?: 'small' | 'medium' | 'large';
		disabled?: boolean;
		fullWidth?: boolean;
		onClick?: (e: MouseEvent) => void;
		children: Snippet;
	} = $props();

	const btnClass = $derived(`btn btn-${variant} btn-${size}${fullWidth ? ' btn-fullWidth' : ''}`);
</script>

<button
	type="button"
	class={btnClass}
	disabled={disabled}
	onclick={(e) => onClick?.(e)}
>
	{@render children()}
</button>

<style>
	.btn {
		font-family: inherit;
		font-weight: var(--font-weight-600);
		border: none;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all 0.2s ease;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		outline: none;
		letter-spacing: 0.01em;
	}

	.btn:focus-visible {
		box-shadow: 0 0 0 3px var(--color-focus-strong);
	}

	/* svelte-ignore css_unused_selector */
	.btn-primary {
		background-color: var(--color-primary);
		color: var(--color-bg-white);
	}

	/* svelte-ignore css_unused_selector */
	.btn-primary:hover:not(:disabled) {
		background-color: var(--color-primary-hover);
		transform: translateY(-1px);
		box-shadow: var(--shadow-primary);
	}

	/* svelte-ignore css_unused_selector */
	.btn-primary:active:not(:disabled) {
		transform: translateY(0) scale(0.98);
		box-shadow: var(--shadow-sm);
	}

	/* svelte-ignore css_unused_selector */
	.btn-secondary {
		background-color: var(--color-bg-white);
		color: var(--color-text);
		border: 1.5px solid var(--color-border-light);
	}

	/* svelte-ignore css_unused_selector */
	.btn-secondary:hover:not(:disabled) {
		background-color: var(--color-bg-light);
		border-color: var(--color-border);
		transform: translateY(-1px);
		box-shadow: var(--shadow-sm);
	}

	/* svelte-ignore css_unused_selector */
	.btn-secondary:active:not(:disabled) {
		transform: translateY(0) scale(0.98);
		background-color: var(--color-bg-hover);
	}

	/* svelte-ignore css_unused_selector */
	.btn-ghost {
		background-color: transparent;
		color: var(--color-text-muted);
		padding: var(--spacing-xs) var(--spacing-sm);
		border: 1.5px solid var(--color-border-light);
	}

	/* svelte-ignore css_unused_selector */
	.btn-ghost:hover:not(:disabled) {
		color: var(--color-text);
		border-color: var(--color-border);
		background-color: var(--color-bg-light);
	}

	/* svelte-ignore css_unused_selector */
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

	/* svelte-ignore css_unused_selector */
	.btn-small {
		font-size: 0.8rem;
		padding: 4px 12px;
		border-radius: 6px;
	}

	/* svelte-ignore css_unused_selector */
	.btn-medium {
		font-size: 0.9rem;
		padding: 7px 20px;
	}

	/* svelte-ignore css_unused_selector */
	.btn-large {
		font-size: 1rem;
		padding: 11px 32px;
		border-radius: 10px;
	}

	/* svelte-ignore css_unused_selector */
	.btn-fullWidth {
		width: 100%;
	}
</style>
