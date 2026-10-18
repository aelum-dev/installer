import { describe, it, expect, vi, beforeEach } from 'vitest';
import { installState } from '$lib/store.svelte';
import { uiState } from '$lib/ui.svelte';
import { goNext, goBack } from '$lib/navigation';
import { PAGE_IDS } from '$lib/types';
import type { DiskInfo } from '$lib/types';

// Mock Wails Go bindings
vi.mock('@wails/go/gui/App.js', () => ({
	SetCurrentPage: vi.fn(() => Promise.resolve()),
	SetWallpaper: vi.fn(() => Promise.resolve()),
	SaveUserAccount: vi.fn(() => Promise.resolve()),
	SaveDiskSelection: vi.fn(() => Promise.resolve()),
	SaveManualLayout: vi.fn(() => Promise.resolve()),
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
};

describe('App navigation', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		installState.username = '';
		installState.password = '';
		installState.confirmPassword = '';
		installState.diskId = '';
		installState.disk = null;
		installState.fileSystem = '';
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
		});
	});
});
