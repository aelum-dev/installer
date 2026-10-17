<script lang="ts">
	import type { Snippet } from 'svelte';
	import './dialog.css';

	/**
	 * Shared modal chrome: overlay, focus handling, Escape-to-close, title and
	 * action row. Dialogs supply their own body and buttons, so the
	 * accessibility behaviour lives in one place instead of being reimplemented
	 * (inconsistently) per dialog.
	 */
	let {
		show = $bindable(false),
		title,
		onCancel,
		focusOnOpen = true,
		children,
		actions,
	}: {
		show?: boolean;
		title: string;
		onCancel: () => void;
		/** Set false when the dialog body focuses its own first field. */
		focusOnOpen?: boolean;
		children: Snippet;
		actions: Snippet;
	} = $props();

	const titleId = $props.id();
	let dialogEl = $state<HTMLElement | null>(null);
	let restoreFocus: HTMLElement | null = null;

	// Move focus into the dialog on open and back to the trigger on close, so
	// keyboard users are never left on a detached element.
	$effect(() => {
		if (show) {
			restoreFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
			if (focusOnOpen) requestAnimationFrame(() => dialogEl?.focus());
		} else if (restoreFocus) {
			restoreFocus.focus();
			restoreFocus = null;
		}
	});
</script>

{#if show}
	<div
		class="dialog-overlay"
		role="presentation"
		onclick={onCancel}
		onkeydown={(e) => {
			if (e.key === 'Escape') onCancel();
		}}
	>
		<div
			class="dialog"
			role="dialog"
			aria-modal="true"
			aria-labelledby={titleId}
			tabindex="-1"
			bind:this={dialogEl}
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => {
				if (e.key === 'Escape') onCancel();
				e.stopPropagation();
			}}
		>
			<h2 id={titleId} class="dialog-title">{title}</h2>
			{@render children()}
			<div class="dialog-actions">
				{@render actions()}
			</div>
		</div>
	</div>
{/if}
