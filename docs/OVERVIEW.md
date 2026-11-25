# Aelum Installer — Project Overview

Graphical Arch Linux installer written with [Wails v2](https://wails.io/) (Go backend +
Svelte 5 frontend), plus a headless CLI (`aelum-installer-cli`). Both run the same eight
installation steps; the GUI spawns the CLI via `sudo` for all privileged operations.

**Offline installer:** every package comes from the mirror embedded in the live ISO — the
install never touches the network. The mirrorlist it writes to the target is only for
future online updates.

`AGENTS.md` (repo root) is the authoritative agent context: architecture map, step weights,
key patterns, rules. This file is the human-facing companion.

---

## Live ISO vs Installed System

**Never confuse these two. It is the #1 source of bugs.**

| Term | What | Key facts |
|------|------|-----------|
| **Live ISO** | Bootable Arch running the installer (read-only overlay rootfs). | The `aelum` user has passwordless sudo for the CLI binary. |
| **Installed System** | The Arch being written to the target disk. | Mounted at `/mnt/`, ESP at `/mnt/efi`, this install's `aelum/<id>/` boot dir bind-mounted at `/mnt/boot`; touched via `arch-chroot /mnt`. Writes `/etc/sudoers.d/wheel` for the new user. |
| **Host** | Developer machine. | `build.sh` and git live here; `../shared` is the host side of the VM's `/shared`. |
| **VM** | QEMU guest booting the ISO (the only real install target in this project). | `/shared` 9p-mounted; see `docker/` in the distro workspace. |

**Code rule:** `/mnt/*` or `arch-chroot` → installed system. `/dev/*` or direct exec → live ISO.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| GUI | Wails v2 (Go + WebKitGTK 4.1) |
| Backend | Go 1.26 |
| Frontend | Svelte 5 (TypeScript), Vite |
| Target OS | Arch Linux, installed via `pacstrap` |
| Desktop | Hyprland + Dux Shell (Quickshell-based) + Dux Greeter (greetd) |
| Bootloader | rEFInd (auto-detects other OSes) |
| Filesystems | ext4 or btrfs (subvolumes + snapshots) |
| Encryption | LUKS2 (optional, always below LVM), optional LVM |
| Boot splash | Plymouth, branded Aelum theme |

---

## Installation Steps (8, order owned by `install/step.go`)

| # | Step | Weight | What it does |
|---|------|--------|--------------|
| 1 | Format Disk | 3 | GPT (EFI 512 MiB + root), format, optional LUKS2 → LVM → mkfs stack, btrfs subvolumes and the initial snapshot (`install/format/`, `install/partition/`) |
| 2 | Mount Filesystem | 2 | Activate the VG when LVM is on, mount root + subvolumes to `/mnt`, ESP to `/mnt/efi`, bind this install's `aelum/<id>/` ESP dir at `/mnt/boot` (`install/mount/`) |
| 3 | Install Base | 37 | `pacstrap` base + kernel + firmware, phase-banded progress (`install/installbase/`) |
| 4 | Install Desktop | 50 | `pacstrap` Hyprland, Dux Greeter, foot, firefox, pipewire, the fcitx5 core; GPU set from the PCI bus, IM engines from the language (`install/installdesktop/`) |
| 5 | Setup Users | 2 | Copy what the live ISO ships (`install/isocopy/`), write the weather location into the skeleton (`install/users/weather.go`), then useradd, chpasswd, sudoers (`install/users/`) |
| 6 | Configure System | 4 | Merged step, see sub-steps below (`install/configure/`) |
| 7 | Clear Cache | 1 | `pacman -Scc` in the chroot (`install/clearcache/`) |
| 8 | Verify System | 1 | Read-only post-install checks: fstab, kernel + initrd, rEFInd entry, packages, firewall enabled, IM profile, weather location, user, hostname (`install/verify/`) |

Weights total 100 and drive the progress bar (`shared.Weight*` in `backend/shared/config.go`).

**Step 6 sub-steps** (`configure.Run`, in order — all before the snapshot, because a
snapshot only contains what was written before it was taken):

1. Generate fstab (ESP at `/efi`, per-install `/boot` bind; no `subvol=` for the btrfs root)
2. System config: hostname, clock (the chosen zone), locale (guarded against a glibc the target cannot generate), keymap, NTP, mirrorlist
3. Input method list for the session (`/etc/xdg/fcitx5/profile`, from language + location)
4. initramfs HOOKS (plymouth always, `sd-encrypt` with LUKS, `lvm2` with LVM) + regenerate
5. Bootloader: rEFInd install, branded theme, per-install `aelum/<id>.conf`, NVRAM entry
6. greetd + Dux Greeter
7. Plymouth theme + boot-progress service
8. zram swap
9. Link `/etc/resolv.conf` → `/run/NetworkManager/resolv.conf` (so a snapshot boot resolves)
10. Enable services (NetworkManager, greetd, firewalld; bluetooth is optional)
11. Enable the `[aelum]` repo + `[multilib]`, trust the distro key
12. Snapper, **btrfs only, last**: configs for `@`/`@home`, pacman transaction hooks, rEFInd
    snapshot menu, snapshot-boot state generator, then the "after installation" snapshot

### Progress tracking

The two pacstrap steps together are 87 of the 100 weight points. `install/progress/pacstrap.go`
parses pacstrap output lines into download/install/hook bands; a tick goroutine nudges the
percentage during the other steps.

---

## Frontend (Wizard Flow)

```
Welcome (language + location + time zone — one summary card, one searchable list)
  → User Account (username + password)
    → Disk Selection (disk list + storage options in the info rail)
      [→ Manual Partition (only in manual mode)]
        → Confirmation (summary + root check + confirm dialog)
          → Installation (progress bar + live events)
            → Finish (success)
```

| Path | Purpose |
|------|---------|
| `frontend/src/App.svelte` | Root component, page router |
| `frontend/src/pages/*.svelte` | The wizard pages |
| `frontend/src/lib/store.svelte.ts` | Svelte 5 stores (`installState`, `navState`, LUKS actions) |
| `frontend/src/lib/navigation.ts` | Navigation state |
| `frontend/src/lib/validate.ts` | Single user-account rule engine (shared by UI and `page-validation.ts`) |
| `frontend/src/lib/constants.ts` | **Generated** from Go by `backend/scripts/gen-ts-constants.go` |
| `frontend/src/lib/welcome-cascade.ts` | Pure Welcome-page logic: the language→location→clock cascade, the three filters, the country-only clock list |
| `frontend/src/lib/i18n/` | 28-locale translation runtime (`t()`, English fallback, RTL for ar/he/fa) |
| `frontend/wailsjs/` | Generated Wails bindings (regenerate after adding Go methods) |

---

## Key Design Decisions

### Three answers, each over the complete set
The Welcome page takes language, location and time zone as three separate questions, and each
list is the whole set rather than a curated subset: every locale glibc can generate, every
ISO 3166-1 country, every civil clock tzdata has kept since 1970. A curated list is the
installer deciding the answer for someone before they have been asked, and each widening
removes a case where the right answer was simply absent:

- **Language** — a person should be able to install in the language their machine reads. The
  UI has 28 translations; a language with none sets the system locale and keeps English
  chrome, which is a working system.
- **Location** — where a machine is is a fact about the machine. A country Arch has no mirror
  section for still installs (the worldwide list is the fallback), so mirror availability is a
  flag shown on the row, not a gate on the list.
- **Time zone** — a country is not a clock. Someone in a multi-zone country, or one who simply
  wants a named city, picks the zone directly instead of being held to whatever their country
  defaults to.

The three are linked by a cascade, not merged: the language pre-fills its country and that
country's clock, the location sets the clock, and a hand-picked answer outranks the suggestion
below it. `frontend/src/lib/welcome-cascade.ts` owns those rules and is unit-tested without a
browser; the CLI mirrors them through `--language` / `--region` / `--timezone`.

### CLI spawning
The GUI (running as a normal user) spawns `sudo aelum-installer-cli`, writes the password to
its stdin, and reads JSON progress events from its stderr. No separate helper binary, no
command whitelist. **Passwords are stdin-only — never a flag** (avoids `/proc`, `ps`, shell history).

### Primitives seams for testability
Every install-step exec and file operation goes through `install/primitives/`
(`Run`/`RunChroot` behind `CommandRunner`; `WriteFile`/`ReadFile`/`Stat`/… behind `FileSystem`).
Tests inject `install/primitives/primitivestest/` — a scripted command recorder and an
in-memory filesystem — so steps test without root or `/mnt`. (Streaming pacstraps and the GUI's
process spawn use `os/exec` directly and are not behind the seam.)

### Dry run / fake disks / start page
`AELUM_DRY_RUN=1` simulates every step (auto-on when not root); `FAKE_DISKS=1` serves synthetic
disks; `AELUM_START_PAGE=N` opens the wizard on page N for debugging.

### btrfs layout (btrfs installs only)
Six subvolumes: `@`, `@home`, `@srv`, `@cache` (nodatacow), `@tmp` (nodatacow), `@log`.
**The system is installed into `@/.snapshots/1/snapshot`, which is the filesystem's default
subvolume** — `@` itself is never mounted or booted. Nothing pins `/` with `subvol=` or
`rootflags=`: fstab and the boot entry name no subvolume, so the boot follows the default
subvolume pointer — and that pointer is exactly what `snapper rollback` moves.

Snapshots: snapper configs for `@` and `@home` (own qgroups, every limit a range), a
pre/post snapshot pair around every `pacman` transaction (with `important=yes` marking when a
kernel/glibc/systemd-class package changed), rEFInd shows the history as a scrollable submenu
under one entry per install, and booting a snapshot reaches the greeter (a systemd generator
tmpfs-mounts the state dirs) and asks whether to keep it (Quickshell dialog + `pkexec`
polkit-granted restore helper). The whole restore path is live-ISO source, copied unchanged
by `install/isocopy`. `AGENTS.md` → Constants Reference holds the measured details of all of this.

### Partition layout
Both filesystems use the same 2-partition GPT: 512 MiB EFI (vfat) + rest (ext4/btrfs).
Manual mode formats only the designated Boot + System partitions and never reuses another
OS's ESP. LUKS2 always sits below LVM; one PV per install, the VG named after the install id
(PARTUUID-derived). **fstab and the boot entry always reference filesystem UUIDs, never
`/dev/mapper` paths.** Each install owns its own kernel dir on the ESP (`aelum/<id>/`), so
several installs share a disk without overwriting each other.

### Inbound filtering (every install)
A desktop is carried onto networks nobody owns, so an install ships a default-deny
inbound policy: `firewalld` (base step) with `firewall-config` beside it, and
`firewalld.service` enabled as a **required** service — a machine that was supposed to
get a firewall behaves exactly like a machine that has one until somebody tries its
ports, so a failed enable has to abort the install and name the unit. The policy is the
zone a network lands in: NetworkManager leaves connections unzoned, so firewalld's
`DefaultZone` (`public`, the packaged value) answers for every network a laptop joins
until a person picks a zone per connection. The installer deliberately never edits
`/etc/firewalld/firewalld.conf` — it is in the package's backup list, so a line added
there becomes a merge prompt on every future upgrade and a pinned value would outlive a
change of upstream default — which is why `verify.checkFirewall` *reads* the zone and
refuses only `trusted`, the one zone that accepts everything. `openssh` is on the disk as
a dependency and nothing enables `sshd`, so a fresh install has no listener at all.

---

## Build & Test

```bash
./build.sh                # or: make build → build/bin/{aelum-installer,aelum-installer-cli}
make dev | test | lint    # wails dev / go test / golangci-lint --fix
cd frontend && npm run test|check|e2e
```

`./build.sh` runs: `go mod tidy` → mirrorlist refresh (`mirrorlist_gen.go`) → TS constants
(`gen-ts-constants.go`) → frontend build → a Wails window-startup patch → `wails build` →
CLI `go build`. `SHARED_DIR=`/`BIN_DIR=` also publish the binaries (VM shared dir / ISO bin dir).

| Test layer | Where | What |
|-----------|-------|------|
| Go unit + integration | `go test ./...` | steps via the primitives seams, validators, progress, disks |
| Frontend unit | `cd frontend && npm run test` | Vitest: stores, validation, components |
| Frontend e2e | `cd frontend && npm run e2e` | Playwright against the real Wails app in dry-run (see `frontend/e2e/README.md`) |
| Real installs | `docker/` in the distro workspace | QEMU matrix auto/manual × ext4/btrfs × LUKS × LVM against `/dev/vda` — never on the host |

---

## Project Structure (top level)

```
main.go                        # Wails entry (AELUM_START_PAGE, SIGINT/TERM/HUP → cancel + cleanup)
backend/
  cli/                         # headless installer (password from stdin)
  gui/                         # Wails layer: app, bindings, CLI spawn, navigation, validation
  install/
    installer.go  step.go  context.go  cleanup.go  errors.go
    primitives/   # + primitivestest/ (fake runner + memfs)
    format/  lvm/  mount/  partition/            # disk stack
    installbase/  installdesktop/                # pacstrap steps (+ drivers, input methods)
    users/  isocopy/                             # accounts, weather location, ISO file copy
    configure/                                   # the merged config step (fstab…snapper)
    verify/  clearcache/  greetd/  progress/
  shared/                      # types, config.go (ALL constants), disks, regions, mirrorlists
  validation/  scripts/        # validation facade; code generators
frontend/                      # Svelte 5 wizard (see frontend/AGENTS.md)
docs/                          # this file, EXECUTION_FLOW, TROUBLESHOOTING, CONTRIBUTING, …
```

---

## Desktop Installed

Hyprland + Dux Shell + Dux Greeter + Foot + Firefox + PipeWire + zram + branded Plymouth.
Ships: Nautilus (+video thumbnailers), gnome-text-editor, mpv, amberol, papers, loupe, btop, fastfetch,
brightnessctl, ddcutil, cava, matugen, `aelum-video-player`, `aelum-package-installer`,
the Fcitx5 input method core (engines chosen per language), the firewall and its
configuration window (`firewalld` + `firewall-config`, "Firewall" in the launcher),
the VPN plugins the shell's toggle drives (`networkmanager-vpn-plugin-openvpn` and
`-openconnect`, plus `wireguard-tools`; WireGuard itself is built into
NetworkManager and the kernel),
Noto CJK/Emoji + Terminus + JetBrains Mono Nerd fonts. Btrfs installs additionally
carry snapper (base) and btrfs-assistant (desktop).
