<script lang="ts">
	import { installState, commitLuksPassphrase } from '$lib/store.svelte';
	import { t } from '$lib/i18n';
	import { glyph } from '$lib/icons';
	import Dialog from '$components/Dialog.svelte';

	let {
		// Drives visibility; binds back to the shared luks dialog flag.
		show = $bindable(installState.luksDialogOpen),
	}: {
		show?: boolean;
	} = $props();

	let passphrase = $state('');
	let confirm = $state('');
	let showPass = $state(false);
	let showConfirm = $state(false);
	let focusRef = $state<HTMLElement | null>(null);

	// Live inline validation: error when fields diverge or the first is empty.
	const error = $derived.by(() => {
		if (passphrase === '') return t('luks.dialog.errorEmpty');
		if (confirm !== '' && confirm !== passphrase) return t('luks.dialog.errorMatch');
		return '';
	});

	const valid = $derived(passphrase !== '' && confirm === passphrase);

	function handleCancel(): void {
		reset();
		show = false;
	}

	function handleConfirm(): void {
		if (!valid) return;
		commitLuksPassphrase(passphrase, confirm);
		reset();
	}

	function reset(): void {
		passphrase = '';
		confirm = '';
		showPass = false;
		showConfirm = false;
	}

	// The dialog shell skips its own focus so the first field can take it.
	$effect(() => {
		if (show) requestAnimationFrame(() => focusRef?.focus());
	});
</script>

<Dialog bind:show title={t('luks.dialog.title')} onCancel={handleCancel} focusOnOpen={false}>
	<div class="field">
		<label for="luks-passphrase">{t('luks.dialog.passphrase')}</label>
		<div class="password-field">
			<input
				id="luks-passphrase"
				bind:this={focusRef}
				bind:value={passphrase}
				type={showPass ? 'text' : 'password'}
				placeholder={t('luks.dialog.passphrasePlaceholder')}
				autocomplete="off"
				spellcheck="false"
				onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleConfirm(); } }}
			/>
			<button
				type="button"
				class="toggle-visibility"
				aria-label={showPass ? t('luks.dialog.hide') : t('luks.dialog.show')}
				onclick={() => (showPass = !showPass)}
			>
				<span class="nf-icon" aria-hidden="true">{showPass ? glyph('f070') : glyph('f06e')}</span>
			</button>
		</div>
	</div>
	<div class="field">
		<label for="luks-confirm">{t('luks.dialog.confirmPassphrase')}</label>
		<div class="password-field">
			<input
				id="luks-confirm"
				bind:value={confirm}
				type={showConfirm ? 'text' : 'password'}
				placeholder={t('luks.dialog.confirmPlaceholder')}
				autocomplete="off"
				spellcheck="false"
				onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleConfirm(); } }}
			/>
			<button
				type="button"
				class="toggle-visibility"
				aria-label={showConfirm ? t('luks.dialog.hide') : t('luks.dialog.show')}
				onclick={() => (showConfirm = !showConfirm)}
			>
				<span class="nf-icon" aria-hidden="true">{showConfirm ? glyph('f070') : glyph('f06e')}</span>
			</button>
		</div>
		{#if error}
			<p class="field-error">{error}</p>
		{/if}
	</div>

	{#snippet actions()}
		<button type="button" class="btn-cancel" onclick={handleCancel}>
			{t('dialog.confirm.cancel')}
		</button>
		<button type="button" class="btn-confirm" onclick={handleConfirm} disabled={!valid}>
			{t('dialog.confirm.confirm')}
		</button>
	{/snippet}
</Dialog>

<style>
	/* Shared dialog chrome lives in Dialog.svelte / dialog.css. */
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--spacing-xs);
		margin-bottom: var(--spacing-md);
	}

	.field label {
		font-size: var(--font-size-sm);
		font-weight: 600;
		color: var(--color-text);
	}

	.password-field {
		display: flex;
		align-items: center;
		border: 1.5px solid var(--color-border);
		border-radius: var(--radius-xl);
		background: var(--color-bg-light);
		overflow: hidden;
	}

	.password-field input {
		flex: 1;
		border: none;
		background: transparent;
		padding: var(--pad-btn);
		font-family: inherit;
		font-size: var(--font-size-md);
		color: var(--color-text);
		outline: none;
		min-width: 0;
	}

	.password-field input::placeholder {
		color: var(--color-text-muted);
	}

	.toggle-visibility {
		background: transparent;
		border: none;
		padding: var(--pad-btn) var(--spacing-md);
		font-family: inherit;
		font-size: var(--font-size-sm);
		font-weight: 600;
		color: var(--color-primary);
		cursor: pointer;
	}

	/* Keep the show/hide eye small and aligned with the button text; the global
	.nf-icon is --font-size-xl (2rem), far too large for this inline control. */
	.toggle-visibility .nf-icon {
		font-size: 1rem;
		vertical-align: -0.1em;
	}

	.field-error {
		margin: 0;
		font-size: var(--font-size-sm);
		color: var(--color-danger);
	}

	.btn-confirm:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
