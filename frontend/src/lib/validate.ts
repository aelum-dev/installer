import { MaxPasswordLength, MaxUsernameLength } from './constants';
import {
	RESERVED_USERNAMES,
	USERNAME_ALL_DIGITS,
	USERNAME_ALLOWED_CHARS,
	USERNAME_FIRST_CHAR,
} from './validation-rules';

// Single rule engine for the user-account page.
//
// The inline field hints (`validateUserAccount`) and the shared "Cannot
// proceed" modal (`page-validation.ts`) both derive from the functions below,
// so the two can no longer disagree about what counts as valid.
//
// These are Linux's rules, not a product policy: shadow's useradd accepts the
// username forms allowed here, and PAM accepts any non-empty password up to
// 511 UTF-8 bytes.

export type UsernameRule =
	| 'usernameRequired'
	| 'usernameTooLong'
	| 'usernameDotNames'
	| 'usernameDigitsOnly'
	| 'usernameInvalidStart'
	| 'usernameInvalidCharacters'
	| 'usernameReserved';

export type PasswordRule = 'passwordRequired' | 'passwordTooLong' | 'passwordInvalidCharacters';

export type ConfirmPasswordRule = 'passwordsMatch' | 'passwordsNoMatch';

export type ValidationMessageKey =
	| UsernameRule
	| PasswordRule
	| ConfirmPasswordRule;

const textEncoder = new TextEncoder();

/** First failing username rule, or null when valid. */
export function usernameRule(username: string): UsernameRule | null {
	if (!username.trim()) return 'usernameRequired';
	if (username.length > MaxUsernameLength) return 'usernameTooLong';
	if (username === '.' || username === '..') return 'usernameDotNames';
	if (USERNAME_ALL_DIGITS.test(username)) return 'usernameDigitsOnly';
	if (!USERNAME_FIRST_CHAR.test(username)) return 'usernameInvalidStart';
	if (!USERNAME_ALLOWED_CHARS.test(username)) return 'usernameInvalidCharacters';
	if (RESERVED_USERNAMES.has(username)) return 'usernameReserved';
	return null;
}

/** First failing password rule, or null when valid. */
export function passwordRule(password: string): PasswordRule | null {
	if (!password) return 'passwordRequired';
	// chpasswd is line-based and PAM passwords are C strings, so neither can
	// represent a newline or NUL. Everything else (spaces, punctuation, any
	// UTF-8) is accepted by Linux.
	if (/[\n\r\u0000]/.test(password)) return 'passwordInvalidCharacters';
	if (textEncoder.encode(password).length > MaxPasswordLength) return 'passwordTooLong';
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
	usernameTooLong: string;
	usernameDotNames: string;
	usernameDigitsOnly: string;
	usernameInvalidStart: string;
	usernameInvalidCharacters: string;
	usernameReserved: string;
	passwordRequired: string;
	passwordTooLong: string;
	passwordInvalidCharacters: string;
	passwordsMatch: string;
	passwordsNoMatch: string;
}

const DEFAULT_MESSAGES: ValidationMessages = {
	usernameRequired: 'Username is required',
	usernameTooLong: `Username must be at most ${MaxUsernameLength} characters`,
	usernameDotNames: 'Username cannot be "." or ".."',
	usernameDigitsOnly: 'Username cannot consist only of digits',
	usernameInvalidStart: 'Username must start with a letter, number, underscore, or dot',
	usernameInvalidCharacters:
		'Username may only contain letters, numbers, dots, hyphens, and underscores (with an optional trailing \'$\')',
	usernameReserved: 'Username \'{name}\' is reserved',
	passwordRequired: 'Password is required',
	passwordTooLong: `Password must be at most ${MaxPasswordLength} UTF-8 bytes`,
	passwordInvalidCharacters: 'Password cannot contain newlines or NUL characters',
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
