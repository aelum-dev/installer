<script lang="ts">
	import { LogInfo } from '@wails/runtime/runtime';
	import { userAccountData } from '$lib/store';
	import { validateUserAccount, hasErrors } from '$lib/validate';

	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let usernameTouched = $state(false);
	let passwordTouched = $state(false);
	let confirmPasswordTouched = $state(false);

	const validationState = $derived.by(() => ({
		usernameTouched,
		passwordTouched,
		confirmPasswordTouched,
	}));

	const errors = $derived(validateUserAccount(username, password, confirmPassword, validationState));

	$effect(() => {
		userAccountData.set({ username, password, confirmPassword });
	});

	function handleBlur(field: 'username' | 'password' | 'confirmPassword'): void {
		switch (field) {
			case 'username': usernameTouched = true; break;
			case 'password': passwordTouched = true; break;
			case 'confirmPassword': confirmPasswordTouched = true; break;
		}
	}

	$effect(() => {
		const handler = () => {
			LogInfo(`[UserAccountPage] username: ${username}, password: ${password}`);
		};
		document.addEventListener('page-leave', handler);
		return () => document.removeEventListener('page-leave', handler);
	});
</script>

<div class="page">
	<h2>Create User Account</h2>

	<form class="form" onsubmit={(e) => e.preventDefault()}>
		<div class="field">
			<label for="username">Username</label>
			<input
				id="username"
				type="text"
				class={['line-edit', { 'line-edit--error': !!errors.username }]}
				placeholder="Enter username"
				bind:value={username}
				onblur={() => handleBlur('username')}
			/>
			{#if errors.username}
				<span class="error">{errors.username}</span>
			{/if}
		</div>

		<div class="field">
			<label for="password">Password</label>
			<input
				id="password"
				type="password"
				class={['line-edit', { 'line-edit--error': !!errors.password }]}
				placeholder="Enter password"
				bind:value={password}
				onblur={() => handleBlur('password')}
			/>
			{#if errors.password}
				<span class="error">{errors.password}</span>
			{/if}
		</div>

		<div class="field">
			<label for="confirmPassword">Confirm Password</label>
			<input
				id="confirmPassword"
				type="password"
				class={['line-edit', { 'line-edit--error': !!errors.confirmPassword }]}
				placeholder="Confirm password"
				bind:value={confirmPassword}
				onblur={() => handleBlur('confirmPassword')}
			/>
			{#if errors.confirmPassword}
				<span class="error">{errors.confirmPassword}</span>
			{/if}
		</div>
	</form>
</div>

<style>
	.page {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		gap: 32px;
	}

	h2 {
		position: absolute;
		top: 24px;
		left: 50%;
		transform: translateX(-50%);
		font-size: 1.5rem;
		color: #333;
		margin: 0;
	}

	.form {
		display: flex;
		flex-direction: column;
		gap: 20px;
		width: 100%;
		max-width: 320px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		text-align: left;
	}

	label {
		font-size: 0.85rem;
		font-weight: 600;
		color: #666;
		letter-spacing: 0.02em;
	}

	.line-edit {
		background-color: #fff;
		border: 1px solid #999;
		border-radius: 10px;
		padding: 11px 16px;
		font-family: inherit;
		font-size: 1rem;
		color: #333;
		outline: none;
		transition: background-color 0.15s ease, border-color 0.15s ease;
	}

	.line-edit::placeholder {
		color: #999;
	}

	.line-edit:focus {
		border-color: #4a90d9;
		outline: 2px solid #4a90d9;
		outline-offset: 1px;
	}

	.line-edit:hover:not(:focus) {
		background-color: #f5f5f5;
	}

	.line-edit--error {
		border-color: #d32f2f;
	}

	.line-edit--error:focus {
		border-color: #d32f2f;
		outline-color: #d32f2f;
	}

	.error {
		font-size: 0.8rem;
		color: #d32f2f;
		margin: 0;
	}
</style>
