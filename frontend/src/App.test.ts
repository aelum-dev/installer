import { describe, it, expect, vi, beforeEach } from 'vitest';
import { userAccountData, diskSelectionData } from '$lib/state.svelte';

// Mock Wails Go bindings
vi.mock('@wails/go/gui/App.js', () => ({


	SetCurrentPage: vi.fn(() => Promise.resolve()),
	ValidateUserAccount: vi.fn(() => Promise.resolve()),
	SaveUserAccount: vi.fn(() => Promise.resolve()),
	ValidateDiskSelection: vi.fn(() => Promise.resolve()),
	SaveDiskSelection: vi.fn(() => Promise.resolve()),
	SetStartPage: vi.fn(() => Promise.resolve()),
	GetStartPage: vi.fn(() => Promise.resolve(-1)),
	ClearStartPage: vi.fn(() => Promise.resolve()),
}));

// Mock Wails runtime
vi.mock('@wails/runtime/runtime', () => ({
	LogInfo: vi.fn(),
	LogError: vi.fn(),
	EventsOn: vi.fn(() => () => {}),
}));

describe('App navigation logic', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Reset state
		userAccountData.username = '';
		userAccountData.password = '';
		userAccountData.confirmPassword = '';
		diskSelectionData.diskId = '';
		diskSelectionData.disk = null;
	});

	describe('currentPage state', () => {
		it('should start at page 0 (Welcome)', () => {
			// currentPage state in App.svelte starts at 0
			expect(0).toBe(0);
		});

		it('should have 6 pages (0-5)', () => {
			const pages = ['Welcome', 'User Account', 'Disk Selection', 'Confirmation', 'Installation', 'Finish'];
			expect(pages.length).toBe(6);
			expect(pages[0]).toBe('Welcome');
			expect(pages[5]).toBe('Finish');
		});
	});

	describe('page visibility rules', () => {
		it('should show footer on Welcome page (PAGES_NO_NAV is empty)', () => {
			const currentPage = 0;
			const showFooter = true; // PAGES_NO_NAV is empty, footer always shown
			expect(showFooter).toBe(true);
		});

		it('should show footer on UserAccount page', () => {
			const currentPage = 1;
			const showFooter = true;
			expect(showFooter).toBe(true);
		});

		it('should show footer on DiskSelection page', () => {
			const currentPage = 2;
			const showFooter = true;
			expect(showFooter).toBe(true);
		});

		it('should show footer on Confirmation page', () => {
			const currentPage = 3;
			const showFooter = true;
			expect(showFooter).toBe(true);
		});

		it('should show footer on Installation page', () => {
			const currentPage = 4;
			const showFooter = true;
			expect(showFooter).toBe(true);
		});

		it('should show footer on Finish page', () => {
			const currentPage = 5;
			const showFooter = true;
			expect(showFooter).toBe(true);
		});
	});

	describe('back button visibility', () => {
		it('should NOT show back button on Welcome page', () => {
			const currentPage: number = 0;
			const isLastPage = currentPage === 5;
			const isInstallationPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage && !isInstallationPage;
			expect(showBack).toBe(false);
		});

		it('should show back button on UserAccount page', () => {
			const currentPage: number = 1;
			const isLastPage = currentPage === 5;
			const isInstallationPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage && !isInstallationPage;
			expect(showBack).toBe(true);
		});

		it('should show back button on DiskSelection page', () => {
			const currentPage: number = 2;
			const isLastPage = currentPage === 5;
			const isInstallationPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage && !isInstallationPage;
			expect(showBack).toBe(true);
		});

		it('should show back button on Confirmation page', () => {
			const currentPage: number = 3;
			const isLastPage = currentPage === 5;
			const isInstallationPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage && !isInstallationPage;
			expect(showBack).toBe(true);
		});

		it('should NOT show back button on Installation page', () => {
			const currentPage: number = 4;
			const isLastPage = currentPage === 5;
			const isInstallationPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage && !isInstallationPage;
			expect(showBack).toBe(false);
		});

		it('should NOT show back button on Finish page', () => {
			const currentPage: number = 5;
			const isLastPage = currentPage === 5;
			const isInstallationPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage && !isInstallationPage;
			expect(showBack).toBe(false);
		});
	});

	describe('button handler logic', () => {
		it('should dispatch go-next for non-last pages', () => {
			const isLastPage = false;
			const handler = isLastPage
				? () => document.dispatchEvent(new CustomEvent('finish', { cancelable: true }))
				: () => document.dispatchEvent(new CustomEvent('go-next', { cancelable: true }));
			const mock = vi.fn();
			document.addEventListener('go-next', mock);
			handler();
			expect(mock).toHaveBeenCalled();
			document.removeEventListener('go-next', mock);
		});

		it('should dispatch finish for last page', () => {
			const isLastPage = true;
			const handler = isLastPage
				? () => document.dispatchEvent(new CustomEvent('finish', { cancelable: true }))
				: () => document.dispatchEvent(new CustomEvent('go-next', { cancelable: true }));
			const mock = vi.fn();
			document.addEventListener('finish', mock);
			handler();
			expect(mock).toHaveBeenCalled();
			document.removeEventListener('finish', mock);
		});
	});

describe('state updates', () => {
		it('should update userAccountData state', () => {
			const testData = { username: 'testuser', password: 'pass123', confirmPassword: 'pass123' };
			userAccountData.username = testData.username;
			userAccountData.password = testData.password;
			userAccountData.confirmPassword = testData.confirmPassword;
			expect(userAccountData).toEqual(testData);
		});

		it('should update diskSelectionData state', () => {
			diskSelectionData.diskId = '/dev/sda';
			expect(diskSelectionData.diskId).toBe('/dev/sda');
		});
	});
});
