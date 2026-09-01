import { defineConfig } from '@playwright/test';
import { fileURLToPath, URL } from 'url';

// E2E tests drive the real Wails app. The pages call Go backend bindings
// (GetDisks, GetLanguages, ...) through the Wails runtime, so tests must run
// against `wails dev` (app + backend served together), NOT plain `vite dev`.
//
// `wails dev` is auto-started here via the webServer option, using synthetic
// data (FAKE_DISKS) so the wizard has disks to select, and dry-run mode so the
// install steps finish quickly instead of pacstrapping a real system.
//
// Run from the frontend/ directory:
//   npm run e2e            # headless
//   npm run e2e:headed     # visible browser
//   npm run e2e:report     # open the HTML report

const devPort = 34115; // Wails v2 dev server default (frontend:dev:serverUrl: "auto")
const projectRoot = fileURLToPath(new URL('../', import.meta.url));

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI
    ? [['list'], ['github']]
    : [['list'], ['html', { open: 'never' }]],
  timeout: 30_000,
  expect: { timeout: 5_000 },
  use: {
    baseURL: `http://localhost:${devPort}`,
    headless: true,
    viewport: { width: 1280, height: 900 },
    trace: 'on-first-retry',
    actionTimeout: 10_000,
  },
  webServer: {
    // `wails dev` must run from the project root (where wails.json lives).
    command: `cd ${projectRoot} && env AELUM_DRY_RUN=1 FAKE_DISKS=1 wails dev -tags webkit2_41`,
    url: `http://localhost:${devPort}`,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
