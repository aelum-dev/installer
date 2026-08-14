export interface ValidationResult {
	username: string | null;
	password: string | null;
	confirmPassword: string | null;
}

export interface ValidationState {
	usernameTouched: boolean;
	passwordTouched: boolean;
	confirmPasswordTouched: boolean;
}

export function validateUserAccount(
	username: string,
	password: string,
	confirmPassword: string,
	state: ValidationState,
	messages?: { usernameRequired: string; usernameMinLength: string; passwordRequired: string; passwordMinLength: string; passwordsMatch: string; passwordsNoMatch: string }
): ValidationResult {
	const msg = messages ?? {
		usernameRequired: 'Username is required',
		usernameMinLength: 'Username must be at least 3 characters',
		passwordRequired: 'Password is required',
		passwordMinLength: 'Password must be at least 6 characters',
		passwordsMatch: 'Please confirm your password',
		passwordsNoMatch: 'Passwords do not match',
	};
	return {
		username:
			state.usernameTouched
				? !username.trim()
					? msg.usernameRequired
					: username.length < 3
						? msg.usernameMinLength
						: null
				: null,
		password:
			state.passwordTouched
				? !password
					? msg.passwordRequired
					: password.length < 6
						? msg.passwordMinLength
						: null
				: null,
		confirmPassword:
			state.confirmPasswordTouched
				? !confirmPassword
					? msg.passwordsMatch
					: confirmPassword !== password
						? msg.passwordsNoMatch
						: null
				: null,
	};
}
