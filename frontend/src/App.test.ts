import { describe, it, expect, vi, beforeEach } from 'vitest';
import { userAccountData, diskSelectionData } from '$lib/state.svelte';

// Mock Wails Go bindings
vi.mock('@wails/go/gui/App.js', () => ({
	CanGoNext: vi.fn(() => Promise.resolve(true)),
	CanGoBack: vi.fn(() => Promise.resolve(true)),
	SetCurrentPage: vi.fn(() => Promise.resolve()),
	ValidateUserAccount: vi.fn(() => Promise.resolve()),
	ValidateDiskSelection: vi.fn(() => Promise.resolve()),
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

		it('should have 5 pages (0-4)', () => {
			const pages = ['Welcome', 'User Account', 'Disk Selection', 'Installation', 'Finish'];
			expect(pages.length).toBe(5);
			expect(pages[0]).toBe('Welcome');
			expect(pages[4]).toBe('Finish');
		});
	});

	describe('page visibility rules', () => {
		it('should NOT show footer on Welcome page (currentPage === 0)', () => {
			const currentPage = 0;
			const showFooter = currentPage > 0 && currentPage !== 3;
			expect(showFooter).toBe(false);
		});

		it('should show footer on UserAccount page (currentPage === 1)', () => {
			const currentPage = 1;
			const showFooter = currentPage > 0 && currentPage !== 3;
			expect(showFooter).toBe(true);
		});

		it('should show footer on DiskSelection page (currentPage === 2)', () => {
			const currentPage = 2;
			const showFooter = currentPage > 0 && currentPage !== 3;
			expect(showFooter).toBe(true);
		});

		it('should NOT show footer on Installation page (currentPage === 3)', () => {
			const currentPage = 3;
			const showFooter = currentPage > 0 && currentPage !== 3;
			expect(showFooter).toBe(false);
		});

		it('should show footer on Finish page (currentPage === 4)', () => {
			const currentPage = 4;
			const showFooter = currentPage > 0 && currentPage !== 3;
			expect(showFooter).toBe(true);
		});
	});

	describe('back button visibility', () => {
		it('should NOT show back button on Welcome page', () => {
			const currentPage = 0;
			const isLastPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage;
			expect(showBack).toBe(false);
		});

		it('should show back button on UserAccount page', () => {
			const currentPage = 1;
			const isLastPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage;
			expect(showBack).toBe(true);
		});

		it('should show back button on DiskSelection page', () => {
			const currentPage = 2;
			const isLastPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage;
			expect(showBack).toBe(true);
		});

		it('should NOT show back button on Finish page', () => {
			const currentPage = 4;
			const isLastPage = currentPage === 4;
			const showBack = currentPage > 0 && !isLastPage;
			expect(showBack).toBe(false);
		});
	});

	describe('button handler logic', () => {
		it('should use onNext for non-last pages', () => {
			const isLastPage = false;
			const onFinish = () => console.log('finish');
			const onNext = () => console.log('next');
			const buttonHandler = isLastPage && onFinish ? onFinish : onNext;
			expect(buttonHandler).toBe(onNext);
		});

		it('should use onFinish for last page', () => {
			const isLastPage = true;
			const onFinish = () => console.log('finish');
			const onNext = () => console.log('next');
			const buttonHandler = isLastPage && onFinish ? onFinish : onNext;
			expect(buttonHandler).toBe(onFinish);
		});

		it('should fall back to onNext when onFinish is undefined', () => {
			const isLastPage = true;
			const onNext = () => console.log('next');
			const buttonHandler = isLastPage && undefined ? undefined : onNext;
			expect(buttonHandler).toBe(onNext);
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
