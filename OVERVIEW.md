# AestheticOS Installer — Project Overview

## Live ISO vs Installed System

**Never confuse these two. It is the #1 source of bugs.**

| Term | What It Is | Key Paths |
|------|-----------|-----------|
| **Live ISO** | Bootable Arch environment running the installer. Read-only rootfs (overlay). | `/etc/sudoers.d/aestheticos` — lets helper run via sudo. Helper binary at `/usr/local/bin/aestheticos-helper`. |
| **Installed System** | Arch Linux being written to the target disk. | Mounted at `/mnt/` during install. ESP at `/mnt/boot/`. `/etc/sudoers.d/wheel` written here for the new user. Accessed via `arch-chroot /mnt`. |
| **Host** | Developer's main machine. | Where `build.sh` runs, git lives. `~/Projects/shared` shared with VM. |
| **VM** | QEMU VM booting from ISO for testing. | `/shared` mounted from host shared dir. |

**Code rule:** If it touches `/mnt/*` or runs via `arch-chroot` → installed system. If it touches `/dev/*` or runs directly → live ISO.

---

## What It Is

AestheticOS Installer is a **graphical Arch Linux installer** built with [Wails v2](https://wails.io/) (Go backend + Svelte 5 frontend). It provides a step-by-step wizard to install a custom Arch Linux desktop environment ("AestheticOS") onto a target disk.

It also ships a **CLI installer** (`aestheticos-installer-cli`) and a **privileged helper binary** (`aestheticos-helper`) for headless/automated use.

**This is an offline installer.** All packages are pulled from the live ISO's pacman database — no network connectivity is required or used during installation.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **GUI Framework** | [Wails v2](https://wails.io/) (Go + WebView2/WebKit) |
| **Backend** | Go 1.25 |
| **Frontend** | Svelte 5 (TypeScript), Vite, Tailwind-like CSS variables |
| **Target OS** | Arch Linux (installed via `pacstrap`) |
| **Desktop** | Hyprland + DankMaterialShell (based on Quickshell) + DMS Greeter (greetd) |
| **Bootloader** | systemd-boot (bootctl) |
| **Filesystems** | ext4 or btrfs (with subvolumes) |
| **Boot Splash** | Plymouth BGRT (silent boot with firmware background) |

---

## Architecture

```
┌───────────────────────────────────────────────────────────┐
│                    Wails Application                      │
├──────────────────────┬────────────────────────────────────┤
│   Frontend (Svelte)  │   Backend (Go)                     │
│                      │                                    │
│  6 Wizard Pages      │  GUI Layer (gui/)                  │
│  • Welcome           │    • App struct (state mgmt)       │
│  • User Account      │    • Validation (delegate)         │
│  • Disk Selection    │    • Navigation                    │
│  • Confirmation      │    • Root check                    │
│  • Installation      │    • Installation orchestration    │
│  • Finish            │                                    │
│                      │  Core Layer (core/)                │
│                      │    • Installer (orchestrator)      │
│                      │    • RootCheck                     │
│                      │                                    │
│                      │  Install Layer (install/)          │
│                      │    • format, mount, fstab          │
│                      │    • install_base, install_desktop │
│                      │    • bootloader, users, system     │
│                      │    • greetd, plymouth, services    │
│                      │    • swap, copy (ISO files)        │
│                      │    • cleanup                       │
│                      │    • command (chroot/write helpers)│
│                      │    • config (constants)            │
│                      │    • context (InstallContext)      │
│                      │    • checks (validation helpers)   │
│                      │    • logging (async logger)        │
│                      │                                    │
│                      │  Shared Layer (shared/)            │
│                      │    • types, config, validators     │
│                      │    • disks (lsblk), regions,       │
│                      │      locales, btrfs, mirrors       │
└──────────────────────┴────────────────────────────────────┘
                          │
                          ▼
              ┌──────────────────────────┐
              │ aestheticos-helper       │
              │ (sudo-privileged)        │
              │ • parted, format         │
              │ • mount, umount          │
              │ • chroot-run (whitelist) │
              │ • write/append/read-file │
              └──────────────────────────┘
```

### Three Binaries

| Binary | Purpose | Entry Point |
|--------|---------|-------------|
| `aestheticos-installer` | GUI wizard (Wails) | `main.go` |
| `aestheticos-installer-cli` | CLI automated install | `backend/cmd/installer-cli/main.go` |
| `aestheticos-helper` | Privileged system ops via sudo | `backend/cmd/helper/main.go` |

---

## Installation Steps (6 top-level, 12 logical)

The `Installer.Install()` method runs 6 top-level steps sequentially. The last step merges 7 configuration sub-steps into one:

| # | Step | Weight | Function | Description |
|---|------|--------|----------|-------------|
| 1 | Format Disk | 3 | `FormatDisk` | Partitions (GPT: EFI 128MB + Root rest), formats (vfat/ext4/btrfs), btrfs subvolumes |
| 2 | Mount Filesystem | 2 | `MountFilesystem` | Mounts root + btrfs subvolumes to `/mnt`, ESP to `/mnt/boot` |
| 3 | Install Base | 45 | `Install` | `pacstrap /mnt base linux-lts linux-firmware ...` |
| 4 | Install Desktop | 35 | `InstallDesktop` | `pacstrap` Hyprland, DMS greeter, alacritty, firefox, pipewire, etc. |
| 5 | Setup Users | 2 | `SetupUsers` | Creates user (wheel group), sets passwords, copies ISO skel files, sudoers |
| 6 | Configure System | 13 | `ConfigureSystem` | Merged: fstab → bootloader → system config → greetd → Plymouth → zram → services |

**Step 6 sub-steps (in order):**

| # | Sub-step | Function | Description |
|---|----------|----------|-------------|
| 6.1 | Generate fstab | `GenerateFstab` | Writes `/etc/fstab` (custom for btrfs subvolumes) |
| 6.2 | Configure Bootloader | `ConfigureBootloader` | systemd-boot install, boot entry, random seed |
| 6.3 | Configure System | `ConfigureSystemBase` | Hostname, timezone, locale, keymap, NTP, mirrorlist |
| 6.4 | Configure greetd | `ConfigureGreetd` | Writes `/etc/greetd/config.toml`, creates greeter user, syncs DMS greeter |
| 6.5 | Configure Plymouth | `ConfigurePlymouth` | BGRT theme, mkinitcpio hook, `.hushlogin` |
| 6.6 | Configure zram | `ConfigureZram` | Compressed swap via zram-generator |
| 6.7 | Enable Services | `EnableServices` | Enables NetworkManager, greetd, bluetooth |

### Progress Tracking

The two pacstrap steps (base + desktop) together account for ~80% of total installation time. Progress is tracked via:
- **Line-count-based pacstrap tracker**: parses actual pacstrap output lines (downloading, verifying, installing, hooks) for smooth, accurate progress
- **Weighted step percentages**: base pacstrap (45), desktop pacstrap (35), remaining steps (20 combined)
- **Tick goroutine**: advances percentage during long steps to keep the UI feeling alive

---

## Frontend Pages (Wizard Flow)

```
Welcome (language select)
    → User Account (username + password)
        → Disk Selection (disk cards + filesystem pills)
            → Confirmation (summary + root check + confirm dialog)
                → Installation (progress bar + live events)
                    → Finish (success)
```

### Key Frontend Files

| File | Purpose |
|------|---------|
| `src/App.svelte` | Root component, page routing, navigation events, start page support |
| `src/lib/state.svelte.ts` | Svelte stores for user account, disk selection, page state |
| `src/lib/types.ts` | TypeScript interfaces, page IDs, constants |
| `src/lib/navigation.ts` | Frontend validation calls to Go backend |
| `src/lib/validate.ts` | Client-side form validation |
| `src/pages/*.svelte` | 6 wizard pages |
| `src/components/*.svelte` | Reusable components (Button, Dropdown, PageLayout, ConfirmDialog, ErrorBanner, etc.) |

---

## Key Design Decisions

### Helper Binary Pattern
All privileged operations go through `aestheticos-helper`, invoked via `sudo`. This avoids giving the entire Wails app root access — only specific commands are allowed via sudoers. The helper is a single `main.go` file with a **command registration pattern**: each command is a `HelperCommand` struct (`Name`, `Usage`, `Help`, `Run`) registered in a slice.

Two whitelists control allowed operations:
- **`runWhitelist`** — host-level commands via the generic `run` command
- **`chrootRunWhitelist`** — commands allowed inside chroot via `chroot-run`

**Commands:**
- **Chroot commands**: `chroot-run` (whitelisted cmds only), `run-chpasswd`
- **Target filesystem ops**: `write-file`, `append-file`, `read-file`, `set-symlink`
- **Host-level (via `run`)**: `blkid`, `findmnt`, `ls`, `test`, `find`, `chmod`, `chown`, `touch`, `mkdir`, `rmdir`, `cp`, `dd`, `fallocate`, `sgdisk`, `mkswap`, `swapon`, `swapoff`, `reboot`, `passwd`, `useradd`, `chattr`, `bootctl`
- **Partition/format**: `parted`, `format`, `mount`, `umount`, `mountpoints`, `lsblk-partitions`, `wipefs`
- **Package/fstab**: `pacstrap`, `genfstab`
- **Bootloader**: `bootctl <subcommand> <esp>` (install|cleanup|random-seed|set-default|is-installed|status), `efibootmgr` (list|delete)
- **btrfs**: `btrfs-subvol-create`, `btrfs-subvol-set-default`, `btrfs-set-compression`, `btrfs-mkswapfile`
- **Other**: `udev-settle`

### Chroot via arch-chroot
System configuration inside the installed system uses `arch-chroot /mnt` through the helper's `chroot-run` command (whitelisted commands only). This handles `/proc`, `/sys`, `/dev` mounting automatically.

### ISO File Copying
Static files from the live ISO are copied to the installed system during `SetupUsers`:
- `/etc/skel/` → user home directory
- `/etc/dconf/profile/user` and `01-icons`
- DMS wallpaper
- `/etc/environment`
- `/etc/greetd/hypr.lua`

### Dry Run Mode
Set `AESTHETIC_DRY_RUN=1` environment variable to simulate installation (steps sleep 200ms instead of executing). Useful for UI testing.

### Fake Disks
Set `FAKE_DISKS=1` to get synthetic disk data for frontend development without real hardware.

### Start Page
The installer supports starting on any page via the `StartPage` constant in `main.go` (0=Welcome, 1=User Account, etc.). Set to `-1` for default Welcome page. The GUI exposes `GetStartPage()` and `SetStartPage()` Wails bindings.

### btrfs Subvolumes
When btrfs is selected, 7 subvolumes are created: `@`, `@home`, `@srv`, `@cache`, `@tmp`, `@log`, `@swap`. Each has configurable compression (zstd) and mount options. `@` is set as default. `@cache`, `@tmp`, `@swap` use `nodatacow` to avoid CoW overhead.

### Partition Layout
Both ext4 and btrfs use the same 2-partition GPT layout:
- **Partition 1**: 128MB EFI (vfat, type GUID `C12A7328...`)
- **Partition 2**: Rest of disk (ext4/btrfs, type GUID `4F68BCE3...`)

### Mirrorlists
Region-specific mirrorlists are embedded in the binary. A `mirrorlist_gen.go` script refreshes them from `archlinux.org/mirrorlist/all/https/`.

---

## Shared Host/VM Setup

- **Host**: `~/Projects/shared`
- **VM**: `/shared` (mounted from host shared dir)
- **ISO bin dir**: `~/Projects/AestheticOS/airootfs/usr/local/bin`
- Built binaries are copied to both locations by `build.sh`

---

## Building

```bash
./build.sh
```

This runs:
1. Mirrorlist refresh (`mirrorlist_gen.go`)
2. Frontend `npm install` + `npm run build`
3. `wails build -clean -tags webkit2_41` (GUI binary)
4. `go build` for helper and CLI binaries
5. Copies all binaries to shared dir and ISO bin dir

---

## Testing

```bash
go test ./...          # Run all Go tests (unit + integration)
./run.sh               # Start Wails dev server (hot-reload)
```

### Test Types

| Type | Location | What it covers |
|------|----------|----------------|
| Unit | `backend/internal/*/ *_test.go` | Validators, progress tracker, disk parsing, context |
| Integration | `backend/internal/core/installer_integration_test.go` | Full install flow (dry-run), step ordering, progress, error handling, rollback |
| GUI | `backend/gui/*_test.go` | Validation logic, root check |
| Frontend | `frontend/` (Vitest) | Component tests |

---

## Project Structure

```
AestheticOS-Installer/
├── main.go                      # Wails app entry point (StartPage support)
├── wails.json                   # Wails config
├── build.sh / run.sh            # Build & dev scripts
├── RULES.md                     # Development rules
├── backend/
│   ├── cmd/
│   │   ├── helper/main.go       # Privileged helper (command registration pattern)
│   │   └── installer-cli/main.go # CLI installer
│   ├── gui/
│   │   ├── app.go               # Wails app struct, reboot, start page
│   │   ├── bindings.go          # Wails API bindings (disks, languages, fs)
│   │   ├── installation.go      # StartInstallation orchestration
│   │   ├── navigation.go        # Page IDs, navigation state
│   │   ├── root_check.go        # Sudo verification
│   │   ├── validation.go        # Validate + Save methods
│   │   └── logger.go            # Error log writer
│   └── internal/
│       ├── core/
│       │   ├── installer.go     # Installer orchestrator (shared by CLI/GUI)
│       │   ├── installer_test.go
│       │   ├── logging.go
│       │   └── root_check.go
│       ├── install/
│       │   ├── format.go        # Partitioning + formatting
│       │   ├── mount.go         # Mount filesystem + verification
│       │   ├── install_base.go  # pacstrap base packages
│       │   ├── install_desktop.go # pacstrap desktop packages
│       │   ├── install.go       # Pacstrap progress tracking
│       │   ├── fstab.go         # fstab generation + verification
│       │   ├── bootloader.go    # systemd-boot install + boot entries
│       │   ├── users.go         # User account creation + ISO copy
│       │   ├── system.go        # Hostname, locale, timezone, mirrors
│       │   ├── greetd.go        # Login manager config + DMS sync
│       │   ├── plymouth.go      # Silent boot (BGRT theme)
│       │   ├── services.go      # systemd service enabling
│       │   ├── swap.go          # Swap file creation
│       │   ├── copy.go          # ISO-to-target file copying
│       │   ├── cleanup.go       # Mount cleanup
│       │   ├── command.go       # chroot + writeFile/appendFile helpers
│       │   ├── config.go        # All install constants
│       │   ├── context.go       # InstallContext + Logger interface
│       │   ├── checks.go        # Validation helpers
│       │   └── logging.go       # Async logger
│       └── shared/
│           ├── types.go         # Data structures (DiskInfo, Region, etc.)
│           ├── config.go        # Constants + thresholds
│           ├── disks.go         # lsblk parsing + fake data
│           ├── regions.go       # 28 locale/region definitions
│           ├── locales.go
│           ├── btrfs.go         # Subvolume definitions
│           ├── mirrorlist.go    # Mirrorlist handling
│           ├── mirrorlist_gen.go # Mirrorlist refresh script
│           ├── validators.go    # Input validation
│           └── logpath.go
└── frontend/
    ├── src/
    │   ├── App.svelte           # Root component
    │   ├── main.ts              # Entry point
    │   ├── lib/
    │   │   ├── state.svelte.ts  # Svelte stores
    │   │   ├── types.ts         # TS types + constants
    │   │   ├── navigation.ts    # Nav + validation calls
    │   │   └── validate.ts      # Client validation
    │   ├── pages/               # 6 wizard pages
    │   └── components/          # Reusable UI components
    └── wailsjs/                 # Auto-generated Wails bindings
```

---

## Desktop Environment (AestheticOS)

The installed system includes:
- **Window Manager**: Hyprland (Wayland compositor)
- **Shell**: DankMaterialShell (based on Quickshell) + dms-shell-hyprland
- **Login Manager**: greetd + DMS greeter
- **Terminal**: Alacritty
- **File Manager**: Nautilus
- **Browser**: Firefox
- **Audio**: PipeWire + WirePlumber
- **Utilities**: btop, fastfetch, brightnessctl, ddcutil, cava, matugen
- **Fonts**: Noto (including CJK + Emoji) + Terminus
- **Boot Splash**: Plymouth BGRT (uses firmware background image for silent boot)
