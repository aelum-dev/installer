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

## Website screenshots

```bash
cd frontend
npm run shots          # the walkthrough, dark + light, into $TMPDIR/aelum-installer-shots
AELUM_SHOT_DIR=… npm run shots     # somewhere else
AELUM_SHOT_SCALE=2 npm run shots   # a different raster (default 3x)
```

`screenshots.spec.ts` is the source of the website's installer walkthrough
(`website/src/components/Install.astro` → the `installer-step-N-theme` keys). It is
skipped by `npm run e2e` and runs only when `AELUM_SHOTS=1` is set, which
`npm run shots` does.

It drives the same real app for a reason: the frames have to be the installer a
visitor meets, and only this harness has the backend under them.

- The viewport is the shipped window (900×600, fixed in `main.go`) and
  `deviceScaleFactor: 3`, so the site's 1280 px derivative is a downscale of real
  pixels rather than an upscale of a 1× grab off a display.
- One run per theme, so all seven frames of a theme share one session: same
  language, same user, same disk (`Samsung 990 PRO 2TB` — one of the clean rows of
  the fake disk table, so no row in the disk list reads as mounted).
- The theme is switched with the app's own header control; the DRY RUN badge —
  which marks this dev-mode session, not a screen the installer has — is hidden by
  a stylesheet injected into the loaded document. An `addInitScript` does **not**
  work against the Wails dev page (measured: it never runs), so anything the
  capture needs has to be injected after load. Nothing navigates after that, so it
  holds for every frame.

Then, from `website/`: `scripts/prepare-sources.py --single` for each frame (they
are already the whole window, so there is nothing to crop) and
`scripts/build-images.py`. The full command is in `website/AGENTS.md` → Assets →
Screenshots.

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
- `screenshots.spec.ts` — the website's walkthrough frames. Not a test of
  behaviour: it asserts only that each screen is the one it is about to capture,
  and it is skipped unless `AELUM_SHOTS=1`. See **Website screenshots** above.

## Note on validation

The **Next button is always enabled**; invalid input no longer blocks navigation.
Clicking Next with invalid input instead shows a "Cannot proceed" validation
modal (an OK-only `ConfirmDialog`). Tests assert this modal appears on bad input
and that the wizard advances once the input is valid. In manual mode, the Manual
mode pill stays disabled until a disk is selected. The add control only exists
after clicking the FREE row, and a fully used disk has no FREE row at all — those
are the states the tests assert.

## Note on the install step

The dry-run flow runs to the end: the GUI spawns the CLI without sudo, the
synthetic disk the suite selects has a layout the CLI accepts, and
`install.spec.ts` waits for the Finish screen's "Aelum has been installed
successfully." — about ten seconds. The wizard's success path is therefore
covered here, including the progress events and the finish state
`screenshots.spec.ts` captures. A real install against a real disk is tested in
QEMU from `docker/`, never here.
