# Aelum Installer

The installer behind [Aelum](https://aelum.dev). It takes a machine from the live ISO to a finished Arch
desktop by asking four questions — language, user, disk, confirm — and it installs from packages the ISO
carries, so the install needs no internet. There's a GUI (Wails v2: Go + Svelte 5) and a headless CLI;
both run the same eight steps.

![The Aelum installer welcome screen](./images/installer-screenshot.png)

It ships inside the live ISO and opens on its own when that ISO boots.

## What it writes

- **A desktop, not a base system** — Hyprland with Dux Shell, the Dux Greeter login, Kitty, Firefox,
  PipeWire audio, zram instead of a swap file, and a branded rEFInd menu and Plymouth splash.
- **Whatever you chose on the disk screen** — ext4 or btrfs, LUKS2 encryption, LVM. Encryption always sits
  below LVM. A btrfs install gets snapper snapshots of `@` and `@home`, a pair around every `pacman`
  transaction, and a snapshot submenu under its own boot menu entry.
- **Boot entries by UUID**, never `/dev/mapper` paths, with each install's files in its own directory on
  the ESP — several Aelum installs can share one disk without overwriting each other.
- **28 languages.** The one you pick sets the installer's UI and your system's locale, and gives the
  installed system a mirror list for that region for future updates; the install itself never touches the
  network. Not every language is fully translated inside Dux Shell yet — the desktop language can be
  changed later in its settings.

## Automatic or manual disk mode

Automatic mode is what most people want: pick a disk, pick a filesystem, confirm. **It erases the whole
disk** and writes its own Boot + System pair.

Manual mode formats only the two partitions you designate — Boot (FAT32, at least 512 MiB) and System —
and leaves every other partition exactly as it was found, so another operating system and its files
survive. Give Aelum its own Boot partition instead of reusing another system's ESP. It needs a GPT table
(or a disk with no table at all); an MBR disk is refused before anything on it is written — use automatic
mode there, since it clears the table and writes a GPT.

If this disk holds nothing you need to keep, stay in automatic mode.

## Headless installs

The CLI runs the same steps, so an install can be scripted:

```bash
aelum-installer-cli --install /dev/vda                        # btrfs by default; --filesystem ext4 for ext4
aelum-installer-cli --install /dev/vda --luks --lvm
aelum-installer-cli --install /dev/vda --manual-layout '<json>'
```

Passwords come from stdin only, never from a flag. `--dry-run` walks the steps without touching a disk.

## Building it

```bash
./build.sh        # → build/bin/aelum-installer and build/bin/aelum-installer-cli
```

Toolchain, tests and code conventions are in [CONTRIBUTING.md](./CONTRIBUTING.md). You don't build it to
use it: [aelum/docker](https://github.com/aelum-dev/docker) builds the installer as part of building the
ISO.

## Docs

| File | What's in it |
|---|---|
| [AGENTS.md](./AGENTS.md) | architecture map, install steps, key patterns, rules |
| [OVERVIEW.md](./OVERVIEW.md) · [EXECUTION_FLOW.md](./EXECUTION_FLOW.md) | feature overview · step-by-step execution flow |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | building, tests, style, commit messages |
| [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) | install and boot failures, dual boot, the boot menu |
| [SECURITY.md](./SECURITY.md) | how to report a vulnerability |

## License

GPL-3.0 — see [LICENSE](./LICENSE).
