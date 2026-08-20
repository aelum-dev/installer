# AestheticOS Installer


https://github.com/user-attachments/assets/ff3c87f9-c308-4d91-9b27-7b59683c8f89


A graphical installer for [AestheticOS](https://github.com/KornienkoDev/AestheticOS) — a fully offline Arch Linux ISO. Built with [Wails v2](https://wails.io/) (Go + Svelte 5).

Just boot the ISO, run the installer, and answer a few questions. No terminal, no internet, no fuss.

## What you get

- **6 step wizard** — pick a language, create your user, choose a disk, confirm, and watch it install.
- **ext4 or btrfs** — btrfs comes with a sensible automatic subvolume layout.
- **Beautiful desktop** — Hyprland + DankMaterialShell, greetd login, Alacritty terminal, Firefox, and PipeWire audio, all pre-configured.
- **Fast, quiet boot** — Plymouth boot splash

## Getting started

1. Clone this repo and the [AestheticOS](https://github.com/KornienkoDev/AestheticOS) live-ISO repo.
2. Build the installer:

   ```bash
   ./build.sh
   ```

3. Copy both binaries into the ISO's airootfs so they're included when the ISO is rebuilt:

   ```bash
   cp build/bin/aestheticos-installer build/bin/aestheticos-installer-cli ~/airootfs/usr/local/bin/
   ```

4. Build the live ISO from the AestheticOS repo (this bakes the installer into the airootfs). The resulting ISO is written to `output/`.
5. Flash the ISO to a USB drive (or [Ventoy](https://www.ventoy.io/)), boot from it, and let the installer launch automatically.
6. Follow the wizard: choose your language, create your account, select your disk, and confirm.

The installer runs entirely from the live ISO — it needs no internet connection.

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
