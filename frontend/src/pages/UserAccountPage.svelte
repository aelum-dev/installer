<script lang="ts">
	import { installState } from '$lib/store.svelte';
	import { validateUserAccount } from '$lib/validate';
	import { getValidationMessages } from '$lib/validate-messages';
	import { t } from '$lib/i18n';
	import PageLayout from '$components/PageLayout.svelte';
	import '$lib/layout.css';

	let usernameTouched = $state(false);
	let passwordTouched = $state(false);
	let confirmPasswordTouched = $state(false);

	const errors = $derived(
		validateUserAccount(installState.username, installState.password, installState.confirmPassword, {
			usernameTouched,
			passwordTouched,
			confirmPasswordTouched,
		}, getValidationMessages())
	);

	// Username validation rules
	const usernameMinLength = $derived(installState.username.length >= 3);
	const usernameFirstChar = $derived(/^[a-z_]/.test(installState.username));
	const usernameValidChars = $derived(/^[a-z0-9_-]+$/.test(installState.username));
	const passwordMinLength = $derived(installState.password.length >= 6);

	function handleBlur(field: 'username' | 'password' | 'confirmPassword'): void {
		switch (field) {
			case 'username': usernameTouched = true; break;
			case 'password': passwordTouched = true; break;
			case 'confirmPassword': confirmPasswordTouched = true; break;
		}
	}
</script>

<PageLayout
	title={t('page.userAccount.title')}
	subtitle={t('page.userAccount.subtitle')}
>
	<div slot="left">
		<div class="requirements">
			<div class="requirement-item">
				<span class="check-icon" class:visible={usernameMinLength}>
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
						<polyline points="20 6 9 17 4 12" />
					</svg>
				</span>
				<span>{t('validation.usernameMinLength')}</span>
			</div>
			<div class="requirement-item">
				<span class="check-icon" class:visible={usernameFirstChar}>
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
						<polyline points="20 6 9 17 4 12" />
					</svg>
				</span>
				<span>{t('validation.usernameInvalidFirstChar')}</span>
			</div>
			<div class="requirement-item">
				<span class="check-icon" class:visible={usernameValidChars}>
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
						<polyline points="20 6 9 17 4 12" />
					</svg>
				</span>
				<span>{t('validation.usernameInvalidChars')}</span>
			</div>
			<div class="requirement-item">
				<span class="check-icon" class:visible={passwordMinLength}>
					<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
						<polyline points="20 6 9 17 4 12" />
					</svg>
				</span>
				<span>{t('validation.passwordMinLength')}</span>
			</div>
		</div>
	</div>

	<div slot="right">
		<form class="form" onsubmit={(e) => e.preventDefault()}>
			<div class="field">
				<label for="username">{t('page.userAccount.username')}</label>
				<input
					id="username"
					type="text"
					class={['line-edit', { 'line-edit--error': !!errors.username }]}
					placeholder={t('page.userAccount.usernamePlaceholder')}
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
	</div>
</PageLayout>

<style>
	.form {
		width: 100%;
	}

	.requirements {
		display: flex;
		flex-direction: column;
		gap: var(--spacing-sm);
		align-items: flex-start;
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
	}

	.check-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		flex-shrink: 0;
		opacity: 0;
		transition: opacity 0.2s ease;
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
