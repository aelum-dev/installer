<script lang="ts">
	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
</script>

<div class="page">
	<h2>Create User Account</h2>

	<div class="form">
		<div class="field">
			<label for="username">Username</label>
			<input
				id="username"
				type="text"
				bind:value={username}
				placeholder="Enter username"
			/>
		</div>

		<div class="field">
			<label for="password">Password</label>
			<input
				id="password"
				type="password"
				bind:value={password}
				placeholder="Enter password"
			/>
		</div>

		<div class="field">
			<label for="confirm-password">Confirm Password</label>
			<input
				id="confirm-password"
				type="password"
				bind:value={confirmPassword}
				placeholder="Confirm password"
			/>
		</div>
	</div>
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
		align-items: center;
		gap: 20px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		width: 320px;
	}

	label {
		font-size: 0.85rem;
		font-weight: 600;
		color: #555;
	}

	input {
		font-family: inherit;
		font-size: 0.95rem;
		padding: 10px 14px;
		border: 1.5px solid #d0d0d0;
		border-radius: 8px;
		outline: none;
		transition: border-color 0.2s ease, box-shadow 0.2s ease;
		background: #fff;
	}

	input::placeholder {
		color: #aaa;
	}

	input:focus {
		border-color: #4a90d9;
		box-shadow: 0 0 0 3px rgba(74, 144, 217, 0.15);
	}
</style>
