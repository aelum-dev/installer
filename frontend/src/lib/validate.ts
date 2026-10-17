import { MaxUsernameLength, MinPasswordLength, MinUsernameLength } from './constants';
import { RESERVED_USERNAMES, USERNAME_ALLOWED_CHARS, USERNAME_FIRST_CHAR } from './validation-rules';

// Single rule engine for the user-account page.
//
// The inline field hints (`validateUserAccount`) and the shared "Cannot
// proceed" modal (`page-validation.ts`) both derive from the functions below,
// so the two can no longer disagree about what counts as valid.

export type UsernameRule =
	| 'usernameRequired'
	| 'usernameMinLength'
	| 'usernameMaxLength'
	| 'usernameInvalidFirstChar'
	| 'usernameInvalidChars'
	| 'usernameReserved';

export type PasswordRule = 'passwordRequired' | 'passwordMinLength';

export type ConfirmPasswordRule = 'passwordsMatch' | 'passwordsNoMatch';

export type ValidationMessageKey =
	| UsernameRule
	| PasswordRule
	| ConfirmPasswordRule;

/** First failing username rule, or null when valid. */
export function usernameRule(username: string): UsernameRule | null {
	if (!username.trim()) return 'usernameRequired';
	if (username.length < MinUsernameLength) return 'usernameMinLength';
	if (username.length > MaxUsernameLength) return 'usernameMaxLength';
	if (!USERNAME_FIRST_CHAR.test(username)) return 'usernameInvalidFirstChar';
	if (!USERNAME_ALLOWED_CHARS.test(username)) return 'usernameInvalidChars';
	if (RESERVED_USERNAMES.has(username.toLowerCase())) return 'usernameReserved';
	return null;
}

/** First failing password rule, or null when valid. */
export function passwordRule(password: string): PasswordRule | null {
	if (!password) return 'passwordRequired';
	if (password.length < MinPasswordLength) return 'passwordMinLength';
	return null;
}

/** First failing confirmation rule, or null when valid. */
export function confirmPasswordRule(
	confirmPassword: string,
	password: string,
): ConfirmPasswordRule | null {
	if (!confirmPassword) return 'passwordsMatch';
	if (confirmPassword !== password) return 'passwordsNoMatch';
	return null;
}

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

/** Human-readable messages, supplied by the i18n layer. */
export interface ValidationMessages {
	usernameRequired: string;
	usernameMinLength: string;
	usernameMaxLength: string;
	usernameInvalidFirstChar: string;
	usernameInvalidChars: string;
	usernameReserved: string;
	passwordRequired: string;
	passwordMinLength: string;
	passwordsMatch: string;
	passwordsNoMatch: string;
}

const DEFAULT_MESSAGES: ValidationMessages = {
	usernameRequired: 'Username is required',
	usernameMinLength: `Username must be at least ${MinUsernameLength} characters`,
	usernameMaxLength: `Username must be at most ${MaxUsernameLength} characters`,
	usernameInvalidFirstChar: 'Username must start with a lowercase letter or underscore',
	usernameInvalidChars: 'Username must contain only lowercase letters, numbers, hyphens, and underscores',
	usernameReserved: 'Username \'{name}\' is reserved',
	passwordRequired: 'Password is required',
	passwordMinLength: `Password must be at least ${MinPasswordLength} characters`,
	passwordsMatch: 'Please confirm your password',
	passwordsNoMatch: 'Passwords do not match',
};

function messageFor(rule: ValidationMessageKey, messages: ValidationMessages, username = ''): string {
	const text = messages[rule];
	return text.replace('{name}', username);
}

/**
 * Inline validation for the user-account form. Only fields the user has
 * touched are reported, so the form starts quiet.
 */
export function validateUserAccount(
	username: string,
	password: string,
	confirmPassword: string,
	state: ValidationState,
	messages: ValidationMessages = DEFAULT_MESSAGES,
): ValidationResult {
	const usernameFailure = state.usernameTouched ? usernameRule(username) : null;
	const passwordFailure = state.passwordTouched ? passwordRule(password) : null;
	const confirmFailure = state.confirmPasswordTouched
		? confirmPasswordRule(confirmPassword, password)
		: null;

	return {
		username: usernameFailure ? messageFor(usernameFailure, messages, username) : null,
		password: passwordFailure ? messageFor(passwordFailure, messages) : null,
		confirmPassword: confirmFailure ? messageFor(confirmFailure, messages) : null,
	};
}
