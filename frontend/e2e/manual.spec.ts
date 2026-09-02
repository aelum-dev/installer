import { test, expect } from '@playwright/test';

// End-to-end tests for manual (dualboot-safe) partition mode. These drive the
// real Wails app through `wails dev` (fake disks, dry-run), same as
// wizard.spec.ts.
//
// Manual mode is reached from the Disk Selection page: pick a disk while in
// automatic mode, then toggle to Manual. The partition editor then lets you
// designate one partition as Boot (ESP) and one as Root (System), add new
// partitions, and delete rows. A valid layout (exactly one ESP + one System) is
// what enables "Next".
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
  test('manual mode without a disk shows a selection hint', async ({ page }) => {
    await gotoDiskSelection(page);

    // Switch to manual before choosing a disk.
    await page.getByRole('button', { name: 'Manual' }).click();
    await expect(
      page.getByText('Select a disk, then switch to manual mode', { exact: false }),
    ).toBeVisible();
    // Nothing to validate yet.
    await expect(next(page)).toBeDisabled();
  });

  test('shows existing partitions as Preserve and keeps Next disabled', async ({ page }) => {
    await selectFirstDisk(page);

    // Switch to manual mode.
    await page.getByRole('button', { name: 'Manual' }).click();

    // /dev/nvme0n1 has 2 existing partitions, shown by their device path.
    await expect(page.getByText('/dev/nvme0n1p1')).toBeVisible();
    await expect(page.getByText('/dev/nvme0n1p2')).toBeVisible();

    // Each row exposes Preserve / ESP / System pills → 6 pills total.
    await expect(page.locator('.type-pill')).toHaveCount(6);

    // All partitions are Preserve (untouched) → no ESP/System → Next disabled.
    await expect(page.locator('.type-pill.selected', { hasText: 'Preserve' })).toHaveCount(2);
    await expect(next(page)).toBeDisabled();
  });

  test('designating ESP + System enables Next', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    // First partition → Boot (ESP), second → Root (System).
    await rows.first().getByRole('button', { name: 'ESP' }).click();
    await rows.nth(1).getByRole('button', { name: 'System' }).click();

    // The chosen roles are reflected as selected pills.
    await expect(rows.first().getByRole('button', { name: 'ESP', exact: true })).toHaveClass(
      /selected/,
    );
    await expect(rows.nth(1).getByRole('button', { name: 'System', exact: true })).toHaveClass(
      /selected/,
    );

    // Exactly one ESP + one System → valid → Next enabled.
    await expect(next(page)).toBeEnabled();
  });

  test('adding a partition then deleting it changes the row count', async ({ page }) => {
    // /dev/sdh (Toshiba X300 8TB) has ~1.3 GB free, so a new partition fits.
    await selectDiskByModel(page, 'Toshiba X300 8TB');
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    await expect(rows).toHaveCount(2);

    // Add a new 1000 MB partition.
    await page.locator('.add-input').fill('1000');
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
    await expect(page.getByRole('heading', { name: 'Delete partition?' })).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();

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

  test('Next disables again when a required type is cleared', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    await rows.first().getByRole('button', { name: 'ESP' }).click();
    await rows.nth(1).getByRole('button', { name: 'System' }).click();
    await expect(next(page)).toBeEnabled();

    // Clicking the active "System" pill clears it back to Preserve → invalid.
    await rows.nth(1).getByRole('button', { name: 'System' }).click();
    await expect(next(page)).toBeDisabled();
  });
});
