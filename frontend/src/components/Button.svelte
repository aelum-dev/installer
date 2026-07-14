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

	function getBtnClass(): string {
		let cls = 'btn';
		if (variant === 'primary') cls += ' btn-primary';
		else if (variant === 'secondary') cls += ' btn-secondary';
		else if (variant === 'ghost') cls += ' btn-ghost';

		if (size === 'small') cls += ' btn-small';
		else if (size === 'medium') cls += ' btn-medium';
		else if (size === 'large') cls += ' btn-large';

		if (fullWidth) cls += ' btn-fullWidth';
		return cls;
	}
</script>

<button
	type="button"
	class={getBtnClass()}
	disabled={disabled}
	onclick={(e) => onClick?.(e)}
>
	{@render children()}
</button>

<style>
	.btn {
		font-family: inherit;
		font-weight: 600;
		border: none;
		border-radius: 8px;
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
		box-shadow: 0 0 0 3px rgba(74, 144, 217, 0.3);
	}

	/* svelte-ignore css_unused_selector */
	.btn-primary {
		background-color: #4a90d9;
		color: #fff;
	}

	/* svelte-ignore css_unused_selector */
	.btn-primary:hover:not(:disabled) {
		background-color: #3a7bc8;
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(74, 144, 217, 0.35);
	}

	/* svelte-ignore css_unused_selector */
	.btn-primary:active:not(:disabled) {
		transform: translateY(0) scale(0.98);
		box-shadow: 0 2px 6px rgba(74, 144, 217, 0.25);
	}

	/* svelte-ignore css_unused_selector */
	.btn-secondary {
		background-color: #ffffff;
		color: #3d3d3d;
		border: 1.5px solid #d0d0d0;
	}

	/* svelte-ignore css_unused_selector */
	.btn-secondary:hover:not(:disabled) {
		background-color: #f7f8fa;
		border-color: #b8b8b8;
		transform: translateY(-1px);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
	}

	/* svelte-ignore css_unused_selector */
	.btn-secondary:active:not(:disabled) {
		transform: translateY(0) scale(0.98);
		background-color: #f0f1f3;
	}

	/* svelte-ignore css_unused_selector */
	.btn-ghost {
		background-color: transparent;
		color: #555;
		padding: 6px 12px;
		border: 1.5px solid #d8d8d8;
	}

	/* svelte-ignore css_unused_selector */
	.btn-ghost:hover:not(:disabled) {
		color: #333;
		border-color: #c0c0c0;
		background-color: rgba(0, 0, 0, 0.02);
	}

	/* svelte-ignore css_unused_selector */
	.btn-ghost:active:not(:disabled) {
		color: #333;
		border-color: #aaa;
		background-color: rgba(0, 0, 0, 0.04);
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
