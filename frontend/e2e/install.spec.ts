import { test, expect } from '@playwright/test';

// Full end-to-end flow: Welcome -> User account -> Disk selection ->
// Confirmation -> Installation -> Finish.
//
// Runs under `wails dev` in dry-run mode (see playwright.config.ts), so the
// install steps simulate quickly and no real disk is touched. The GUI spawns
// the CLI without sudo in dry-run, so this works from a normal dev build.
//
// Run with: npm run e2e

const next = (page: import('@playwright/test').Page) => page.getByRole('button', { name: 'Next' });

async function fillUserAccount(page: import('@playwright/test').Page): Promise<void> {
  await page.getByLabel('Username').fill('alice');
  await page.getByLabel('Password', { exact: true }).fill('secret1');
  await page.getByLabel('Confirm Password').fill('secret1');
}

test('full wizard flow reaches the finish screen', async ({ page }) => {
  // The dry-run install itself runs ~10s; the global 30s timeout is enough,
  // but leave the terminal-state expect room to report the real cause
  // instead of the generic test-timeout message.
  test.setTimeout(60_000);
  await page.goto('/');

  // Welcome -> User account
  await next(page).click();
  await fillUserAccount(page);
  await expect(next(page)).toBeEnabled();

  // User account -> Disk selection
  await next(page).click();
  await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();
  await page.locator('.disk-card').first().click();
  await page.locator('.fs-pill').filter({ hasText: /^ext4$/ }).click();

  // Disk selection -> Confirmation
  await next(page).click();
  await expect(page.getByRole('heading', { name: 'Installation Summary' })).toBeVisible();

  // Confirmation -> Installation. The first Next opens a confirm dialog;
  // clicking "Confirm" commits and advances to Installation.
  await next(page).click();
  await expect(page.getByRole('heading', { name: 'Confirm Installation' })).toBeVisible();
  await page.getByRole('button', { name: 'Confirm' }).click();
  await expect(page.getByRole('heading', { name: 'Installing Aelum' })).toBeVisible();

  // The install runs (dry-run) and settles: it completes and advances to the
  // Finish screen. A failed start instead surfaces as error text in the
  // .step-label (InstallationPage sets errorMessage -> currentLabel), so a
  // failure shows its real cause right in the failure snapshot.
  await expect(page.getByText('Aelum has been installed successfully.')).toBeVisible({
    timeout: 60_000,
  });
});
