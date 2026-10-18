<script lang="ts">
	import { t } from '$lib/i18n';
	import Dialog from '$components/Dialog.svelte';

	let {
		// Drives visibility; bind it to the caller's own boolean.
		show = $bindable(false),
		title = t('dialog.confirm.title'),
		message,
		// Plain-text bullet list (used by validation failures). Rendered as a
		// list, not `<br>`-joined HTML, so callers pass strings — never markup.
		reasons,
		cancelLabel = t('dialog.confirm.cancel'),
		confirmLabel = t('dialog.confirm.confirm'),
		// Delete dialog historically had no warning glyph; opt out with show-icon={false}.
		showIcon = true,
		// Validation modal uses OK-only (no cancel).
		showCancel = true,
		onConfirm,
		onCancel,
	}: {
		show?: boolean;
		title?: string;
		/** Rich single message; may contain trusted `<strong>` markup from translations. */
		message?: string;
		/** Plain-text reasons rendered as a bullet list. */
		reasons?: string[];
		cancelLabel?: string;
		confirmLabel?: string;
		showIcon?: boolean;
		showCancel?: boolean;
		onConfirm?: () => void;
		onCancel?: () => void;
	} = $props();

	function handleConfirm(): void {
		show = false;
		onConfirm?.();
	}

	// Dismiss the dialog: hide it and run the caller's cancel side effect (if any).
	// Used by both the Cancel button and the backdrop/Escape handler.
	function handleCancel(): void {
		show = false;
		onCancel?.();
	}
</script>

<Dialog bind:show {title} onCancel={handleCancel}>
	{#if showIcon}
		<div class="dialog-icon">
			<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
				<path d="M12 2L2 20H22L12 2Z" fill="var(--color-danger)" stroke="var(--color-danger)" stroke-width="1.5" stroke-linejoin="round"/>
				<line x1="12" y1="8" x2="12" y2="13" stroke="white" stroke-width="2" stroke-linecap="round"/>
				<circle cx="12" cy="16.5" r="1" fill="white"/>
			</svg>
		</div>
	{/if}
	{#if reasons && reasons.length > 0}
		<ul class="dialog-reasons">
			{#each reasons as reason, i (i)}
				<li>{reason}</li>
			{/each}
		</ul>
	{:else if message}
		<p class="dialog-message">
			{@html message}
		</p>
	{/if}

	{#snippet actions()}
		{#if showCancel}
			<button type="button" class="btn-cancel" onclick={handleCancel}>
				{cancelLabel}
			</button>
		{/if}
		<button type="button" class="btn-confirm" onclick={handleConfirm}>
			{confirmLabel}
		</button>
	{/snippet}
</Dialog>

<style>
	/* Shared dialog chrome lives in Dialog.svelte / dialog.css. */
	.dialog-icon {
		text-align: center;
		margin-bottom: var(--spacing-md);
	}

	.dialog-icon svg {
		width: 64px;
		height: 64px;
	}

	.dialog-message {
		margin: 0 0 24px;
		font-size: var(--font-size-md);
		color: var(--color-text-muted);
		line-height: 1.6;
		text-align: center;
	}

	.dialog-reasons {
		width: fit-content;
		margin: 0 auto 24px;
		padding-inline-start: 1.25em;
		text-align: start;
		font-size: var(--font-size-md);
		color: var(--color-text-muted);
		line-height: 1.6;
	}

	.dialog-reasons li + li {
		margin-top: 4px;
	}
</style>
