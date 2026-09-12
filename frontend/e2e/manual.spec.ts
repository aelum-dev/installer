import { test, expect } from '@playwright/test';

// End-to-end tests for manual (dualboot-safe) partition mode. These drive the
// real Wails app through `wails dev` (fake disks, dry-run), same as
// wizard.spec.ts.
//
// Manual mode is reached from the Disk Selection page: pick a disk while in
// automatic mode, then toggle to Manual. The partition editor then lets you
// designate one partition as Boot (ESP) and one as Root (System), add new
// partitions, and delete rows. Next is always enabled; an invalid layout shows
// a "Cannot proceed" validation modal instead of blocking navigation.
//
// Run with: npm run e2e

const next = (page) => page.getByRole('button', { name: 'Next' });

// Select the first fake disk (/dev/nvme0n1, 2 existing partitions).
async function selectFirstDisk(page: import('@playwright/test').Page): Promise<void> {
  await gotoDiskSelection(page);
  await page.locator('.disk-card').first().click();
  await expect(page.locator('.disk-card.selected')).toBeVisible();
}

// Select a disk by its model name (used for a disk that has free space).
async function selectDiskByModel(page: import('@playwright/test').Page, model: string): Promise<void> {
  await gotoDiskSelection(page);
  const card = page
    .locator('.disk-card')
    .filter({ has: page.locator('.disk-name').getByText(model, { exact: true }) });
  await card.first().click();
  await expect(page.locator('.disk-card.selected')).toBeVisible();
}

// Navigate Welcome -> User account -> Disk selection (automatic mode).
async function gotoDiskSelection(page: import('@playwright/test').Page): Promise<void> {
  await page.goto('/');
  await next(page).click(); // Welcome -> User account
  await page.getByLabel('Username').fill('alice');
  await page.getByLabel('Password', { exact: true }).fill('secret1');
  await page.getByLabel('Confirm Password').fill('secret1');
  await next(page).click(); // User account -> Disk selection
  await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();
}

test.describe('Manual partitioning', () => {
  test('manual mode is disabled until a disk is selected', async ({ page }) => {
    await gotoDiskSelection(page);

    // No disk selected yet: the Manual mode pill is disabled, and hovering it
    // shows a "select a disk first" hint instead of switching modes.
    const manual = page.getByRole('button', { name: 'Manual' });
    await expect(manual).toBeDisabled();
    await manual.hover();
    await expect(
      page.getByText('Select a disk before switching to manual mode.', { exact: true }),
    ).toBeVisible();
  });

  test('shows existing partitions as Preserve', async ({ page }) => {
    await selectFirstDisk(page);

    // Switch to manual mode.
    await page.getByRole('button', { name: 'Manual' }).click();

    // /dev/nvme0n1 has 2 existing partitions, shown by their device path.
    await expect(page.getByText('/dev/nvme0n1p1')).toBeVisible();
    await expect(page.getByText('/dev/nvme0n1p2')).toBeVisible();

    // Each row exposes a role selector (Preserve by default) → 2 controls.
    await expect(page.locator('.role-trigger')).toHaveCount(2);
    await expect(page.locator('.role-trigger').filter({ hasText: 'Preserve' })).toHaveCount(2);
  });

  test('designating ESP + System is reflected on the rows', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    // First partition → Boot (ESP), second → Root (System), via the role menus.
    await rows.first().locator('.role-trigger').click();
    await page.getByRole('option', { name: 'ESP' }).click();
    await rows.nth(1).locator('.role-trigger').click();
    await page.getByRole('option', { name: 'System' }).click();

    // The chosen roles are reflected on the row triggers.
    await expect(rows.first().locator('.role-trigger')).toHaveText('ESP');
    await expect(rows.nth(1).locator('.role-trigger')).toHaveText('System');
  });

  test('adding a partition then deleting it changes the row count', async ({ page }) => {
    // /dev/sdh (Toshiba X300 8TB) has ~1.3 GB free, so a new partition fits.
    await selectDiskByModel(page, 'Toshiba X300 8TB');
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    await expect(rows).toHaveCount(2);

    // Add a new 1000 MB partition.
    await page.locator('.add-size-input').fill('1000');
    await page.locator('.add-btn').click();

    // A new blank row appears, labelled "New partition".
    await expect(page.getByText('New partition', { exact: true })).toBeVisible();
    await expect(page.locator('.editor-row')).toHaveCount(3);

    // Delete the new (last) row via the confirmation dialog.
    await page
      .locator('.editor-row')
      .last()
      .getByRole('button', { name: 'Delete partition' })
      .click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Remove from install plan?' })).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Remove' }).click();

    await expect(page.getByRole('dialog')).not.toBeVisible();
    await expect(page.locator('.editor-row')).toHaveCount(2);
  });

  test('adding is blocked when the disk has no free space', async ({ page }) => {
    // /dev/nvme0n1 (first disk) is fully used, so the Add control is disabled.
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    // The disk is fully used, so the Add button is disabled (the size input
    // itself stays enabled; it is only disabled when "Use remaining" is on).
    await expect(page.locator('.add-btn')).toBeDisabled();
  });

  test('cancelling the delete dialog keeps the partition', async ({ page }) => {
    await selectDiskByModel(page, 'Toshiba X300 8TB');
    await page.getByRole('button', { name: 'Manual' }).click();

    await page
      .locator('.editor-row')
      .first()
      .getByRole('button', { name: 'Delete partition' })
      .click();
    await expect(page.getByRole('dialog')).toBeVisible();

    // Cancel → dialog closes, partition remains.
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();
    await expect(page.getByRole('dialog')).not.toBeVisible();
    await expect(page.locator('.editor-row')).toHaveCount(2);
  });

  test('clearing a required role reverts the row to Preserve', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    await rows.first().locator('.role-trigger').click();
    await page.getByRole('option', { name: 'ESP' }).click();
    await rows.nth(1).locator('.role-trigger').click();
    await page.getByRole('option', { name: 'System' }).click();
    await expect(rows.first().locator('.role-trigger')).toHaveText('ESP');
    await expect(rows.nth(1).locator('.role-trigger')).toHaveText('System');

    // Selecting the active "System" role again in the menu clears it back to
    // Preserve (the trigger only toggles the menu; the option does the clearing).
    await rows.nth(1).locator('.role-trigger').click();
    await page.getByRole('option', { name: 'Preserve' }).click();
    await expect(rows.nth(1).locator('.role-trigger')).toHaveText('Preserve');
  });
});
