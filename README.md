# Aelum Installer

A graphical installer for [Aelum](https://gitlab.com/aelum/Aelum), an Arch Linux distro. The install works fully offline — no network needed to install it. Built with [Wails v2](https://wails.io/) (Go + Svelte 5).

Just boot the ISO and run the installer — a few clicks in the GUI, no terminal or network needed to install.

## How this fits into Aelum

This repo is the **installer** half of the Aelum project. The [Aelum](https://gitlab.com/aelum/Aelum) repo builds the live ISO; that ISO is intentionally barebones — its only job on boot is to launch *this* installer.

In short: **Aelum = the ISO**, **this repo = the software that installs Aelum onto your disk.** Everything the installer needs is served from the ISO's embedded offline mirror, so the install works with no internet. (Once installed, it's a normal Arch system.)

## What you get

- **6 step wizard** — pick a language, create your user, choose a disk, confirm, and watch it install.
- **ext4 or btrfs** — btrfs comes with a sensible automatic subvolume layout.
- **Configured desktop** — Hyprland + DankMaterialShell, greetd login, Alacritty terminal, Firefox, and PipeWire audio, all set up for you.
- **Fast, quiet boot** — Plymouth boot splash

## Language selection

The first thing you'll pick is your language — there are 28 to choose from. This sets the installer's own UI language, and it also chooses your system locale and the mirror country. The mirror country just configures your installed system's pacman mirror list for future online updates — the install itself pulls packages from the ISO's embedded offline repo, not from that mirror. Note that not all 28 languages are fully translated in the DMS desktop yet — you can always change the desktop language later in DMS settings.

## Getting started

1. Clone this repo and the [Aelum](https://gitlab.com/aelum/Aelum) live-ISO repo (side by side, e.g. `~/Projects/Aelum-Installer` and `~/Projects/Aelum`).
2. Build the installer (outputs `build/bin/aelum-installer` and `build/bin/aelum-installer-cli`):

   ```bash
   ./build.sh
   ```

3. Copy both binaries into the Aelum repo's airootfs so they're included when the ISO is rebuilt:

   ```bash
   cp build/bin/aelum-installer build/bin/aelum-installer-cli ~/Projects/Aelum/airootfs/usr/local/bin/
   ```

   `build.sh` can also do this for you with `BIN_DIR`, e.g.:

   ```bash
   BIN_DIR=~/Projects/Aelum/airootfs/usr/local/bin ./build.sh
   ```

4. Build the live ISO from the Aelum repo (this bakes the installer into the airootfs). The resulting ISO is written to `output/`.
5. Flash the ISO to a USB drive (or [Ventoy](https://www.ventoy.io/)), boot from it, and let the installer launch automatically.
6. Follow the wizard: choose your language, create your account, select your disk, and confirm.

The install runs entirely from the live ISO and needs no internet. (After install, your system is a normal Arch machine and connects to the internet like any other.)

## Running from the command line

Prefer headless or scripted installs? The installer also ships a CLI:

```bash
# GUI
sudo ./build/bin/aelum-installer

# CLI — full install (you'll be prompted for your password)
sudo ./build/bin/aelum-installer-cli --install /dev/vda

# CLI — btrfs root instead of ext4
sudo ./build/bin/aelum-installer-cli --install /dev/vda --filesystem btrfs
```

## Testing

The frontend has Playwright **end-to-end tests** that drive the real Wails app (Go backend + Svelte UI together) — not a mock. They run the wizard, manual partitioning, and the full install flow against `wails dev` in dry-run mode with synthetic disks, so no real disk is touched.

```bash
cd frontend
npm install
npm run e2e:install      # one-time: downloads Playwright's Chromium
npm run e2e              # run the suite (headless)
npm run e2e:headed       # visible browser, handy for debugging
npm run e2e:report       # open the HTML report after a run
```

On headless CI, wrap the run in `xvfb-run` and install the browser system deps (`npx playwright install-deps chromium` needs sudo). The [`playwright.config.ts`](./frontend/playwright.config.ts) auto-starts `wails dev` for you, so you don't need to launch it manually. See [frontend/e2e/README.md](./frontend/e2e/README.md) for the full details (what's covered, dry-run env vars, CI setup).

## Want to build or contribute?

Head over to [AGENTS.md](./AGENTS.md) for architecture details and conventions,
[CONTRIBUTING.md](./CONTRIBUTING.md) for how to contribute, and
[TROUBLESHOOTING.md](./TROUBLESHOOTING.md) if something goes wrong.

---

GPL-3.0 — see [LICENSE](./LICENSE).
