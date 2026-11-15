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

// The row's chosen role: the selected pill inside its role group.
const activeRole = (row) => row.locator('.role-pill.selected');

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

    // Switch to manual mode. The rail keeps its frame — the same head style
    // with the partitioning wording, and the destination card below it.
    await page.getByRole('button', { name: 'Manual' }).click();
    await expect(page.getByRole('heading', { name: 'Select Partitions' })).toBeVisible();
    await expect(page.locator('.disk-page .dest-card')).toBeVisible();
    // The card draws the plan strip once the layout arrives (sync point).
    await expect(page.locator('.disk-page .dest-card .strip .seg').first()).toBeVisible();

    // /dev/nvme0n1 has 2 existing partitions, shown by their device path.
    await expect(page.getByText('/dev/nvme0n1p1')).toBeVisible();
    await expect(page.getByText('/dev/nvme0n1p2')).toBeVisible();

    // Each row exposes a role pill group (Preserve by default) → 2 selected.
    await expect(page.locator('.role-pills')).toHaveCount(2);
    await expect(activeRole(page.locator('.part-row').first())).toHaveText('Preserve');
    await expect(activeRole(page.locator('.part-row').nth(1))).toHaveText('Preserve');
  });

  test('designating Boot + System is reflected on the rows', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.part-row');
    // First partition → Boot, second → Root (System), via the row pills.
    await rows.first().getByRole('button', { name: 'Boot' }).click();
    await rows.nth(1).getByRole('button', { name: 'System' }).click();

    // The chosen roles are marked on the pills and framed on the rows.
    await expect(activeRole(rows.first())).toHaveText('Boot');
    await expect(activeRole(rows.nth(1))).toHaveText('System');
    await expect(rows.first()).toHaveClass(/part-row--role/);
    await expect(rows.nth(1)).toHaveClass(/part-row--role/);
  });

  test('keeps designated roles when switching between automatic and manual', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.part-row');
    await rows.first().getByRole('button', { name: 'Boot' }).click();
    await rows.nth(1).getByRole('button', { name: 'System' }).click();
    await expect(activeRole(rows.first())).toHaveText('Boot');
    await expect(activeRole(rows.nth(1))).toHaveText('System');

    // Leave manual mode and come back: the plan must be re-used, not rescanned
    // into a fresh all-Preserve layout.
    await page.getByRole('button', { name: 'Automatic' }).click();
    await page.getByRole('button', { name: 'Manual' }).click();
    await expect(page.locator('.part-row')).toHaveCount(2);
    await expect(activeRole(page.locator('.part-row').first())).toHaveText('Boot');
    await expect(activeRole(page.locator('.part-row').nth(1))).toHaveText('System');
  });

  test('adding a partition then deleting it changes the row count', async ({ page }) => {
    // /dev/sdh (Toshiba X300 8TB) has ~1.3 GB free, so a new partition fits.
    await selectDiskByModel(page, 'Toshiba X300 8TB');
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.part-row');
    await expect(rows).toHaveCount(2);

    // Reveal the add control by clicking the unallocated-space row, then add a
    // new 1000 MB partition.
    await page.locator('.free-row').click();
    await page.locator('.number-stepper-input').fill('1000');
    await page.locator('.add-btn').click();

    // A new blank row appears, labelled "New partition".
    await expect(page.getByText('New partition', { exact: true })).toBeVisible();
    await expect(page.locator('.part-row')).toHaveCount(3);

    // Delete the new (last) row via the confirmation dialog.
    await page
      .locator('.part-row')
      .last()
      .getByRole('button', { name: 'Delete partition' })
      .click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Remove from install plan?' })).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Remove' }).click();

    await expect(page.getByRole('dialog')).not.toBeVisible();
    await expect(page.locator('.part-row')).toHaveCount(2);
  });

  test('adding is blocked when the disk has no free space', async ({ page }) => {
    // /dev/nvme0n1 (first disk) is fully used: there is no FREE row, so the
    // add control cannot be revealed at all.
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    await expect(page.locator('.free-row')).toHaveCount(0);
    await expect(page.locator('.add-btn')).toHaveCount(0);
  });

  test('cancelling the delete dialog keeps the partition', async ({ page }) => {
    await selectDiskByModel(page, 'Toshiba X300 8TB');
    await page.getByRole('button', { name: 'Manual' }).click();

    await page
      .locator('.part-row')
      .first()
      .getByRole('button', { name: 'Delete partition' })
      .click();
    await expect(page.getByRole('dialog')).toBeVisible();

    // Cancel → dialog closes, partition remains.
    await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click();
    await expect(page.getByRole('dialog')).not.toBeVisible();
    await expect(page.locator('.part-row')).toHaveCount(2);
  });

  test('clearing a required role reverts the row to Preserve', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.part-row');
    await rows.first().getByRole('button', { name: 'Boot' }).click();
    await rows.nth(1).getByRole('button', { name: 'System' }).click();
    await expect(activeRole(rows.first())).toHaveText('Boot');
    await expect(activeRole(rows.nth(1))).toHaveText('System');

    // Re-clicking the active "System" pill clears it back to Preserve.
    await rows.nth(1).getByRole('button', { name: 'System' }).click();
    await expect(activeRole(rows.nth(1))).toHaveText('Preserve');
  });

  test('the disk map previews the plan and the designated rows', async ({ page }) => {
    await selectFirstDisk(page);
    await page.getByRole('button', { name: 'Manual' }).click();

    const rows = page.locator('.part-row');
    await rows.first().getByRole('button', { name: 'Boot' }).click();
    await rows.nth(1).getByRole('button', { name: 'System' }).click();

    // The map labels the designated segments and outlines them — the same
    // roles the pills just chose, drawn on the disk's shape. Labels are the
    // DOM text (CSS uppercases them for display), and a segment under 5% of
    // the disk is too thin to label — its tooltip carries the name instead.
    await expect(page.locator('.map-seg--role')).toHaveCount(2);
    const labels = await page.locator('.map-seg').evaluateAll((els) =>
      els.map((el) => el.textContent?.trim() ?? ''),
    );
    expect(labels).toContain('Root');
    // The 512 MB Boot segment is under the 5% label floor — the tooltip
    // carries its name there, so only the System row is labeled.
    expect(labels[0]).toBe('');

    // The caption names the plan once, in the destination chain's words
    // (the wizard's default filesystem is btrfs).
    await expect(page.locator('.map-caption')).toContainText('btrfs root');
  });

  test('editor rows never collide on the densest fake disk', async ({ page }) => {
    // /dev/nvme3n1 (Samsung 990 PRO 4TB) is the densest case in the fixture:
    // a dozen rows whose paths reach the longest form a device path takes
    // (/dev/nvme3n1p10). A row card that cannot grow shrinks its id column —
    // the path either fits or ellipsizes, never painting over the size.
    // Measured at the pinned 900x600 window (test.use above).
    await selectDiskByModel(page, 'Samsung 990 PRO 4TB');
    // The rail must also fit vertically in the fixed 600px window: its
    // compact head keeps every card inside the content area in both modes.
    // Scoped to .disk-page and run only after navigation transitions settle
    // — an outgoing page mid-fade also mounts a .page-info, and a bare
    // querySelector would measure that ghost instead of this page's rail.
    const railFits = () =>
      page.evaluate(() => {
        const area = document.querySelector('.content-area')!.getBoundingClientRect();
        const icon = document.querySelector('.disk-page .page-info .page-icon')!.getBoundingClientRect();
        const card = document.querySelector('.disk-page .options-card')!.getBoundingClientRect();
        // The destination card sits under the options card in both modes and
        // must stay fully inside too — the manual variant carries the plan
        // strip and the free-space bar and grows past its automatic twin.
        const dest = document.querySelector('.disk-page .dest-card')!.getBoundingClientRect();
        return icon.top >= area.top && dest.bottom <= area.bottom && dest.top >= card.bottom;
      });
    const settled = () => expect(page.locator('.content-area > .page[data-exiting]')).toHaveCount(0);
    await settled();
    expect(await railFits(), 'rail overflows the content area in automatic mode').toBe(true);
    await page.getByRole('button', { name: 'Manual' }).click();
    await settled();

    const rows = page.locator('.part-row');
    // The layout arrives through a backend round-trip; wait for it to render
    // before measuring (a bare locator.count() would race the load).
    await expect(rows.first()).toBeVisible();
    // The manual plan strip renders once the layout lands; the rail card then
    // carries strip + free-space bar + note and is at its tallest.
    await expect(page.locator('.disk-page .dest-card .strip .seg').first()).toBeVisible();
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

    // The role pills keep their localized labels readable at the shipped width.
    const clippedRoles = await page
      .locator('.role-pill')
      .evaluateAll((els) => els.some((el) => el.scrollWidth > el.clientWidth + 1));
    expect(clippedRoles).toBe(false);
  });
});
