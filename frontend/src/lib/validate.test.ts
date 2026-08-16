import { describe, it, expect } from 'vitest';
import { MaxUsernameLength, MinPasswordLength, MinUsernameLength } from './constants';
import { validateUserAccount } from './validate';

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
		it('returns no error when username is valid and touched', () => {
			const result = validateUserAccount('john_doe', 'password123', 'password123', touchedState);
			expect(result.username).toBeNull();
		});

		it('returns error when username is empty and touched', () => {
			const result = validateUserAccount('', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username is required');
		});

		it('returns error when username is whitespace only and touched', () => {
			const result = validateUserAccount('   ', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username is required');
		});

		it('returns error when username is less than minimum length and touched', () => {
			const result = validateUserAccount('ab', 'password123', 'password123', touchedState);
			expect(result.username).toBe(`Username must be at least ${MinUsernameLength} characters`);
		});

		it('returns no error when username is exactly minimum length and touched', () => {
			const result = validateUserAccount('abc', 'password123', 'password123', touchedState);
			expect(result.username).toBeNull();
		});

		it('returns error when username exceeds maximum length and touched', () => {
			const result = validateUserAccount('a'.repeat(MaxUsernameLength + 1), 'password123', 'password123', touchedState);
			expect(result.username).toBe(`Username must be at most ${MaxUsernameLength} characters`);
		});

		it('returns no error when username is exactly maximum length and touched', () => {
			const result = validateUserAccount('a'.repeat(MaxUsernameLength), 'password123', 'password123', touchedState);
			expect(result.username).toBeNull();
		});

		it('returns error when username starts with uppercase letter', () => {
			const result = validateUserAccount('John', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username must start with a lowercase letter or underscore');
		});

		it('returns error when username contains uppercase letters (not first char)', () => {
			const result = validateUserAccount('jOhnDoe', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username must contain only lowercase letters, numbers, hyphens, and underscores');
		});

		it('returns error when username contains dots', () => {
			const result = validateUserAccount('user.name', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username must contain only lowercase letters, numbers, hyphens, and underscores');
		});

		it('returns error when username contains special characters', () => {
			const result = validateUserAccount('user@home', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username must contain only lowercase letters, numbers, hyphens, and underscores');
		});

		it('returns error when username starts with hyphen', () => {
			const result = validateUserAccount('-weird', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username must start with a lowercase letter or underscore');
		});

		it('returns error when username starts with number', () => {
			const result = validateUserAccount('1user', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username must start with a lowercase letter or underscore');
		});

		it('accepts username starting with underscore', () => {
			const result = validateUserAccount('_user', 'password123', 'password123', touchedState);
			expect(result.username).toBeNull();
		});

		it('accepts username with hyphens in the middle', () => {
			const result = validateUserAccount('user-name', 'password123', 'password123', touchedState);
			expect(result.username).toBeNull();
		});

		it('accepts username with numbers', () => {
			const result = validateUserAccount('user123', 'password123', 'password123', touchedState);
			expect(result.username).toBeNull();
		});

		it('returns error when username is root', () => {
			const result = validateUserAccount('root', 'password123', 'password123', touchedState);
			expect(result.username).toBe("Username 'root' is reserved");
		});

		it('returns error when username is bin', () => {
			const result = validateUserAccount('bin', 'password123', 'password123', touchedState);
			expect(result.username).toBe("Username 'bin' is reserved");
		});

		it('returns error when username is daemon', () => {
			const result = validateUserAccount('daemon', 'password123', 'password123', touchedState);
			expect(result.username).toBe("Username 'daemon' is reserved");
		});

		it('returns error when username is sshd', () => {
			const result = validateUserAccount('sshd', 'password123', 'password123', touchedState);
			expect(result.username).toBe("Username 'sshd' is reserved");
		});

		it('returns no errors when not touched', () => {
			const result = validateUserAccount('', '', '', emptyState);
			expect(result.username).toBeNull();
			expect(result.password).toBeNull();
			expect(result.confirmPassword).toBeNull();
		});
	});

	describe('password validation', () => {
		it('returns no error when password is valid and touched', () => {
			const result = validateUserAccount('john_doe', 'password123', 'password123', touchedState);
			expect(result.password).toBeNull();
		});

		it('returns error when password is empty and touched', () => {
			const result = validateUserAccount('john_doe', '', 'password123', touchedState);
			expect(result.password).toBe('Password is required');
		});

		it('returns error when password is less than minimum length and touched', () => {
			const result = validateUserAccount('john_doe', 'abc12', 'password123', touchedState);
			expect(result.password).toBe(`Password must be at least ${MinPasswordLength} characters`);
		});

		it('returns no error when password is exactly minimum length and touched', () => {
			const result = validateUserAccount('john_doe', 'abc123', 'abc123', touchedState);
			expect(result.password).toBeNull();
		});
	});

	describe('confirm password validation', () => {
		it('returns no error when confirm password matches and touched', () => {
			const result = validateUserAccount('john_doe', 'password123', 'password123', touchedState);
			expect(result.confirmPassword).toBeNull();
		});

		it('returns error when confirm password is empty and touched', () => {
			const result = validateUserAccount('john_doe', 'password123', '', touchedState);
			expect(result.confirmPassword).toBe('Please confirm your password');
		});

		it('returns error when confirm password does not match and touched', () => {
			const result = validateUserAccount('john_doe', 'password123', 'different', touchedState);
			expect(result.confirmPassword).toBe('Passwords do not match');
		});

		it('returns no errors when not touched', () => {
			const result = validateUserAccount('john_doe', 'pass', 'nomatch', emptyState);
			expect(result.confirmPassword).toBeNull();
		});
	});

	describe('multiple errors', () => {
		it('returns all errors when all fields are invalid', () => {
			const result = validateUserAccount('', 'ab', 'nomatch', touchedState);
			expect(result.username).toBe('Username is required');
			expect(result.password).toBe('Password must be at least 6 characters');
			expect(result.confirmPassword).toBe('Passwords do not match');
		});

		it('returns all errors when confirm is empty and password is too short', () => {
			const result = validateUserAccount('john', 'ab', '', touchedState);
			expect(result.username).toBeNull();
			expect(result.password).toBe('Password must be at least 6 characters');
			expect(result.confirmPassword).toBe('Please confirm your password');
		});
	});
});
