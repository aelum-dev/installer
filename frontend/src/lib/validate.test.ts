import { describe, it, expect } from 'vitest';
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

		it('returns error when username is less than 3 characters and touched', () => {
			const result = validateUserAccount('ab', 'password123', 'password123', touchedState);
			expect(result.username).toBe('Username must be at least 3 characters');
		});

		it('returns no error when username is exactly 3 characters and touched', () => {
			const result = validateUserAccount('abc', 'password123', 'password123', touchedState);
			expect(result.username).toBeNull();
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

		it('returns error when password is less than 6 characters and touched', () => {
			const result = validateUserAccount('john_doe', 'abc12', 'password123', touchedState);
			expect(result.password).toBe('Password must be at least 6 characters');
		});

		it('returns no error when password is exactly 6 characters and touched', () => {
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
