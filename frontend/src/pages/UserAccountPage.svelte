<script lang="ts">
	import { installState } from '$lib/store.svelte';
	import { validateUserAccount } from '$lib/validate';
	import { getValidationMessages } from '$lib/validate-messages';
	import { t } from '$lib/i18n';
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

	function handleBlur(field: 'username' | 'password' | 'confirmPassword'): void {
		switch (field) {
			case 'username': usernameTouched = true; break;
			case 'password': passwordTouched = true; break;
			case 'confirmPassword': confirmPasswordTouched = true; break;
		}
	}
</script>

<div class="page-content stretch">
		<div class="col col-left">
			<h2 class="page-title">{t('page.userAccount.title')}</h2>
		</div>
		<div class="col col-right">
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
	</div>

<style>
	.page-content {
		display: grid;
		grid-template-columns: 1fr 1fr;
		align-items: center;
		flex: 1;
		width: 100%;
		box-sizing: border-box;
		min-width: 0;
		margin: 0;
	}

	.col-left {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: var(--spacing-lg);
	}

	.col-right {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		justify-content: center;
		overflow-y: auto;
		overflow-x: hidden;
	}

	.col-left,
	.col-right {
		min-width: 0;
	}

	.form {
		width: 100%;
	}
</style>
