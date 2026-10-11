import { describe, it, expect, afterEach } from 'vitest';
import { commitLuksPassphrase, disableLuks, requestEnableLuks, installState } from './store.svelte';

afterEach(() => {
	// Reset LUKS state between tests.
	installState.luksEnabled = false;
	installState.luksPassphrase = '';
	installState.luksDialogOpen = false;
	installState.luksError = '';
});

describe('requestEnableLuks', () => {
	it('opens the passphrase popup without enabling encryption', () => {
		requestEnableLuks();
		expect(installState.luksDialogOpen).toBe(true);
		expect(installState.luksEnabled).toBe(false);
	});
});

describe('commitLuksPassphrase', () => {
	it('enables encryption and stores the passphrase on a match', () => {
		commitLuksPassphrase('s3cret', 's3cret');
		expect(installState.luksEnabled).toBe(true);
		expect(installState.luksPassphrase).toBe('s3cret');
		expect(installState.luksDialogOpen).toBe(false);
		expect(installState.luksError).toBe('');
	});

	it('keeps the popup open and errors when the fields diverge', () => {
		commitLuksPassphrase('s3cret', 'different');
		expect(installState.luksEnabled).toBe(false);
		expect(installState.luksDialogOpen).toBe(true);
		expect(installState.luksError).not.toBe('');
	});

	it('keeps the popup open and errors when empty', () => {
		commitLuksPassphrase('', '');
		expect(installState.luksEnabled).toBe(false);
		expect(installState.luksDialogOpen).toBe(true);
		expect(installState.luksError).not.toBe('');
	});
});

describe('disableLuks', () => {
	it('clears the passphrase and disables encryption', () => {
		installState.luksEnabled = true;
		installState.luksPassphrase = 's3cret';
		disableLuks();
		expect(installState.luksEnabled).toBe(false);
		expect(installState.luksPassphrase).toBe('');
	});
});
