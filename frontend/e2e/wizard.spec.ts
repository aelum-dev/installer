import { test, expect } from '@playwright/test';

// End-to-end tests for the installer wizard navigation. These exercise the full
// stack: Svelte frontend + Go backend bindings served by `wails dev` (fake
// disks, dry-run). Run with: npm run e2e

const next = (page) => page.getByRole('button', { name: 'Next' });
const back = (page) => page.getByRole('button', { name: 'Back' });

async function fillUserAccount(page: import('@playwright/test').Page): Promise<void> {
  await page.getByLabel('Username').fill('alice');
  await page.getByLabel('Password', { exact: true }).fill('secret1');
  await page.getByLabel('Confirm Password').fill('secret1');
}

test.describe('Installer wizard', () => {
  test('Welcome page loads languages and enables Next', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByRole('heading', { name: 'Welcome to Aelum' })).toBeVisible();

    // A language is auto-selected on load (required to proceed).
    await expect(page.locator('.lang-item--selected')).toBeVisible();

    // Next is enabled because a language is selected.
    await expect(next(page)).toBeEnabled();
  });

  test('User account: blocks invalid input, enables Next when valid', async ({ page }) => {
    await page.goto('/');
    await next(page).click();

    await expect(page.getByRole('heading', { name: 'Create user account' })).toBeVisible();

    // Nothing entered -> Next disabled.
    await expect(next(page)).toBeDisabled();

    // Too-short username shows a validation error.
    await page.getByLabel('Username').fill('ab');
    await page.getByLabel('Username').blur();
    await expect(page.locator('.error.visible')).toContainText(/username/i);

    // Valid + matching passwords enable Next.
    await page.getByLabel('Username').fill('alice');
    await page.getByLabel('Password', { exact: true }).fill('secret1');
    await page.getByLabel('Confirm Password').fill('secret1');
    await expect(next(page)).toBeEnabled();
  });

  test('User account: flags mismatched passwords', async ({ page }) => {
    await page.goto('/');
    await next(page).click();

    await fillUserAccount(page);
    // Deliberately break the confirm field.
    await page.getByLabel('Confirm Password').fill('different');
    await page.getByLabel('Confirm Password').blur();

    await expect(page.locator('.error.visible')).toContainText(/match/i);
    await expect(next(page)).toBeDisabled();
  });

  test('Disk selection: select a disk and filesystem, then enable Next', async ({ page }) => {
    await page.goto('/');
    await next(page).click(); // -> User account
    await fillUserAccount(page);
    await next(page).click(); // -> Disk selection

    await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();

    // A fake disk is present and selectable.
    await expect(page.locator('.disk-card').first()).toBeVisible();
    await page.locator('.disk-card').first().click();
    await expect(page.locator('.disk-card.selected')).toBeVisible();

    // Pick a filesystem.
    await page.locator('.fs-pill').filter({ hasText: /^ext4$/ }).click();
    await expect(page.locator('.fs-pill.selected')).toContainText('ext4');

    await expect(next(page)).toBeEnabled();
  });

  test('Confirmation: summarizes language, user, and disk', async ({ page }) => {
    await page.goto('/');
    await next(page).click(); // -> User account
    await fillUserAccount(page);
    await next(page).click(); // -> Disk selection
    await page.locator('.disk-card').first().click();
    await page.locator('.fs-pill').filter({ hasText: /^ext4$/ }).click();
    await next(page).click(); // -> Confirmation

    await expect(page.getByRole('heading', { name: 'Installation Summary' })).toBeVisible();
    await expect(page.getByText('Language')).toBeVisible();
    await expect(page.getByText('User Account')).toBeVisible();
    await expect(page.getByText('Target Disk')).toBeVisible();
    await expect(page.locator('.disk-path code')).toHaveText(/dev/);
  });

  test('Back button navigates between pages', async ({ page }) => {
    await page.goto('/');
    await next(page).click(); // -> User account
    await expect(page.getByRole('heading', { name: 'Create user account' })).toBeVisible();

    await back(page).click();
    await expect(page.getByRole('heading', { name: 'Welcome to Aelum' })).toBeVisible();

    await next(page).click(); // -> User account again
    await back(page).click();
    await expect(page.getByRole('heading', { name: 'Welcome to Aelum' })).toBeVisible();
  });
});
