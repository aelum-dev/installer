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

// Dismiss the "Cannot proceed" validation modal (OK-only). Next is always
// enabled, so invalid input surfaces this modal instead of blocking navigation.
async function dismissValidationModal(page: import('@playwright/test').Page): Promise<void> {
  // `exact` so the language buttons (e.g. "Norsk bokmål") don't also match "OK".
  await page.getByRole('button', { name: 'OK', exact: true }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
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

  test('User account: shows the validation modal on bad input, advances when valid', async ({ page }) => {
    await page.goto('/');
    await next(page).click();

    await expect(page.getByRole('heading', { name: 'Create user account' })).toBeVisible();

    // Next is always enabled; with nothing entered it shows the validation
    // modal instead of advancing.
    await next(page).click();
    await expect(page.getByRole('heading', { name: 'Cannot proceed' })).toBeVisible();
    await dismissValidationModal(page);

    // Too-short username shows an inline validation error.
    await page.getByLabel('Username').fill('ab');
    await page.getByLabel('Username').blur();
    await expect(page.locator('.error.visible')).toContainText(/username/i);

    // Valid + matching passwords -> Next advances to disk selection.
    await page.getByLabel('Username').fill('alice');
    await page.getByLabel('Password', { exact: true }).fill('secret1');
    await page.getByLabel('Confirm Password').fill('secret1');
    await next(page).click();
    await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();
  });

  test('User account: flags mismatched passwords', async ({ page }) => {
    await page.goto('/');
    await next(page).click();

    await fillUserAccount(page);
    // Deliberately break the confirm field.
    await page.getByLabel('Confirm Password').fill('different');
    await page.getByLabel('Confirm Password').blur();

    await expect(page.locator('.error.visible')).toContainText(/match/i);

    // Next is always enabled; clicking it surfaces the mismatch via the modal.
    await next(page).click();
    await expect(page.getByRole('heading', { name: 'Cannot proceed' })).toBeVisible();
    await dismissValidationModal(page);
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

  test('Disk selection: selected disk persists after returning from the summary', async ({ page }) => {
    await page.goto('/');
    await next(page).click(); // -> User account
    await fillUserAccount(page);
    await next(page).click(); // -> Disk selection

    const card = page.locator('.disk-card').first();
    const diskName = await card.locator('.disk-name').innerText();
    await card.click();
    await expect(card).toHaveClass(/selected/);

    await next(page).click(); // -> Confirmation
    await expect(page.getByRole('heading', { name: 'Installation Summary' })).toBeVisible();

    await back(page).click(); // -> Disk selection again
    await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();
    await expect(page.locator('.disk-card.selected')).toBeVisible();
    await expect(page.locator('.disk-card.selected .disk-name')).toHaveText(diskName);
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
