import { defineConfig } from '@playwright/test';
import { fileURLToPath, URL } from 'url';

// E2E tests drive the real Wails app. The pages call Go backend bindings
// (GetDisks, GetLanguages, ...) through the Wails runtime, so tests must run
// against `wails dev` (app + backend served together), NOT plain `vite dev`.
//
// The normal entry point is `npm run e2e` -> scripts/e2e.sh: it keeps a warm
// `wails dev` on :34115 across runs (the cold build is ~4 min; the suite is
// well under 1), and with `reuseExistingServer` this config simply attaches to
// it. The webServer block below is the fallback for running
// `npx playwright test` directly (or on CI).
//
// Run from the frontend/ directory:
//   npm run e2e            # headless (server stays warm; npm run e2e:stop ends it)
//   npm run e2e:headed     # visible browser
//   npm run e2e:report     # open the HTML report

const devPort = 34115; // Wails v2 dev server default (frontend:dev:serverUrl: "auto")
const projectRoot = fileURLToPath(new URL('../', import.meta.url));

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  // One worker: the wails backend is a single app session (a.diskSelection,
  // a.manualMode, installationRunning), and every browser page drives that same
  // session. With several workers, a manual-mode test can flip a.manualMode
  // while the install spec's StartInstallation is serializing its layout —
  // the CLI then gets a foreign/empty --manual-layout and dies at exit 1.
  workers: 1,
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
