# E2E Testing (Playwright)

End-to-end tests for the Aelum installer drive the **real Wails app** through
Playwright. Because the pages call Go backend bindings (`GetDisks`,
`GetLanguages`, …) via the Wails runtime, the tests must run against
`wails dev` (app **+** backend served together), not plain `vite dev`.

## How it works

- `npm run e2e` runs `scripts/e2e.sh`: it keeps a **warm `wails dev`** on
  port `34115` across runs. The first run starts it with `FAKE_DISKS=1`
  (synthetic disks to select) and `AELUM_DRY_RUN=1` (steps simulate quickly, no
  real disk touched, no sudo) and pays the cold build (~4 min); every later run
  attaches to the running server and the suite itself takes well under a
  minute. `npm run e2e:stop` stops it. On CI the script skips the management
  and lets Playwright start its own `webServer`.
- `playwright.config.ts` keeps a `webServer` fallback so a bare
  `npx playwright test` still works (`reuseExistingServer` outside CI).
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
npm run e2e            # headless; the dev server stays warm for the next run
npm run e2e:stop       # stop the warm dev server
npm run e2e:headed     # visible browser, handy for debugging
npm run e2e:report     # open the HTML report (after a run)
```

Note: a warm server left by the script runs with `FAKE_DISKS=1` + dry-run. If
you want to click through the app by hand, that's the same environment the
tests use — stop it with `npm run e2e:stop` first and start a normal
`wails dev` if you need a non-dry one.

## If Playwright appears to hang at startup

`wails dev` opens the native installer window and binds two ports: **34115**
(the Wails backend the tests call) and **5173** (Vite). If a previous run was
interrupted, a stale Vite or `wails dev` process can keep those ports busy, so
the next run opens the installer window but never becomes reachable and
Playwright waits for the webServer URL.

Check for leftovers and stop them before retrying:

```bash
pgrep -af "wails dev|node_modules/.bin/vite|aelum-installer-dev"
```

(`scripts/e2e.sh` starts the server in its own process group, so
`npm run e2e:stop` reaps the whole tree including Vite.)

Note: if you change Go bindings while a warm server runs, wails regenerates
`wailsjs/` but Vite can keep serving the **stale** generated module — every test
then fails on a blank page (`... does not provide an export named ...`).
`npm run e2e:stop && npm run e2e` fixes it; the rebuild is only a few seconds
with warm Go caches.

## What's covered

- `wizard.spec.ts` — Welcome → User account (validation) → Disk selection →
  Confirmation → Back navigation.
- `manual.spec.ts` — manual partition mode: the role menu, adding/
  deleting rows, and the delete-confirmation dialog.
- `install.spec.ts` — the full wizard flow through to the Installation step and
  its terminal state.

## Note on validation

The **Next button is always enabled**; invalid input no longer blocks navigation.
Clicking Next with invalid input instead shows a "Cannot proceed" validation
modal (an OK-only `ConfirmDialog`). Tests assert this modal appears on bad input
and that the wizard advances once the input is valid. In manual mode, the Manual
mode pill stays disabled until a disk is selected. The add control only exists
after clicking the FREE row, and a fully used disk has no FREE row at all — those
are the states the tests assert.

## Note on the install step

In this dev environment the dry-run CLI validates the synthetic disk's
partition layout and reports an error ("exactly one Boot partition is required"),
so the automated flow lands on the Installation page's error hint rather than
the Finish screen. The test accepts either terminal state — it validates that
the install path is wired correctly (navigation, backend bindings, CLI spawn).
A clean run-to-Finish requires a real disk layout / non-dry install.
