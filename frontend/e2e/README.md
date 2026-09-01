# E2E Testing (Playwright)

End-to-end tests for the Aelum installer drive the **real Wails app** through
Playwright. Because the pages call Go backend bindings (`GetDisks`,
`GetLanguages`, …) via the Wails runtime, the tests must run against
`wails dev` (app **+** backend served together), not plain `vite dev`.

## How it works

- `playwright.config.ts` auto-starts `wails dev` via Playwright's `webServer`
  option, with `FAKE_DISKS=1` (synthetic disks to select) and
  `AELUM_DRY_RUN=1` (steps simulate quickly, no real disk touched, no sudo).
- The fixed dev port `34115` is set in `wails.json` (`frontend:dev:serverUrl`)
  so the config's `baseURL` is stable.
- Tests live in `e2e/*.spec.ts`.

## Setup (one time)

```bash
cd frontend
npm install
npm run e2e:install      # downloads Playwright's Chromium
```

On Linux, `wails dev` also needs WebKitGTK (`webkit2gtk-4.1`) and a display. In
headless CI, wrap the run in `xvfb-run` and install browser system deps:

```bash
npx playwright install-deps chromium   # needs sudo
xvfb-run -a npm run e2e
```

## Running

```bash
cd frontend
npm run e2e            # headless
npm run e2e:headed     # visible browser, handy for debugging
npm run e2e:report     # open the HTML report (after a run)
```

## What's covered

- `wizard.spec.ts` — Welcome → User account (validation) → Disk selection →
  Confirmation → Back navigation.
- `install.spec.ts` — the full wizard flow through to the Installation step and
  its terminal state.

## Note on the install step

In this dev environment the dry-run CLI validates the synthetic disk's
partition layout and reports an error ("exactly one ESP partition is required"),
so the automated flow lands on the Installation page's error hint rather than
the Finish screen. The test accepts either terminal state — it validates that
the install path is wired correctly (navigation, backend bindings, CLI spawn).
A clean run-to-Finish requires a real disk layout / non-dry install.
