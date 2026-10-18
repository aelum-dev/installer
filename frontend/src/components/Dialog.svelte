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

	// Native focusable elements used to wrap Tab at the dialog's edges.
	const FOCUSABLE =
		'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

	function focusable(): HTMLElement[] {
		if (!dialogEl) return [];
		// tabIndex >= 0 keeps only elements reachable by Tab (excludes anything
		// with tabindex="-1"). The dialogs in this app never hide their controls,
		// so a layout-based visibility check isn't needed here.
		return Array.from(dialogEl.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
			(el) => el.tabIndex >= 0,
		);
	}

	// Trap Tab/Shift+Tab inside the modal so focus can't reach the obscured page
	// behind the overlay. Escape cancels.
	function handleKeydown(e: KeyboardEvent): void {
		if (e.key === 'Escape') {
			e.stopPropagation();
			onCancel();
			return;
		}
		if (e.key !== 'Tab') return;

		e.stopPropagation();
		const items = focusable();
		if (items.length === 0) {
			e.preventDefault();
			dialogEl?.focus();
			return;
		}

		const first = items[0];
		const last = items[items.length - 1];
		const active = document.activeElement;
		const inside = dialogEl?.contains(active) ?? false;

		if (e.shiftKey) {
			if (!inside || active === first || active === dialogEl) {
				e.preventDefault();
				last.focus();
			}
		} else if (!inside || active === last || active === dialogEl) {
			e.preventDefault();
			first.focus();
		}
	}

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
			onkeydown={handleKeydown}
		>
			<h2 id={titleId} class="dialog-title">{title}</h2>
			{@render children()}
			<div class="dialog-actions">
				{@render actions()}
			</div>
		</div>
	</div>
{/if}
