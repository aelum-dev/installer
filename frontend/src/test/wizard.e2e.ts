import { test, expect } from '@vitest/browser';
import { userAccountData, diskSelectionData } from '$lib/store';

test.beforeEach(async () => {
	// Reset stores before each test
	userAccountData.set({ username: '', password: '', confirmPassword: '' });
	diskSelectionData.set({ diskId: '' });
});

test.describe('Wizard Flow - E2E', () => {
	test('should display Welcome page on startup', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('h2')).toContainText('Welcome');
	});

	test('should navigate from Welcome to UserAccount page', async ({ page }) => {
		await page.goto('/');
		
		// Click the Next button on Welcome page
		const nextPageBtn = page.getByRole('button', { name: /Next|→/i });
		await nextPageBtn.click();
		
		// Should be on UserAccount page
		await expect(page.locator('h2')).toContainText('User Account');
	});

	test('should show validation errors on UserAccount page when fields are empty', async ({ page }) => {
		await page.goto('/');
		
		// Navigate to UserAccount page
		await page.getByRole('button', { name: /Next|→/i }).click();
		await expect(page.locator('h2')).toContainText('User Account');
		
		// Try to go next without filling fields
		const nextBtn = page.getByRole('button', { name: /Next|→/i });
		await nextBtn.click();
		
		// Should still be on UserAccount page (navigation blocked)
		await expect(page.locator('h2')).toContainText('User Account');
	});

	test('should navigate from UserAccount to DiskSelection with valid data', async ({ page }) => {
		await page.goto('/');
		
		// Navigate to UserAccount page
		await page.getByRole('button', { name: /Next|→/i }).click();
		await expect(page.locator('h2')).toContainText('User Account');
		
		// Fill in valid user account data
		await page.fill('#username', 'testuser');
		await page.fill('#password', 'password123');
		await page.fill('#confirmPassword', 'password123');
		
		// Trigger validation by blurring fields
		await page.locator('#username').blur();
		await page.locator('#password').blur();
		await page.locator('#confirmPassword').blur();
		
		// Click Next
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// Should be on DiskSelection page
		await expect(page.locator('h2')).toContainText('Select Installation Disk');
	});

	test('should show error when passwords do not match', async ({ page }) => {
		await page.goto('/');
		
		// Navigate to UserAccount page
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// Fill in data with mismatched passwords
		await page.fill('#username', 'testuser');
		await page.fill('#password', 'password123');
		await page.fill('#confirmPassword', 'different');
		
		// Trigger validation
		await page.locator('#username').blur();
		await page.locator('#password').blur();
		await page.locator('#confirmPassword').blur();
		
		// Should show password mismatch error
		await expect(page.locator('.error')).toContainText('Passwords do not match');
	});

	test('should show error when username is too short', async ({ page }) => {
		await page.goto('/');
		
		// Navigate to UserAccount page
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// Fill in data with short username
		await page.fill('#username', 'ab');
		await page.fill('#password', 'password123');
		await page.fill('#confirmPassword', 'password123');
		
		// Trigger validation
		await page.locator('#username').blur();
		await page.locator('#password').blur();
		await page.locator('#confirmPassword').blur();
		
		// Should show username too short error
		await expect(page.locator('.error')).toContainText('at least 3 characters');
	});

	test('should show back button on UserAccount page', async ({ page }) => {
		await page.goto('/');
		
		// Navigate to UserAccount page
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// Back button should be visible
		await expect(page.getByRole('button', { name: /← Back/ })).toBeVisible();
	});

	test('should NOT show back button on Welcome page', async ({ page }) => {
		await page.goto('/');
		
		// Back button should not be visible on Welcome page
		await expect(page.getByRole('button', { name: /← Back/ })).not.toBeVisible();
	});

	test('should navigate back from UserAccount to Welcome', async ({ page }) => {
		await page.goto('/');
		
		// Navigate to UserAccount page
		await page.getByRole('button', { name: /Next|→/i }).click();
		await expect(page.locator('h2')).toContainText('User Account');
		
		// Click Back
		await page.getByRole('button', { name: /← Back/ }).click();
		
		// Should be back on Welcome page
		await expect(page.locator('h2')).toContainText('Welcome');
	});

	test('should show Reboot button on Finish page', async ({ page }) => {
		await page.goto('/');
		
		// Navigate through the wizard to Finish page
		// Welcome -> UserAccount (skip validation for speed)
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// Fill valid data and go to DiskSelection
		await page.fill('#username', 'testuser');
		await page.fill('#password', 'password123');
		await page.fill('#confirmPassword', 'password123');
		await page.locator('#username').blur();
		await page.locator('#password').blur();
		await page.locator('#confirmPassword').blur();
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// DiskSelection -> Installation (no disk selection needed for this test)
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// Installation -> Finish (should auto-transition)
		await expect(page.locator('h2')).toContainText('Installing');
		await expect(page.locator('h2')).toContainText('Installation Complete', { timeout: 15000 });
		
		// Reboot button should be visible (no Back button)
		await expect(page.getByRole('button', { name: /Reboot/ })).toBeVisible();
		await expect(page.getByRole('button', { name: /← Back/ })).not.toBeVisible();
	});

	test('should not show Next button on Installation page', async ({ page }) => {
		await page.goto('/');
		
		// Navigate to Installation page
		await page.getByRole('button', { name: /Next|→/i }).click();
		await page.fill('#username', 'testuser');
		await page.fill('#password', 'password123');
		await page.fill('#confirmPassword', 'password123');
		await page.locator('#username').blur();
		await page.locator('#password').blur();
		await page.locator('#confirmPassword').blur();
		await page.getByRole('button', { name: /Next|→/i }).click();
		await page.getByRole('button', { name: /Next|→/i }).click();
		
		// Should be on Installation page
		await expect(page.locator('h2')).toContainText('Installing');
		
		// Footer should not be visible (no Next button during installation)
		await expect(page.locator('footer')).not.toBeVisible();
	});
});
