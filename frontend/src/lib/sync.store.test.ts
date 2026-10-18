import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { installState, syncEncryptionToBackend } from './store.svelte';
import { SetLuks, SetLuksPassphrase, SetLvm } from '@wails/go/gui/App.js';

vi.mock('@wails/go/gui/App.js', () => ({
	SetLuks: vi.fn(() => Promise.resolve()),
	SetLuksPassphrase: vi.fn(() => Promise.resolve()),
	SetLvm: vi.fn(() => Promise.resolve()),
}));

// syncEncryptionToBackend runs right before StartInstallation. The Wails runtime
// dispatches each Go binding call on its own goroutine, so a fire-and-forget sync
// can hand the CLI `--luks` before the passphrase lands; the CLI then exits
// before its first step and the user sees a failure at 0%. These tests pin the
// ordering (passphrase first, awaited) that keeps that state unobservable.
describe('syncEncryptionToBackend', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		installState.luksEnabled = false;
		installState.luksPassphrase = '';
		installState.lvmEnabled = false;
	});

	afterEach(() => {
		installState.luksEnabled = false;
		installState.luksPassphrase = '';
		installState.lvmEnabled = false;
	});

	it('sends the passphrase before flagging encryption on', async () => {
		installState.luksEnabled = true;
		installState.luksPassphrase = 's3cret';

		await syncEncryptionToBackend();

		expect(SetLuksPassphrase).toHaveBeenCalledWith('s3cret');
		expect(SetLuks).toHaveBeenCalledWith(true);
		expect(vi.mocked(SetLuksPassphrase).mock.invocationCallOrder[0]).toBeLessThan(
			vi.mocked(SetLuks).mock.invocationCallOrder[0],
		);
	});

	it('does not enable encryption until the passphrase call resolves', async () => {
		installState.luksEnabled = true;
		installState.luksPassphrase = 's3cret';

		let releasePassphrase!: () => void;
		const pending = new Promise<void>((resolve) => {
			releasePassphrase = resolve;
		});
		vi.mocked(SetLuksPassphrase).mockImplementationOnce(() => pending);

		const sync = syncEncryptionToBackend();

		// The passphrase call is in flight: LUKS must still read as disabled.
		expect(SetLuksPassphrase).toHaveBeenCalledWith('s3cret');
		expect(SetLuks).not.toHaveBeenCalled();

		releasePassphrase();
		await sync;

		expect(SetLuks).toHaveBeenCalledWith(true);
	});

	it('awaits the LVM flag too', async () => {
		installState.lvmEnabled = true;

		let releaseLvm!: () => void;
		const pending = new Promise<void>((resolve) => {
			releaseLvm = resolve;
		});
		vi.mocked(SetLvm).mockImplementationOnce(() => pending);

		let settled = false;
		const sync = syncEncryptionToBackend().then(() => {
			settled = true;
		});

		await Promise.resolve();
		expect(settled).toBe(false);

		releaseLvm();
		await sync;
		expect(SetLvm).toHaveBeenCalledWith(true);
	});

	it('disables encryption when no passphrase is held', async () => {
		installState.luksEnabled = false;
		installState.luksPassphrase = '';

		await syncEncryptionToBackend();

		expect(SetLuks).toHaveBeenCalledWith(false);
		expect(SetLuksPassphrase).not.toHaveBeenCalled();
	});

	it('never enables encryption on a passphrase-less toggle', async () => {
		// A committed toggle with no held passphrase must not reach the backend as
		// "encrypt": the CLI would abort at 0% for lack of a passphrase.
		installState.luksEnabled = true;
		installState.luksPassphrase = '';

		await syncEncryptionToBackend();

		expect(SetLuks).toHaveBeenCalledWith(false);
		expect(SetLuksPassphrase).not.toHaveBeenCalled();
	});
});
