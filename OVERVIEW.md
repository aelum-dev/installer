# Aelum Installer — Project Overview

## Live ISO vs Installed System

**Never confuse these two. It is the #1 source of bugs.**

| Term | What It Is | Key Paths |
|------|-----------|-----------|
| **Live ISO** | Bootable Arch environment running the installer. Read-only rootfs (overlay). | Sudoers: user needs passwordless sudo for CLI binary. |
| **Installed System** | Arch Linux being written to the target disk. | Mounted at `/mnt/` during install. ESP at `/mnt/efi`; this install's `aelum/<id>/` boot directory bind-mounted at `/mnt/boot`. `/etc/sudoers.d/wheel` written here for the new user. Accessed via `arch-chroot /mnt`. |
| **Host** | Developer's main machine. | Where `build.sh` runs, git lives. `~/Projects/shared` shared with VM. |
| **VM** | QEMU VM booting from ISO for testing. | `/shared` mounted from host shared dir. |

**Code rule:** If it touches `/mnt/*` or runs via `arch-chroot` → installed system. If it touches `/dev/*` or runs directly → live ISO.

---

## What It Is

Aelum Installer is a **graphical Arch Linux installer** built with [Wails v2](https://wails.io/) (Go backend + Svelte 5 frontend). It provides a step-by-step wizard to install a custom Arch Linux desktop environment ("Aelum") onto a target disk.

It also ships a **CLI installer** (`aelum-installer-cli`) for headless/automated use. The GUI spawns the CLI binary via `sudo` for all privileged operations — no separate helper binary is needed.

**This is an offline installer.** All packages are pulled from the live ISO's pacman database — no network connectivity is required or used during installation.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **GUI Framework** | [Wails v2](https://wails.io/) (Go + WebView2/WebKit) |
| **Backend** | Go 1.26 |
| **Frontend** | Svelte 5 (TypeScript), Vite, Tailwind-like CSS variables |
| **Target OS** | Arch Linux (installed via `pacstrap`) |
| **Desktop** | Hyprland + DankMaterialShell (based on Quickshell) + DMS Greeter (greetd) |
| **Bootloader** | rEFInd (auto-detects other OSes for dualboot) |
| **Filesystems** | ext4 or btrfs (with subvolumes) |
| **Boot Splash** | Plymouth, branded Aelum theme (silent boot; LUKS passphrase unlock) |

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
│  • Installation      │    • CLI spawning + progress       │
│  • Finish            │      (JSON events from CLI stderr) │
│                      │  Install Layer (install/)          │
│                      │    • Installer (orchestrator)      │
│                      │    • Step interface + impls        │
│                      │    • Plan (weights/percentages)    │
│                      │    • Executor interface            │
│                      │    • Logger interface              │
│                      │    • format, mount, fstab          │
│                      │    • install_base, install_desktop │
│                      │    • bootloader, users, system     │
│                      │    • greetd, plymouth, services    │
│                      │    • zram, copy (ISO files)        │
│                      │    • cleanup, rollback             │
│                      │    • command (Run/runChroot)       │
│                      │    • config (constants)            │
│                      │    • context (InstallContext)      │
│                      │    • checks (validation helpers)   │
│                      │    • progress (pacstrap tracker)   │
│                      │                                    │
│                      │  Shared Layer (shared/)            │
│                      │    • types, config, validators     │
│                      │    • disks (lsblk), regions,       │
│                      │      locales, btrfs, mirrors       │
└──────────────────────┴────────────────────────────────────┘
                          │
                          ▼
              ┌──────────────────────────┐
              │ aelum-installer-cli│
              │ (sudo-privileged)        │
              │ • Direct exec.Command    │
              │ • arch-chroot /mnt       │
              │ • JSON progress → stderr │
              └──────────────────────────┘
```

### Two Binaries

| Binary | Purpose | Entry Point |
|--------|---------|-------------|
| `aelum-installer` | GUI wizard (Wails) | `main.go` |
| `aelum-installer-cli` | CLI automated install | `backend/cli/main.go` |

The GUI spawns the CLI via `sudo` — all privileged operations run inside the CLI process.

---

## Installation Steps (6 top-level, 12 logical)

The `Installer.Install()` method runs 6 top-level steps sequentially. The last step merges 7 configuration sub-steps into one:

| # | Step | Weight | Function | Description |
|---|------|--------|----------|-------------|
| 1 | Format Disk | 3 | `FormatDisk` | Partitions (GPT: EFI 512MB + Root rest), formats (vfat/ext4/btrfs), btrfs subvolumes |
| 2 | Mount Filesystem | 2 | `MountFilesystem` | Mounts root + btrfs subvolumes to `/mnt`, the ESP to `/mnt/efi`, and this install's `aelum/<id>/` ESP directory at `/mnt/boot` |
| 3 | Install Base | 45 | `Install` | `pacstrap /mnt base linux-lts linux-firmware ...` |
| 4 | Install Desktop | 35 | `InstallDesktop` | `pacstrap` Hyprland, DMS greeter, kitty, firefox, pipewire, etc. |
| 5 | Setup Users | 2 | `SetupUsers` | Creates user (wheel group), sets passwords, copies ISO skel files, sudoers |
| 6 | Configure System | 13 | `ConfigureSystem` | Merged: fstab → bootloader → system config → greetd → Plymouth → zram → services |

**Step 6 sub-steps (in order):**

| # | Sub-step | Function | Description |
|---|----------|----------|-------------|
| 6.1 | Generate fstab | `GenerateFstab` | Writes `/etc/fstab` (custom for btrfs subvolumes): ESP at `/efi`, this install's `aelum/<id>/` boot dir bind-mounted at `/boot` |
| 6.2 | Configure Bootloader | `ConfigureBootloader` | rEFInd install onto the ESP, per-install `aelum/<id>.conf` menu fragment (loads this install's own `aelum/<id>/` kernel), NVRAM boot entry |
| 6.3 | Configure System | `ConfigureSystemBase` | Hostname, timezone, locale, keymap, NTP, mirrorlist |
| 6.4 | Configure greetd | `ConfigureGreetd` | Writes `/etc/greetd/config.toml`, creates greeter user, syncs DMS greeter |
| 6.5 | Configure Plymouth | `ConfigurePlymouth` | Aelum theme, mkinitcpio hook, `.hushlogin` |
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

### CLI Spawning Pattern
The GUI spawns `aelum-installer-cli` via `sudo` for all privileged operations. The CLI runs as root and:
- Executes system commands directly via `exec.Command`
- Runs commands inside the installed system via `arch-chroot /mnt`
- Emits JSON progress events to stderr (read by the GUI)
- Writes human-readable output to stdout (forwarded to terminal by the GUI)

This eliminates the need for a separate helper binary and its command whitelist security surface.

### Executor Interface
The `Executor` interface in `backend/install/executor.go` abstracts command execution:
- `Run(args...)` — executes a command directly
- `RunChroot(args...)` — executes inside the installed system via `arch-chroot`
- `WriteFile(path, content)` — writes a file inside the installed system
- `AppendFile(path, content)` — appends to a file inside the installed system

The default implementation is `HostExecutor`. This interface enables testing with mock implementations.

### Chroot via arch-chroot
System configuration inside the installed system uses `arch-chroot /mnt` directly. This handles `/proc`, `/sys`, `/dev` mounting automatically.

### ISO File Copying
Static files from the live ISO are copied to the installed system during `SetupUsers`:
- `/etc/skel/` → user home directory
- `/etc/dconf/profile/user` and `01-icons`
- DMS wallpaper
- `/etc/environment`
- `/etc/greetd/hypr.lua`

### Dry Run Mode
Set `AELUM_DRY_RUN=1` environment variable to simulate installation (steps sleep 200ms instead of executing). Useful for UI testing.

### Fake Disks
Set `FAKE_DISKS=1` to get synthetic disk data for frontend development without real hardware.

### Start Page
The installer supports starting on any page via the `StartPage` constant in `main.go` (0=Welcome, 1=User Account, etc.). Set to `-1` for default Welcome page. The GUI exposes `GetStartPage()` and `SetStartPage()` Wails bindings.

### btrfs Subvolumes
When btrfs is selected, 6 subvolumes are created: `@` (default), `@home`, `@srv`, `@cache`, `@tmp`, `@log`. Each has configurable compression (zstd) and mount options. `@` is set as default. `@cache` and `@tmp` use `nodatacow` to avoid CoW overhead. Swap is provided by zram, not a subvolume.

### Partition Layout
Both ext4 and btrfs use the same 2-partition GPT layout:
- **Partition 1**: 512MB EFI (vfat, type GUID `C12A7328...`)
- **Partition 2**: Rest of disk (ext4/btrfs, type GUID `4F68BCE3...`)

### Mirrorlists
Region-specific mirrorlists are embedded in the binary. A `mirrorlist_gen.go` script refreshes them from `archlinux.org/mirrorlist/all/https/`.

---

## Shared Host/VM Setup

- **Host**: `~/Projects/shared`
- **VM**: `/shared` (mounted from host shared dir)
- **ISO bin dir**: `~/Projects/Aelum/airootfs/usr/local/bin`
- Built binaries are copied to both locations by `build.sh`

---

## Building

```bash
./build.sh
```

This runs:
1. Mirrorlist refresh (`mirrorlist_gen.go`)
2. TypeScript constants generation (`gen-ts-constants.go`)
3. Frontend `npm install` + `npm run build`
4. `wails build -clean -tags webkit2_41` (GUI binary)
5. `go build` for CLI binary
6. Copies binaries to shared dir and ISO bin dir (if `SHARED_DIR`/`BIN_DIR` set)

---

## Testing

```bash
go test ./...          # Run all Go tests (unit + integration)
./run.sh               # Start Wails dev server (hot-reload)
```

### Test Types

| Type | Location | What it covers |
|------|----------|----------------|
| Unit | `backend/*/ *_test.go` | Validators, progress tracker, disk parsing, context |
| Integration | `backend/install/installer_integration_test.go` | Full install flow (dry-run), step ordering, progress, error handling, rollback |
| GUI | `backend/gui/*_test.go` | Validation logic, root check |
| Frontend | `frontend/` (Vitest) | Component tests |

---

## Project Structure

```
Aelum-Installer/
├── main.go                      # Wails app entry point (StartPage support)
├── wails.json                   # Wails config
├── build.sh / run.sh            # Build & dev scripts
├── RULES.md                     # Development rules
├── backend/
│   ├── cli/main.go              # CLI installer
│   ├── gui/
│   │   ├── app.go               # Wails app struct, reboot, start page
│   │   ├── bindings.go          # Wails API bindings (disks, languages, fs)
│   │   ├── installation.go      # StartInstallation (spawns CLI via sudo)
│   │   ├── navigation.go        # Page IDs, navigation state
│   │   ├── root_check.go        # Sudo verification
│   │   ├── validation.go        # Validate + Save methods
│   │   └── logger.go            # GUILogger
│   ├── install/
│   │   ├── installer.go         # Installer orchestrator (shared by CLI/GUI)
│   │   ├── step.go              # Step interface + step implementations
│   │   ├── plan.go              # Plan struct for weights/percentages
│   │   ├── executor.go          # Executor interface
│   │   ├── configuration.go     # Merged ConfigureSystem
│   │   ├── errors.go            # ValidationError, InstallError
│   │   ├── logger.go            # Logger interface + ConsoleLogger
│   │   ├── format.go            # Partitioning + formatting
│   │   ├── mount.go             # Mount filesystem + verification
│   │   ├── install_base.go      # pacstrap base packages
│   │   ├── install_desktop.go   # pacstrap desktop packages
│   │   ├── install.go           # Pacstrap progress tracking
│   │   ├── fstab.go             # fstab generation + verification
│   │   ├── bootloader.go        # rEFInd install + branded theme + per-install menu fragment
│   │   ├── users.go             # User account creation + ISO copy
│   │   ├── system.go            # Hostname, locale, timezone, mirrors
│   │   ├── greetd.go            # Login manager config + DMS sync
│   │   ├── greetd/              # greetd package
│   │   │   ├── greetd.go        # ConfigureGreetd + SyncDmsGreeter
│   │   │   ├── config.go        # Embedded config.toml
│   │   │   ├── sync.go          # DMS greeter sync
│   │   │   └── memory.go        # Embedded memory.json
│   │   ├── plymouth.go          # Silent boot (Aelum theme)
│   │   ├── services.go          # systemd service enabling
│   │   ├── zram.go              # Compressed swap via zram-generator
│   │   ├── copy.go              # ISO-to-target file copying
│   │   ├── cleanup.go           # Rollback + mount cleanup
│   │   ├── primitives.go        # Run/RunChroot/WriteFile/AppendFile/CleanupMounts
│   │   ├── rollback.go          # Rollback partial installations
│   │   ├── context.go           # InstallContext
│   │   ├── checks.go            # Validation helpers
│   │   ├── root_check.go        # RootCheckResult
│   │   └── progress/
│   │       └── pacstrap.go      # PacstrapProgressTracker
│   ├── validation/
│   │   └── validation.go        # Centralized validation
│   └── shared/
│       ├── types.go             # Data structures (DiskInfo, Region, etc.)
│       ├── config.go            # Constants + thresholds
│       ├── disks.go             # lsblk parsing + fake data
│       ├── regions.go           # 28 locale/region definitions
│       ├── locales.go
│       ├── btrfs.go             # Subvolume definitions
│       ├── mirrorlist.go        # Mirrorlist handling
│       ├── mirrorlist_gen.go    # Mirrorlist refresh script
│       ├── validators.go        # Input validation
│       └── logpath.go
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

## Desktop Environment (Aelum)

The installed system includes:
- **Window Manager**: Hyprland (Wayland compositor)
- **Shell**: DankMaterialShell (based on Quickshell) + dms-shell-hyprland
- **Login Manager**: greetd + DMS greeter
- **Terminal**: Kitty
- **File Manager**: Nautilus
- **Browser**: Firefox
- **Audio**: PipeWire + WirePlumber
- **Utilities**: btop, fastfetch, brightnessctl, ddcutil, cava, matugen
- **Editor**: gnome-text-editor
- **Media**: mpv, imv
- **Fonts**: Noto (including CJK + Emoji) + Terminus + JetBrains Mono Nerd
- **Boot Splash**: Plymouth with the branded Aelum theme (silent boot; LUKS passphrase splash on encrypted installs)
