import { expect, test, type Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

// Website screenshots of the installer, shot from the real app.
//
// These are the sources for the website's installer walkthrough
// (website/src/components/Install.astro → keys installer-step-1…7, both themes).
// They are captured here, and only here, because this harness is the one place
// the wizard runs end to end with real backend data (fake disks, dry-run) and a
// browser that can be asked for any pixel density:
//
//   - the viewport is the shipped window size (900x600, fixed in main.go), so the
//     site shows what a person actually sees — the captures it replaced were
//     1200x800, from a window the installer no longer opens;
//   - deviceScaleFactor rasterises at 3x, so the 1280px derivative the site
//     serves to a HiDPI screen is a downscale of real pixels;
//   - every frame is scripted, so the seven frames are one run: the same
//     language, the same user, the same disk from the first slide to the last.
//
// What it writes (AELUM_SHOT_DIR, default <tmpdir>/aelum-installer-shots):
//
//   installer-step-1-<theme>.png   Welcome — language list + location card
//   installer-step-2-<theme>.png   Create user account
//   installer-step-3-<theme>.png   Select disk — automatic mode
//   installer-step-4-<theme>.png   the same screen in manual mode
//   installer-step-5-<theme>.png   Installation summary
//   installer-step-6-<theme>.png   Installing the desktop environment
//   installer-step-7-<theme>.png   Installation complete
//
// Run it, then hand the frames to the website's pipeline:
//
//   cd installer/frontend && npm run shots          # dark + light, one run each
//   cd ../../website && SHOTS=/tmp/opencode/aelum-installer-shots
//   args=(); for n in 1 2 3 4 5 6 7; do for t in light dark; do
//     args+=(--single "installer-step-$n-$t=$SHOTS/installer-step-$n-$t.png"); done; done
//   python3 scripts/prepare-sources.py "${args[@]}" && python3 scripts/build-images.py
//
// The frames are already the whole window, so they enter the pipeline through
// prepare-sources.py --single (nothing to crop) instead of the window-crop mode a
// capture off a real desktop needs.
//
// The DRY RUN badge this build would show is hidden for every frame: it marks a
// dev-mode session, and the installer a visitor runs never shows it.

/** The disk the walkthrough runs on: one of the clean fake disks. */
const DISK_MODEL = 'Samsung 990 PRO 2TB';

/** The account the walkthrough creates. */
const USERNAME = 'alice';

const OUT_DIR = process.env.AELUM_SHOT_DIR ?? join(tmpdir(), 'aelum-installer-shots');

/** The page transition is 140ms delay + 200ms fade; the footer's dots move with it. */
const SETTLE_MS = 700;

type Theme = 'light' | 'dark';

const next = (page: Page) => page.getByRole('button', { name: 'Next' });

// Capturing is opt-in: `npm run e2e` runs the suite, `npm run shots` takes the
// website's frames. Both attach to the same warm `wails dev`.
test.skip(!process.env.AELUM_SHOTS, 'opt-in capture: npm run shots');

test.describe.configure({ mode: 'serial' });

// The shipped window (main.go fixes the app at 900x600), rasterised at 3x.
test.use({
  viewport: { width: 900, height: 600 },
  deviceScaleFactor: Number(process.env.AELUM_SHOT_SCALE ?? 3),
});

/**
 * Take the DRY RUN badge out of the picture.
 *
 * It marks a dev-mode session (`GetDryRun`, which the app answers yes to whenever
 * it is not root), and the installer a visitor runs never shows it.
 *
 * It is injected into the loaded document rather than through an init script:
 * against the Wails dev server an init script never runs (measured — the style
 * element is simply absent after load), and every frame here is taken from this
 * one document, because the wizard changes pages without navigating.
 */
async function hideDryRunBadge(page: Page): Promise<void> {
  await page.evaluate(() => {
    if (document.getElementById('aelum-shots')) return;
    const style = document.createElement('style');
    style.id = 'aelum-shots';
    style.textContent = '.dry-run-badge { display: none !important; }';
    document.head.append(style);
  });
}

/**
 * Put the app in `theme` with its own header control.
 *
 * The toggle is the real mechanism (it writes <html data-theme> and persists the
 * choice), so a frame can never show a scheme the wizard itself cannot reach —
 * and clicking only when the schemes disagree makes a rerun start from whatever
 * theme the last one left behind.
 */
async function chooseTheme(page: Page, theme: Theme): Promise<void> {
  if ((await page.getAttribute('html', 'data-theme')) !== theme) {
    await page.locator('.theme-toggle').click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
  }
  await hideDryRunBadge(page);
}

/**
 * Frame `n` of the walkthrough, in `theme`.
 *
 * The pointer is parked in the header's corner so no control is caught mid-hover
 * or mid-tooltip, and the fonts are awaited so no frame is taken between the
 * fallback face and the real one.
 */
async function shoot(page: Page, theme: Theme, n: number): Promise<void> {
  await page.mouse.move(1, 1);
  await page.evaluate(() => document.fonts.ready);
  await hideDryRunBadge(page);
  const file = join(OUT_DIR, `installer-step-${n}-${theme}.png`);
  await page.screenshot({ path: file });
  console.log(`  ${file}`);
}

/** One theme, one run of the wizard: the seven frames of the walkthrough. */
async function walkthrough(page: Page, theme: Theme): Promise<void> {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Welcome to Aelum' })).toBeVisible();
  await chooseTheme(page, theme);

  // English pins the UI language: the backend remembers whatever the last session
  // chose, and the site's frames are the English wizard. Selected by language code
  // — the row's name now carries its territory and its locale beside it.
  await page.locator('.option-item[data-code="en_US"]').click();
  await page.waitForTimeout(SETTLE_MS);
  await shoot(page, theme, 1);

  // 2 — User account. Filled, then blurred: the frame shows answers, not a caret.
  await next(page).click();
  await expect(page.getByRole('heading', { name: 'Create user account' })).toBeVisible();
  await page.getByLabel('Username').fill(USERNAME);
  await page.getByLabel('Password', { exact: true }).fill('ae1um-demo');
  await page.getByLabel('Confirm Password').fill('ae1um-demo');
  await page.getByLabel('Confirm Password').blur();
  await page.waitForTimeout(SETTLE_MS);
  await shoot(page, theme, 2);

  // 3 — Disk selection in automatic mode, the path most people stay on.
  await next(page).click();
  await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();
  const disk = page
    .locator('.disk-card')
    .filter({ has: page.locator('.disk-name').getByText(DISK_MODEL, { exact: true }) })
    .first();
  await disk.click();
  await expect(disk).toHaveClass(/selected/);
  await page.waitForTimeout(SETTLE_MS);
  await shoot(page, theme, 3);

  // 4 — The same screen in manual mode, with the two roles it asks for chosen.
  await page.getByRole('button', { name: 'Manual' }).click();
  await expect(page.getByRole('heading', { name: 'Select Partitions' })).toBeVisible();
  const rows = page.locator('.part-row');
  await rows.first().getByRole('button', { name: 'Boot' }).click();
  await rows.nth(1).getByRole('button', { name: 'System' }).click();
  await page.waitForTimeout(SETTLE_MS);
  await shoot(page, theme, 4);

  // Back to automatic for the rest of the run: the summary and the progress screen
  // the site shows are the whole-disk path they describe.
  await page.getByRole('button', { name: 'Automatic' }).click();
  await expect(page.getByRole('heading', { name: 'Select Installation Disk' })).toBeVisible();

  // 5 — Summary: the four answers on one screen.
  await next(page).click();
  await expect(page.getByRole('heading', { name: 'Installation Summary' })).toBeVisible();
  await page.waitForTimeout(SETTLE_MS);
  await shoot(page, theme, 5);

  // 6 — The install running. The longest step is the desktop environment, and that
  // is the label worth showing; it is caught as it appears, before the bar leaves it.
  await next(page).click();
  await expect(page.getByRole('heading', { name: 'Confirm Installation' })).toBeVisible();
  await page.getByRole('button', { name: 'Confirm' }).click();
  await expect(page.getByRole('heading', { name: 'Installing Aelum' })).toBeVisible();
  await expect(page.getByText('Installing desktop environment...')).toBeVisible({ timeout: 90_000 });
  await shoot(page, theme, 6);

  // 7 — Done. The page advances itself 1s after the install settles.
  await expect(page.getByText('Aelum has been installed successfully.')).toBeVisible({
    timeout: 120_000,
  });
  await page.waitForTimeout(SETTLE_MS);
  await shoot(page, theme, 7);
}

for (const theme of ['dark', 'light'] as const) {
  test(`walkthrough in the ${theme} scheme`, async ({ page }) => {
    test.setTimeout(300_000);
    mkdirSync(OUT_DIR, { recursive: true });
    await walkthrough(page, theme);
  });
}
