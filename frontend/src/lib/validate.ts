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
	state: ValidationState
): ValidationResult {
	return {
		username:
			state.usernameTouched
				? !username.trim()
					? 'Username is required'
					: username.length < 3
						? 'Username must be at least 3 characters'
						: null
				: null,
		password:
			state.passwordTouched
				? !password
					? 'Password is required'
					: password.length < 6
						? 'Password must be at least 6 characters'
						: null
				: null,
		confirmPassword:
			state.confirmPasswordTouched
				? !confirmPassword
					? 'Please confirm your password'
					: confirmPassword !== password
						? 'Passwords do not match'
						: null
				: null,
	};
}
