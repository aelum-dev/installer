<script lang="ts">
	import { installState } from '$lib/store.svelte';
	import { validateUserAccount } from '$lib/validate';
	import { getValidationMessages } from '$lib/validate-messages';
	import { t } from '$lib/i18n';
	import { glyph, STEP_ICONS } from '$lib/icons';
	import { MinPasswordLength, MinUsernameLength } from '$lib/constants';
	import PageLayout from '$components/PageLayout.svelte';

	const pageIcon = glyph(STEP_ICONS.userAccount);

	let usernameTouched = $state(false);
	let passwordTouched = $state(false);
	let confirmPasswordTouched = $state(false);
	let usernameInput = $state<HTMLInputElement | null>(null);

	$effect(() => {
		usernameInput?.select();
	});

	const errors = $derived(
		validateUserAccount(installState.username, installState.password, installState.confirmPassword, {
			usernameTouched,
			passwordTouched,
			confirmPasswordTouched,
		}, getValidationMessages())
	);

	// Validation requirements
	const usernameMinLength = $derived(installState.username.length >= MinUsernameLength);
	const passwordMinLength = $derived(installState.password.length >= MinPasswordLength);
	const passwordsMatch = $derived(
		installState.confirmPassword.length > 0 && installState.confirmPassword === installState.password
	);

	function handleBlur(field: 'username' | 'password' | 'confirmPassword'): void {
		switch (field) {
			case 'username': usernameTouched = true; break;
			case 'password': passwordTouched = true; break;
			case 'confirmPassword': confirmPasswordTouched = true; break;
		}
	}
</script>

{#snippet requirement(met: boolean, label: string)}
	<div class="requirement-item">
		<span class="check-icon" class:visible={met}>
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
				<polyline points="20 6 9 17 4 12" />
			</svg>
		</span>
		<span>{label}</span>
	</div>
{/snippet}

<PageLayout icon={pageIcon} title={t('page.userAccount.title')} subtitle={t('page.userAccount.subtitle')}>
	{#snippet info()}
		<div class="requirements">
			{@render requirement(usernameMinLength, t('validation.usernameMinLength'))}
			{@render requirement(passwordMinLength, t('validation.passwordMinLength'))}
			{@render requirement(passwordsMatch, t('validation.passwordsMatch'))}
		</div>
	{/snippet}

	<form class="form" onsubmit={(e) => e.preventDefault()}>
		<div class="field">
			<label for="username">{t('page.userAccount.username')}</label>
			<input
				id="username"
				type="text"
				class={['line-edit', { 'line-edit--error': !!errors.username }]}
				placeholder={t('page.userAccount.usernamePlaceholder')}
				bind:this={usernameInput}
				bind:value={installState.username}
				onblur={() => handleBlur('username')}
			/>
			<span class="error" class:visible={!!errors.username}>{errors.username}</span>
		</div>

		<div class="field">
			<label for="password">{t('page.userAccount.password')}</label>
			<input
				id="password"
				type="password"
				class={['line-edit', { 'line-edit--error': !!errors.password }]}
				placeholder={t('page.userAccount.passwordPlaceholder')}
				bind:value={installState.password}
				onblur={() => handleBlur('password')}
			/>
			<span class="error" class:visible={!!errors.password}>{errors.password}</span>
		</div>

		<div class="field">
			<label for="confirmPassword">{t('page.userAccount.confirmPassword')}</label>
			<input
				id="confirmPassword"
				type="password"
				class={['line-edit', { 'line-edit--error': !!errors.confirmPassword }]}
				placeholder={t('page.userAccount.confirmPasswordPlaceholder')}
				bind:value={installState.confirmPassword}
				onblur={() => handleBlur('confirmPassword')}
			/>
			<span class="error" class:visible={!!errors.confirmPassword}>{errors.confirmPassword}</span>
		</div>
	</form>
</PageLayout>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: var(--spacing-xs);
		width: 100%;
	}

	.field {
		display: flex;
		flex-direction: column;
		text-align: start;
	}

	.field > :first-child {
		margin-bottom: var(--spacing-xs);
	}

	.field > :nth-child(2) {
		margin-bottom: var(--spacing-xs);
	}

	.field label {
		font-size: var(--font-size-md);
		font-weight: var(--font-weight-600);
		color: var(--color-text-muted);
		letter-spacing: 0.02em;
	}

	.line-edit {
		background-color: var(--color-bg-white);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-xl);
		padding: 11px 20px;
		font-family: inherit;
		font-size: var(--font-size-base);
		color: var(--color-text);
		transition: background-color 0.15s ease, border-color 0.15s ease;
	}

	.line-edit::placeholder {
		color: var(--color-text-light);
	}


	.line-edit--error {
		border-color: var(--color-danger);
	}

	.line-edit--error:focus {
		border-color: var(--color-danger);
	}

	.error {
		font-size: var(--font-size-sm);
		color: var(--color-danger);
		margin: 0;
		min-height: 1.4em;
		overflow: hidden;
	}

	.error.visible {
		overflow: visible;
	}

	.requirements {
		display: flex;
		flex-direction: column;
		gap: var(--spacing-sm);
		width: fit-content;
		margin-inline: auto;
	}

	.requirement-item {
		display: flex;
		align-items: center;
		gap: var(--spacing-sm);
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
		line-height: 1.4;
		margin: 0;
		text-align: left;
		white-space: nowrap;
	}

	.check-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		flex-shrink: 0;
		opacity: 0.3;
		color: var(--color-text-muted);
		transition: opacity 0.2s ease, color 0.2s ease;
	}

	.check-icon.visible {
		opacity: 1;
		color: var(--color-primary);
	}

	.check-icon svg {
		width: 100%;
		height: 100%;
	}
</style>
