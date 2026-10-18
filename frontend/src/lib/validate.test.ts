import { describe, it, expect } from 'vitest';
import { MaxPasswordLength, MaxUsernameLength } from './constants';
import {
	validateUserAccount,
	type ValidationMessages,
	type ValidationState,
} from './validate';

// The rule engine no longer carries English defaults, so the test owns its
// message fixtures. These tests assert *which* rule fires; the wording itself
// is the i18n layer's concern.
const messages: ValidationMessages = {
	usernameRequired: 'Username is required',
	usernameTooLong: `Username must be at most ${MaxUsernameLength} characters`,
	usernameDotNames: 'Username cannot be "." or ".."',
	usernameDigitsOnly: 'Username cannot consist only of digits',
	usernameInvalidStart: 'Username must start with a letter, number, underscore, or dot',
	usernameInvalidCharacters:
		"Username may only contain letters, numbers, dots, hyphens, and underscores (with an optional trailing '$')",
	usernameReserved: "Username '{name}' is reserved",
	passwordRequired: 'Password is required',
	passwordTooLong: `Password must be at most ${MaxPasswordLength} UTF-8 bytes`,
	passwordInvalidCharacters: 'Password cannot contain newlines or NUL characters',
	passwordsMatch: 'Please confirm your password',
	passwordsNoMatch: 'Passwords do not match',
};

function validate(
	username: string,
	password: string,
	confirmPassword: string,
	state: ValidationState,
) {
	return validateUserAccount(username, password, confirmPassword, state, messages);
}

const emptyState = {
	usernameTouched: false,
	passwordTouched: false,
	confirmPasswordTouched: false,
};

const touchedState = {
	usernameTouched: true,
	passwordTouched: true,
	confirmPasswordTouched: true,
};

describe('validateUserAccount', () => {
	describe('username validation', () => {
		it('accepts the same names Linux useradd accepts', () => {
			for (const name of [
				'a',
				'alice',
				'Alice',
				'ALICE',
				'1alice',
				'_alice',
				'.alice',
				'alice.b',
				'alice-b',
				'alice_b',
				'alice$',
				'1.23',
				'123$',
				'a'.repeat(MaxUsernameLength),
			]) {
				expect(validate(name, 'password', 'password', touchedState).username, name).toBeNull();
			}
		});

		it('returns error when username is empty and touched', () => {
			const result = validate('', 'password', 'password', touchedState);
			expect(result.username).toBe('Username is required');
		});

		it('returns error when username is only whitespace and touched', () => {
			const result = validate('   ', 'password', 'password', touchedState);
			expect(result.username).toBe('Username is required');
		});

		it('returns error when username exceeds the Linux maximum length', () => {
			const result = validate('a'.repeat(MaxUsernameLength + 1), 'password', 'password', touchedState);
			expect(result.username).toBe(`Username must be at most ${MaxUsernameLength} characters`);
		});

		it('rejects all-digit names like useradd does', () => {
			const result = validate('123', 'password', 'password', touchedState);
			expect(result.username).toBe('Username cannot consist only of digits');
		});

		it('rejects "." and ".." like useradd does', () => {
			expect(validate('.', 'password', 'password', touchedState).username).toBe(
				'Username cannot be "." or ".."',
			);
			expect(validate('..', 'password', 'password', touchedState).username).toBe(
				'Username cannot be "." or ".."',
			);
		});

		it('rejects a leading hyphen', () => {
			const result = validate('-alice', 'password', 'password', touchedState);
			expect(result.username).toBe('Username must start with a letter, number, underscore, or dot');
		});

		it('rejects characters Linux useradd rejects', () => {
			for (const name of ['alice@home', 'alice x', 'alice#x', 'alice:x', 'alice/x', 'alice+x', 'a$b', 'aliceä']) {
				const result = validate(name, 'password', 'password', touchedState);
				expect(result.username, name).toBe(
					"Username may only contain letters, numbers, dots, hyphens, and underscores (with an optional trailing '$')",
				);
			}
		});

		it('reports existing target accounts (users and groups)', () => {
			expect(validate('root', 'password', 'password', touchedState).username).toBe(
				"Username 'root' is reserved",
			);
			expect(validate('wheel', 'password', 'password', touchedState).username).toBe(
				"Username 'wheel' is reserved",
			);
		});

		it('allows names that are not on the target, even if other distros use them', () => {
			expect(validate('avahi', 'password', 'password', touchedState).username).toBeNull();
		});

		it('returns no errors when not touched', () => {
			const result = validate('', '', '', emptyState);
			expect(result.username).toBeNull();
			expect(result.password).toBeNull();
			expect(result.confirmPassword).toBeNull();
		});
	});

	describe('password validation', () => {
		it('accepts a single character, like Linux/PAM', () => {
			const result = validate('alice', 'x', 'x', touchedState);
			expect(result.password).toBeNull();
		});

		it('returns error when password is empty and touched', () => {
			const result = validate('alice', '', 'x', touchedState);
			expect(result.password).toBe('Password is required');
		});

		it('accepts a password of exactly the PAM maximum in bytes', () => {
			const password = 'a'.repeat(MaxPasswordLength);
			const result = validate('alice', password, password, touchedState);
			expect(result.password).toBeNull();
		});

		it('rejects a password above the PAM maximum in bytes', () => {
			const password = 'a'.repeat(MaxPasswordLength + 1);
			const result = validate('alice', password, password, touchedState);
			expect(result.password).toBe(`Password must be at most ${MaxPasswordLength} UTF-8 bytes`);
		});

		it('counts UTF-8 bytes rather than UTF-16 code units', () => {
			// 256 × "é" is 256 characters but 512 bytes, which PAM rejects.
			const tooLong = 'é'.repeat(256);
			expect(validate('alice', tooLong, tooLong, touchedState).password).toBe(
				`Password must be at most ${MaxPasswordLength} UTF-8 bytes`,
			);
			// 255 × "é" is 510 bytes and fits.
			const fits = 'é'.repeat(255);
			expect(validate('alice', fits, fits, touchedState).password).toBeNull();
		});

		it('rejects newlines and NULs that chpasswd/PAM cannot represent', () => {
			for (const password of ['pass\nword', 'pass\rword', 'pass\u0000word']) {
				expect(validate('alice', password, password, touchedState).password).toBe(
					'Password cannot contain newlines or NUL characters',
				);
			}
		});
	});

	describe('confirm password validation', () => {
		it('returns no error when confirm password matches and touched', () => {
			const result = validate('alice', 'password', 'password', touchedState);
			expect(result.confirmPassword).toBeNull();
		});

		it('returns error when confirm password is empty and touched', () => {
			const result = validate('alice', 'password', '', touchedState);
			expect(result.confirmPassword).toBe('Please confirm your password');
		});

		it('returns error when confirm password does not match and touched', () => {
			const result = validate('alice', 'password', 'different', touchedState);
			expect(result.confirmPassword).toBe('Passwords do not match');
		});

		it('returns no errors when not touched', () => {
			const result = validate('alice', 'pass', 'nomatch', emptyState);
			expect(result.confirmPassword).toBeNull();
		});
	});

	describe('multiple errors', () => {
		it('returns all errors when all fields are invalid', () => {
			const result = validate('123', 'a'.repeat(MaxPasswordLength + 1), 'nomatch', touchedState);
			expect(result.username).toBe('Username cannot consist only of digits');
			expect(result.password).toBe(`Password must be at most ${MaxPasswordLength} UTF-8 bytes`);
			expect(result.confirmPassword).toBe('Passwords do not match');
		});
	});
});
