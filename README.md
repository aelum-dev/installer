# AestheticOS Installer

![Demo Video](media/installer-demo.mp4)

A graphical installer for [AestheticOS](https://gitlab.com/aestheticos/AestheticOS), an Arch Linux distro. The install works fully offline — no network needed to install it. Built with [Wails v2](https://wails.io/) (Go + Svelte 5).

Just boot the ISO and run the installer — a few clicks in the GUI, no terminal or network needed to install.

## How this fits into AestheticOS

This repo is the **installer** half of the AestheticOS project. The [AestheticOS](https://gitlab.com/aestheticos/AestheticOS) repo builds the live ISO; that ISO is intentionally barebones — its only job on boot is to launch *this* installer.

In short: **AestheticOS = the ISO**, **this repo = the software that installs AestheticOS onto your disk.** Everything the installer needs is served from the ISO's embedded offline mirror, so the install works with no internet. (Once installed, it's a normal Arch system.)

## What you get

- **6 step wizard** — pick a language, create your user, choose a disk, confirm, and watch it install.
- **ext4 or btrfs** — btrfs comes with a sensible automatic subvolume layout.
- **Configured desktop** — Hyprland + DankMaterialShell, greetd login, Alacritty terminal, Firefox, and PipeWire audio, all set up for you.
- **Fast, quiet boot** — Plymouth boot splash

## Language selection

The first thing you'll pick is your language — there are 28 to choose from. This sets the installer's own UI language, and it also chooses your system locale and the mirror country. The mirror country just configures your installed system's pacman mirror list for future online updates — the install itself pulls packages from the ISO's embedded offline repo, not from that mirror. Note that not all 28 languages are fully translated in the DMS desktop yet — you can always change the desktop language later in DMS settings.

## Getting started

1. Clone this repo and the [AestheticOS](https://gitlab.com/aestheticos/AestheticOS) live-ISO repo (side by side, e.g. `~/Projects/AestheticOS-Installer` and `~/Projects/AestheticOS`).
2. Build the installer (outputs `build/bin/aestheticos-installer` and `build/bin/aestheticos-installer-cli`):

   ```bash
   ./build.sh
   ```

3. Copy both binaries into the AestheticOS repo's airootfs so they're included when the ISO is rebuilt:

   ```bash
   cp build/bin/aestheticos-installer build/bin/aestheticos-installer-cli ~/Projects/AestheticOS/airootfs/usr/local/bin/
   ```

   `build.sh` can also do this for you with `BIN_DIR`, e.g.:

   ```bash
   BIN_DIR=~/Projects/AestheticOS/airootfs/usr/local/bin ./build.sh
   ```

4. Build the live ISO from the AestheticOS repo (this bakes the installer into the airootfs). The resulting ISO is written to `output/`.
5. Flash the ISO to a USB drive (or [Ventoy](https://www.ventoy.io/)), boot from it, and let the installer launch automatically.
6. Follow the wizard: choose your language, create your account, select your disk, and confirm.

The install runs entirely from the live ISO and needs no internet. (After install, your system is a normal Arch machine and connects to the internet like any other.)

## Running from the command line

Prefer headless or scripted installs? The installer also ships a CLI:

```bash
# GUI
sudo ./build/bin/aestheticos-installer

# CLI — full install (you'll be prompted for your password)
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda

# CLI — btrfs root instead of ext4
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda --filesystem btrfs
```

## Want to build or contribute?

Head over to [AGENTS.md](./AGENTS.md) for architecture details and conventions,
[CONTRIBUTING.md](./CONTRIBUTING.md) for how to contribute, and
[TROUBLESHOOTING.md](./TROUBLESHOOTING.md) if something goes wrong.

---

GPL-3.0 — see [LICENSE](./LICENSE).
