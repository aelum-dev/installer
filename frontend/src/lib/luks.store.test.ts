import { describe, it, expect, afterEach } from 'vitest';
import { commitLuksPassphrase, disableLuks, requestEnableLuks, installState } from './store.svelte';
import { uiState } from './ui.svelte';

afterEach(() => {
	// Reset LUKS state between tests.
	installState.luksEnabled = false;
	installState.luksPassphrase = '';
	uiState.luksDialogOpen = false;
	uiState.luksError = '';
});

describe('requestEnableLuks', () => {
	it('opens the passphrase popup without enabling encryption', () => {
		requestEnableLuks();
		expect(uiState.luksDialogOpen).toBe(true);
		expect(installState.luksEnabled).toBe(false);
	});
});

describe('commitLuksPassphrase', () => {
	it('enables encryption and stores the passphrase on a match', () => {
		commitLuksPassphrase('s3cret', 's3cret');
		expect(installState.luksEnabled).toBe(true);
		expect(installState.luksPassphrase).toBe('s3cret');
		expect(uiState.luksDialogOpen).toBe(false);
		expect(uiState.luksError).toBe('');
	});

	it('keeps the popup open and errors when the fields diverge', () => {
		commitLuksPassphrase('s3cret', 'different');
		expect(installState.luksEnabled).toBe(false);
		expect(uiState.luksDialogOpen).toBe(true);
		expect(uiState.luksError).not.toBe('');
	});

	it('keeps the popup open and errors when empty', () => {
		commitLuksPassphrase('', '');
		expect(installState.luksEnabled).toBe(false);
		expect(uiState.luksDialogOpen).toBe(true);
		expect(uiState.luksError).not.toBe('');
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
