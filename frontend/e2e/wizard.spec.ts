import { test, expect } from '@playwright/test';
import { settleEnglishBackend } from './reset';

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
  // English pins the UI language and, through the cascade, the location and
  // clock: the warm backend remembers whatever the last session chose, and
  // every assertion below is the English wizard starting from en_US. Selected
  // by language code, so it works from any starting UI (see screenshots.spec).
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.locator('.option-item[data-code="en_US"]').click();
    await settleEnglishBackend(page);
  });

  test('Welcome page loads languages and enables Next', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByRole('heading', { name: 'Welcome to Aelum' })).toBeVisible();

    // A language is auto-selected on load (required to proceed).
    await expect(page.locator('.option-item--selected')).toBeVisible();

    // Name and territory separated by exactly one space. The tail is its own
    // span (pinned to a Latin font), and Svelte's whitespace trimming has
    // broken that space twice — swallowed, then doubled — so compare the raw
    // textContent, which no normalisation forgives.
    expect(await page.locator('.option-item--selected .option-main').textContent()).toBe(
      'English (United States)',
    );
    expect(await page.locator('.summary .s-value').first().textContent()).toBe(
      'English (United States)',
    );

    // The list opens scrolled to that language: it is the first row in view,
    // not something to hunt for deep in the table. (en_US sits mid-table once
    // the list is sorted by native name, so this only passes if it scrolled.)
    await expect
      .poll(async () => {
        const row = await page.locator('.option-item--selected').boundingBox();
        const box = await page.locator('.scroll-box').boundingBox();
        return row && box ? Math.abs(row.y - box.y) : Number.MAX_SAFE_INTEGER;
      })
      .toBeLessThan(4);

    // Next is enabled because a language is selected.
    await expect(next(page)).toBeEnabled();
  });

  test('Welcome page: switching list opens it on the current answer', async ({ page }) => {
    await page.goto('/');

    // How far the scroll sits from "selected row at the top" — the one place
    // that can differ is the bottom of the list, where the browser clamps the
    // scroll and the row rides at the foot of the view instead. That is the
    // same distance-to-ideal, so the assertion is viewport-independent.
    const topOffset = () =>
      page.evaluate(() => {
        const box = document.querySelector('.scroll-box');
        const row = document.querySelector('.option-item--selected');
        if (!box || !row) return Number.MAX_SAFE_INTEGER;
        const inContent = row.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop;
        const maxScroll = box.scrollHeight - box.clientHeight;
        return Math.abs(box.scrollTop - Math.min(inContent, maxScroll));
      });
    const summary = page.locator('.summary');

    // Location: the cascade's pre-fill (United States) is brought as far to the
    // top of the country list as the list allows — near the list's end, that is
    // its foot, which the clamped distance-to-ideal accepts.
    await summary.getByRole('button', { name: /Location/ }).click();
    await expect(page.locator('.option-item--selected')).toHaveAttribute('data-code', 'us');
    await expect.poll(topOffset).toBeLessThan(4);

    // Time zone: the country's clock list opens on the selected clock too.
    await summary.getByRole('button', { name: /Time zone/ }).click();
    await expect(page.locator('.option-item--selected')).toHaveAttribute('data-code', 'America/New_York');
    await expect.poll(topOffset).toBeLessThan(4);

    // And back to Language: the language leads again, not the scroll position
    // the country list left behind.
    await summary.getByRole('button', { name: /Language/ }).click();
    await expect(page.locator('.option-item--selected')).toHaveAttribute('data-code', 'en_US');
    await expect.poll(topOffset).toBeLessThan(4);
  });

  test('Welcome page: three answers in the rail, one list in the content column', async ({ page }) => {
    await page.goto('/');

    // The summary card holds all three answers and is the only way to switch which
    // list the content column edits.
    const summary = page.locator('.summary');
    await expect(summary.getByRole('button')).toHaveCount(3);
    await expect(summary.getByText('Language')).toBeVisible();
    await expect(summary.getByText('Location')).toBeVisible();
    await expect(summary.getByText('Time zone')).toBeVisible();

    // It opens on the language list, and the row it selected is the marked one.
    await expect(page.locator('.option-item--selected')).toHaveAttribute('data-code', 'en_US');

    // The pre-selected language brings its country and that country's clock with
    // it, so the card is complete on the first paint — the same three answers a
    // click produces, not an empty location and clock until the person touches
    // the list.
    await expect(summary.getByRole('button', { name: /Keyboard: US/ })).toBeVisible();
    await expect(summary.getByRole('button', { name: /New York/ })).toBeVisible();

    // Location: the list becomes every country. A country Arch has no mirror
    // section for is still there (Afghanistan), and every row says where the
    // installed system's updates will come from — Ukraine has its own section, so
    // it names it; Afghanistan falls back to the worldwide list.
    await summary.getByRole('button', { name: /Location/ }).click();
    await expect(page.locator('.option-item[data-code="af"]')).toContainText('Mirrors: Worldwide');
    await page.locator('.option-item[data-code="ua"]').click();
    await expect(summary.getByText('Ukraine')).toBeVisible();
    await expect(summary.getByText(/Mirrors: UA/)).toBeVisible();

    // Time zone: the country's own clocks first, with the whole set one press away.
    await summary.getByRole('button', { name: /Time zone/ }).click();
    await expect(page.locator('.tz-pill')).toHaveCount(2);
    await expect(page.locator('.option-item')).toHaveCount(2); // Kyiv and Simferopol

    await page.locator('.tz-pill', { hasText: 'All zones' }).click();
    await page.locator('.option-item[data-code="Europe/Kyiv"]').click();
    // The city leads the row and the summary, so the offset beside it is not
    // asserted here — it moves with the season.
    await expect(summary.getByRole('button', { name: /Kyiv/ })).toBeVisible();

    // The clock the person named survives the wizard: the summary carries it.
    await next(page).click();
    await fillUserAccount(page);
    await next(page).click(); // Disk selection
    await page.locator('.disk-card').first().click();
    await next(page).click(); // Confirmation
    // Scoped to the confirmation's own value markup: the page transition keeps
    // the Welcome page mounted briefly, and its clock row would match too.
    await expect(page.locator('.cvalue', { hasText: 'Europe/Kyiv' })).toBeVisible();
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

    // An all-digit username is invalid under Linux's useradd rules and shows an
    // inline validation error.
    await page.getByLabel('Username').fill('123');
    await page.getByLabel('Username').blur();
    await expect(page.locator('.error.visible')).toContainText(/digits/i);

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

  test('Disk selection: a disk the system is using cannot be selected', async ({ page }) => {
    await page.goto('/');
    await next(page).click(); // -> User account
    await fillUserAccount(page);
    await next(page).click(); // -> Disk selection

    // /dev/sda is the fake machine's booted system (root, home and swap are
    // live): the row carries the danger chip, says why, and is not a button
    // anyone can press.
    const row = page
      .locator('.disk-card')
      .filter({ has: page.locator('.disk-name', { hasText: 'WD Blue SA516 1TB' }) });
    await expect(row).toBeVisible();
    await expect(row.locator('.chip--danger')).toHaveText('In use');
    await expect(row.locator('.disk-content')).toHaveText(
      'In use — the system is using this disk',
    );
    await expect(row).toBeDisabled();
    await row.click({ force: true });
    await expect(page.locator('.disk-card.selected')).toHaveCount(0);

    // A data disk the system merely mounted (/dev/sdh) is refused the same way.
    const mounted = page
      .locator('.disk-card')
      .filter({ has: page.locator('.disk-name', { hasText: 'Toshiba X300 8TB' }) });
    await expect(mounted).toBeDisabled();
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
    // Assert the summary's exact section headings, not loose text: the page
    // transition keeps the previous page mounted briefly, and substring text
    // (e.g. "User Account" ⊂ "Create user account") races the exiting page.
    await expect(page.getByRole('heading', { name: 'Language', exact: true })).toBeVisible();
    // The three Welcome answers each get their own row — the summary is where a
    // person catches one that came out wrong.
    await expect(page.getByRole('heading', { name: 'Location', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Time zone', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'User Account', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Target Disk', exact: true })).toBeVisible();
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

  test('Disk selection: leaving it backwards to the user account resets its answers', async ({ page }) => {
    await page.goto('/');
    await next(page).click(); // -> User account
    await fillUserAccount(page);
    await next(page).click(); // -> Disk selection

    await page.locator('.disk-card').first().click();
    await page.locator('.fs-pill').filter({ hasText: /^ext4$/ }).click();
    const lvm = page.getByRole('switch').nth(1);
    await lvm.click();
    await expect(lvm).toHaveAttribute('aria-checked', 'true');

    await back(page).click(); // -> User account (the answers belong to the disk visit)
    await expect(page.getByRole('heading', { name: 'Create user account' })).toBeVisible();

    await next(page).click(); // -> Disk selection again — a fresh visit
    await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();
    await expect(page.locator('.disk-card.selected')).toHaveCount(0);
    await expect(page.locator('.fs-pill.selected')).toContainText('btrfs'); // back to the default
    await expect(lvm).toHaveAttribute('aria-checked', 'false');
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
