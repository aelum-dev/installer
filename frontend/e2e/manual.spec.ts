import { test, expect } from '@playwright/test';

// End-to-end tests for manual partition mode. These drive the
// real Wails app through `wails dev` (fake disks, dry-run), same as
// wizard.spec.ts.
//
// Manual mode is reached from the Disk Selection page: pick a disk while in
// automatic mode, then toggle to Manual. The partition editor then lets you
// designate one partition as Boot and one as Root (System), add new
// partitions, and delete rows. Next is always enabled; an invalid layout shows
// a "Cannot proceed" validation modal instead of blocking navigation.
//
// Run with: npm run e2e

// Pin to the shipped window size: main.go fixes the app at 900x600
// (MinWidth == MaxWidth), and the editor's column budget in
// ManualPartitionPage.css is measured against it. The default 1280x900
// viewport hides column collisions that every real user would see.
test.use({ viewport: { width: 900, height: 600 } });

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
  // Pin the UI language: the backend may remember a different selection from
  // the native dev window, and the rest of the flow assumes English labels.
  // (Language names carry no country since bba8044 — match the bare name.)
  await page.getByRole('button', { name: 'English', exact: true }).click();
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

  test('designating Boot + System is reflected on the rows', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    // First partition → Boot, second → Root (System), via the role menus.
    await rows.first().locator('.role-trigger').click();
    await page.getByRole('option', { name: 'Boot' }).click();
    await rows.nth(1).locator('.role-trigger').click();
    await page.getByRole('option', { name: 'System' }).click();

    // The chosen roles are reflected on the row triggers. (Assert the label
    // span: the trigger's own text ends with whitespace next to the chevron.)
    await expect(rows.first().locator('.role-trigger-label')).toHaveText('Boot');
    await expect(rows.nth(1).locator('.role-trigger-label')).toHaveText('System');
  });

  test('keeps designated roles when switching between automatic and manual', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    await rows.first().locator('.role-trigger').click();
    await page.getByRole('option', { name: 'Boot' }).click();
    await rows.nth(1).locator('.role-trigger').click();
    await page.getByRole('option', { name: 'System' }).click();
    await expect(rows.first().locator('.role-trigger-label')).toHaveText('Boot');
    await expect(rows.nth(1).locator('.role-trigger-label')).toHaveText('System');

    // Leave manual mode and come back: the plan must be re-used, not rescanned
    // into a fresh all-Preserve layout.
    await page.getByRole('button', { name: 'Automatic' }).click();
    await page.getByRole('button', { name: 'Manual' }).click();
    await expect(page.locator('.editor-row')).toHaveCount(2);
    await expect(page.locator('.editor-row').first().locator('.role-trigger-label')).toHaveText('Boot');
    await expect(page.locator('.editor-row').nth(1).locator('.role-trigger-label')).toHaveText('System');
  });

  test('adding a partition then deleting it changes the row count', async ({ page }) => {
    // /dev/sdh (Toshiba X300 8TB) has ~1.3 GB free, so a new partition fits.
    await selectDiskByModel(page, 'Toshiba X300 8TB');
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    await expect(rows).toHaveCount(2);

    // Reveal the add control by clicking the unallocated-space row, then add a
    // new 1000 MB partition.
    await page.locator('.unallocated-row').click();
    await page.locator('.number-stepper-input').fill('1000');
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
    // /dev/nvme0n1 (first disk) is fully used: there is no FREE row, so the
    // add control cannot be revealed at all.
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    await expect(page.locator('.unallocated-row')).toHaveCount(0);
    await expect(page.locator('.add-btn')).toHaveCount(0);
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
    await page.getByRole('option', { name: 'Boot' }).click();
    await rows.nth(1).locator('.role-trigger').click();
    await page.getByRole('option', { name: 'System' }).click();
    await expect(rows.first().locator('.role-trigger-label')).toHaveText('Boot');
    await expect(rows.nth(1).locator('.role-trigger-label')).toHaveText('System');

    // Selecting the active "System" role again in the menu clears it back to
    // Preserve (the trigger only toggles the menu; the option does the clearing).
    await rows.nth(1).locator('.role-trigger').click();
    await page.getByRole('option', { name: 'Preserve' }).click();
    await expect(rows.nth(1).locator('.role-trigger-label')).toHaveText('Preserve');
  });

  test('editor columns never collide on the densest fake disk', async ({ page }) => {
    // /dev/nvme3n1 (Samsung 990 PRO 4TB) is the densest case in the fixture:
    // a dozen rows whose paths reach the longest form a device path takes
    // (/dev/nvme3n1p10). With table-layout: fixed, an over-narrow Name cell
    // paints its text straight over the Size column — the regression this
    // guards. Measured at the pinned 900x600 window (test.use above).
    await selectDiskByModel(page, 'Samsung 990 PRO 4TB');
    // The rail must also fit vertically in the fixed 600px window: its
    // centered stack previously split the overflow into a step glyph clipped
    // at the top and an options card sitting under the footer. Checked in both
    // modes — the card is present (and the stack just as tall) in automatic.
    const railFits = () =>
      page.evaluate(() => {
        const area = document.querySelector('.content-area')!.getBoundingClientRect();
        const icon = document.querySelector('.page-info .page-icon')!.getBoundingClientRect();
        const card = document.querySelector('.options-card')!.getBoundingClientRect();
        return icon.top >= area.top && card.bottom <= area.bottom;
      });
    expect(await railFits(), 'rail overflows the content area in automatic mode').toBe(true);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.editor-row');
    // The layout arrives through a backend round-trip; wait for it to render
    // before measuring (a bare locator.count() would race the load).
    await expect(rows.first()).toBeVisible();
    expect(await railFits(), 'rail overflows the content area in manual mode').toBe(true);
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(10);

    for (let i = 0; i < count; i++) {
      const overlaps = await rows.nth(i).evaluate((el) => {
        const path = el.querySelector<HTMLElement>('.part-path');
        const size = el.querySelector<HTMLElement>('.part-size');
        if (!path || !size) return false;
        return path.getBoundingClientRect().right > size.getBoundingClientRect().left + 0.5;
      });
      expect(overlaps, `row ${i + 1}: path text collides with the Size column`).toBe(false);
    }

    // The longest path is shown whole — the ellipsis safety net must not even
    // engage at the shipped window size.
    await expect(page.getByText('/dev/nvme3n1p10')).toBeVisible();
    const clippedPaths = await page
      .locator('.part-path')
      .evaluateAll((els) => els.some((el) => el.scrollWidth > el.clientWidth + 1));
    expect(clippedPaths).toBe(false);

    // The role trigger keeps "Preserve" readable at its fixed column width.
    const clippedRoles = await page
      .locator('.role-trigger-label')
      .evaluateAll((els) => els.some((el) => el.scrollWidth > el.clientWidth + 1));
    expect(clippedRoles).toBe(false);
  });
});
