import { describe, it, expect, vi, beforeEach } from 'vitest';
import { installState, resetDiskChoicesIfStale } from '$lib/store.svelte';
import { uiState } from '$lib/ui.svelte';
import { canGoNext, goBack, goNext } from '$lib/navigation';
import { DefaultFileSystem } from '$lib/constants';
import { PAGE_IDS } from '$lib/types';
import type { DiskInfo, ManualLayout } from '$lib/types';

// Mock Wails Go bindings
vi.mock('@wails/go/gui/App.js', () => ({
	SetCurrentPage: vi.fn(() => Promise.resolve()),
	SetWallpaper: vi.fn(() => Promise.resolve()),
	SaveUserAccount: vi.fn(() => Promise.resolve()),
	SaveDiskSelection: vi.fn(() => Promise.resolve()),
	SaveManualLayout: vi.fn(() => Promise.resolve()),
	ResetDiskSelection: vi.fn(() => Promise.resolve()),
	RebootAndClose: vi.fn(() => Promise.resolve()),
}));

// Mock Wails runtime
vi.mock('@wails/runtime/runtime', () => ({
	LogInfo: vi.fn(),
	LogError: vi.fn(),
}));

const fakeDisk: DiskInfo = {
	name: 'sda',
	path: '/dev/sda',
	size: 1000000000,
	sizeFormatted: '1.0 GB',
	type: 'disk',
	model: 'Test',
	vendor: '',
	rotational: false,
	readonly: false,
	fstype: null,
	mountpoint: null,
	serial: '',
	trans: '',
	isRemovable: false,
	partitionTable: 'gpt',
	inUse: false,
	isLiveDevice: false,
};

describe('App navigation', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// The deferred-reset marker is module state: drain it so a test can
		// never inherit a pending wipe from its predecessor.
		resetDiskChoicesIfStale();
		installState.username = '';
		installState.password = '';
		installState.confirmPassword = '';
		installState.diskId = '';
		installState.disk = null;
		installState.fileSystem = '';
		installState.luksEnabled = false;
		installState.luksPassphrase = '';
		installState.lvmEnabled = false;
		installState.manualMode = false;
		installState.manualLayout = null;
		uiState.showConfirmDialog = false;
	});

	describe('goNext', () => {
		it('should advance from Welcome to UserAccount', async () => {
			installState.locale = 'en-US';
			const result = await goNext(PAGE_IDS.WELCOME);
			expect(result).toBeNull();
		});

		it('should save user account data when advancing from UserAccount', async () => {
			installState.locale = 'en-US';
			installState.username = 'testuser';
			installState.password = 'pass123';
			installState.confirmPassword = 'pass123';
			const result = await goNext(PAGE_IDS.USER_ACCOUNT);
			expect(result).toBeNull();
			expect(installState.username).toBe('testuser');
		});

		it('should show confirmation dialog on Confirmation page', async () => {
			installState.locale = 'en-US';
			const result = await goNext(PAGE_IDS.CONFIRMATION);
			expect(result).toBeNull();
			expect(uiState.showConfirmDialog).toBe(true);
		});

		it('surfaces a save failure instead of rejecting', async () => {
			const { SaveUserAccount } = await import('@wails/go/gui/App.js');
			vi.mocked(SaveUserAccount).mockRejectedValueOnce(new Error('save failed'));
			installState.locale = 'en-US';

			const result = await goNext(PAGE_IDS.USER_ACCOUNT);

			expect(result).toBe('save failed');
		});
	});

	describe('goBack', () => {
		it('should go back from DiskSelection to UserAccount', async () => {
			installState.locale = 'en-US';
			const result = await goBack(PAGE_IDS.DISK_SELECTION);
			expect(result).toBeNull();
		});

		it('returns the backend error when navigation fails', async () => {
			const { SetCurrentPage } = await import('@wails/go/gui/App.js');
			vi.mocked(SetCurrentPage).mockRejectedValueOnce(new Error('nav failed'));

			await expect(goBack(PAGE_IDS.DISK_SELECTION)).resolves.toBe('nav failed');
		});

		it('should preserve disk selection when going back from Confirmation', async () => {
			installState.diskId = '/dev/sda';
			installState.disk = fakeDisk;
			await goBack(PAGE_IDS.CONFIRMATION);
			expect(installState.diskId).toBe('/dev/sda');
			expect(installState.disk).not.toBeNull();
			// A summary visit must not even mark the choices stale: the next
			// mount restores everything.
			resetDiskChoicesIfStale();
			expect(installState.disk).not.toBeNull();
		});

		it('leaving the disk page backwards keeps its answers on screen, then resets them on the next mount', async () => {
			installState.diskId = '/dev/sda';
			installState.disk = fakeDisk;
			installState.fileSystem = 'ext4';
			installState.luksEnabled = true;
			installState.luksPassphrase = 'secret1';
			installState.lvmEnabled = true;
			installState.manualMode = true;
			installState.manualLayout = { disk: '/dev/sda', partitions: [] } as unknown as ManualLayout;

			const result = await goBack(PAGE_IDS.DISK_SELECTION);

			expect(result).toBeNull();
			// The disk page is still fading out showing its answers — the wipe
			// must not touch the shared state during the transition…
			expect(installState.disk).not.toBeNull();
			expect(installState.luksPassphrase).toBe('secret1');
			// …but the backend copy is cleared right away, secret included.
			const { ResetDiskSelection } = await import('@wails/go/gui/App.js');
			expect(ResetDiskSelection).toHaveBeenCalledOnce();

			// The page's next mount performs the deferred reset.
			resetDiskChoicesIfStale();
			expect(installState.diskId).toBe('');
			expect(installState.disk).toBeNull();
			expect(installState.fileSystem).toBe(DefaultFileSystem);
			expect(installState.luksEnabled).toBe(false);
			expect(installState.luksPassphrase).toBe('');
			expect(installState.lvmEnabled).toBe(false);
			expect(installState.manualMode).toBe(false);
			expect(installState.manualLayout).toBeNull();
			// And performs it exactly once: a second mount changes nothing.
			installState.lvmEnabled = true;
			resetDiskChoicesIfStale();
			expect(installState.lvmEnabled).toBe(true);
		});
	});
});

describe('canGoNext', () => {
	// The footer asks this one predicate whether to show Next and Back, so a screen
	// nobody may navigate away from has to answer it itself.
	it.each([
		['the running installation', PAGE_IDS.INSTALLATION],
		['the cancelled screen', PAGE_IDS.CANCELLED],
		['the finish page', PAGE_IDS.FINISH],
	])('refuses to advance from %s', (_name, page) => {
		expect(canGoNext(page)).toBe(false);
	});

	it.each([
		PAGE_IDS.WELCOME,
		PAGE_IDS.USER_ACCOUNT,
		PAGE_IDS.DISK_SELECTION,
		PAGE_IDS.CONFIRMATION,
	])('advances from page %i', (page) => {
		expect(canGoNext(page)).toBe(true);
	});
});
