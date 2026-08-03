# AestheticOS Installer — Project Overview

## What It Is

AestheticOS Installer is a **graphical Arch Linux installer** built with [Wails v2](https://wails.io/) (Go backend + Svelte 5 frontend). It provides a step-by-step wizard to install a custom Arch Linux desktop environment ("AestheticOS") onto a target disk.

It also ships a **CLI installer** (`aestheticos-installer-cli`) and a **privileged helper binary** (`aestheticos-helper`) for headless/automated use.

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
│  • Installation      │                                    │
│  • Finish            │  Core Layer (core/)                │
│                      │    • Installer (orchestrator)      │
│                      │    • RootCheck                     │
│                      │                                    │
│                      │  Install Layer (install/)          │
│                      │    • format, mount, fstab          │
│                      │    • install_base, install_desktop │
│                      │    • bootloader, users, system     │
│                      │    • greetd, services, swap        │
│                      │    • cleanup                       │
│                      │                                    │
│                      │  Shared Layer (shared/)            │
│                      │    • types, config, validators     │
│                      │    • disks (lsblk), regions,       │
│                      │      locales, btrfs, mirrors       │
└──────────────────────┴────────────────────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │ aestheticos-helper    │
              │ (sudo-privileged)     │
              │ • parted, format      │
              │ • mount, umount       │
              │ • run (arbitrary cmd) │
              │ • bootctl-install     │
              │ • btrfs subvol cmds   │
              │ • passwd, useradd     │
              │ • reboot              │
              └───────────────────────┘
```

### Three Binaries

| Binary | Purpose | Entry Point |
|--------|---------|-------------|
| `aestheticos-installer` | GUI wizard (Wails) | `main.go` |
| `aestheticos-installer-cli` | CLI automated install | `backend/cmd/installer-cli/main.go` |
| `aestheticos-helper` | Privileged system ops via sudo | `backend/cmd/helper/main.go` |

---

## Installation Steps (11 total)

The `Installer.Install()` method runs these steps sequentially:

| # | Step | Function | Description |
|---|------|----------|-------------|
| 1 | Format Disk | `FormatDisk` | Partitions (GPT: EFI 128MB + Root rest), formats (vfat/ext4/btrfs), btrfs subvolumes |
| 2 | Mount Filesystem | `MountFilesystem` | Mounts root + btrfs subvolumes to `/mnt`, ESP to `/mnt/boot` |
| 3 | Install Base | `Install` | `pacstrap /mnt base linux-lts linux-firmware ...` |
| 4 | Install Desktop | `InstallDesktop` | `pacstrap` Hyprland, DMS greeter, alacritty, firefox, pipewire, etc. |
| 5 | Create Swap | `CreateSwap` | 4GB swap file (btrfs: inside `@swap` subvolume; ext4: fallocate/dd) |
| 6 | Generate fstab | `GenerateFstab` | Writes `/etc/fstab` (custom for btrfs subvolumes) |
| 7 | Configure Bootloader | `ConfigureBootloader` | systemd-boot install, boot entry, random seed |
| 8 | Setup Users | `SetupUsers` | Creates user (wheel group), sets passwords, sudoers |
| 9 | Configure System | `ConfigureSystem` | Hostname, timezone, locale, keymap, mirrorlist |
| 10 | Configure greetd | `ConfigureGreetd` | Writes `/etc/greetd/config.toml`, creates greeter user |
| 11 | Enable Services | `EnableServices` | Enables NetworkManager, greetd |

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
| `src/App.svelte` | Root component, page routing, navigation events |
| `src/lib/state.svelte.ts` | Svelte stores for user account, disk selection, page state |
| `src/lib/types.ts` | TypeScript interfaces, page IDs, constants |
| `src/lib/navigation.ts` | Frontend validation calls to Go backend |
| `src/lib/validate.ts` | Client-side form validation |
| `src/pages/*.svelte` | 6 wizard pages |
| `src/components/*.svelte` | Reusable components (Button, Dropdown, PageLayout, etc.) |

---

## Key Design Decisions

### Helper Binary Pattern
All privileged operations go through `aestheticos-helper`, invoked via `sudo`. This avoids giving the entire Wails app root access — only specific commands are allowed via sudoers.

### Chroot via arch-chroot
System configuration inside the installed system uses `arch-chroot /mnt` through the helper's `run` command. This handles `/proc`, `/sys`, `/dev` mounting automatically.

### Dry Run Mode
Set `AESTHETIC_DRY_RUN=1` environment variable to simulate installation (steps sleep 200ms instead of executing). Useful for UI testing.

### Fake Disks
Set `FAKE_DISKS=1` to get synthetic disk data for frontend development without real hardware.

### btrfs Subvolumes
When btrfs is selected, 7 subvolumes are created: `@`, `@home`, `@srv`, `@cache`, `@tmp`, `@log`, `@swap`. Each has configurable compression (zstd) and mount options. `@` is set as default.

### Partition Layout
Both ext4 and btrfs use the same 2-partition GPT layout:
- **Partition 1**: 128MB EFI (vfat, type GUID `C12A7328...`)
- **Partition 2**: Rest of disk (ext4/btrfs, type GUID `4F68BCE3...`)

### Mirrorlists
Region-specific mirrorlists are embedded in the binary. A `mirrorlist_gen.go` script refreshes them from `archlinux.org/mirrorlist/all/https/`.

---

## Shared Host/VM Setup

- **Host**: `/home/power007/Projects/shared`
- **VM**: `/shared` (mounted from host shared dir)
- **ISO bin dir**: `/home/power007/Projects/AestheticOS/airootfs/usr/local/bin`
- Built binaries are copied to both locations by `build.sh`

---

## Building

```bash
./build.sh
```

This runs:
1. Mirrorlist refresh
2. Frontend `npm install` + `npm run build`
3. `wails build -clean -tags webkit2_41` (GUI binary)
4. `go build` for helper and CLI binaries
5. Copies all binaries to shared dir and ISO bin dir

---

## Testing

```bash
go test ./...          # Run all Go tests
./run.sh               # Start Wails dev server (hot-reload)
```

---

## Project Structure

```
AestheticOS-Installer/
├── main.go                      # Wails app entry point
├── wails.json                   # Wails config
├── build.sh / run.sh            # Build & dev scripts
├── RULES.md                     # Development rules
├── backend/
│   ├── cmd/
│   │   ├── helper/main.go       # Privileged helper (22 commands)
│   │   └── installer-cli/main.go # CLI installer
│   ├── gui/
│   │   ├── app.go               # Wails app struct, reboot
│   │   ├── bindings.go          # Wails API bindings
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
│       │   ├── fstab.go         # fstab generation + verification
│       │   ├── bootloader.go    # systemd-boot install + boot entries
│       │   ├── users.go         # User account creation
│       │   ├── system.go        # Hostname, locale, timezone, mirrors
│       │   ├── greetd.go        # Login manager config
│       │   ├── services.go      # systemd service enabling
│       │   ├── swap.go          # Swap file creation
│       │   ├── cleanup.go       # Mount cleanup
│       │   ├── command.go       # chroot + writeFile helpers
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
