# AestheticOS Installer

[![CI](https://github.com/KornienkoDev/AestheticOS-Installer/actions/workflows/ci.yml/badge.svg)](https://github.com/KornienkoDev/AestheticOS-Installer/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.25+-blue)](https://go.dev/dl/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-yellow.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/KornienkoDev/AestheticOS-Installer)](https://goreportcard.com/report/github.com/KornienkoDev/AestheticOS-Installer)

A graphical Arch Linux installer with a polished UI, built with [Wails v2](https://wails.io/) (Go + Svelte 5).

Paired with the [AestheticOS](https://github.com/KornienkoDev/AestheticOS) distro — a fully offline Arch Linux ISO that requires zero internet during installation.

## Features

- **6-step wizard** — Welcome, User Account, Disk Selection, Confirmation, Installation, Finish
- **ext4 and btrfs** — Choose your root filesystem; btrfs includes automatic subvolume layout (`@`, `@home`, `@srv`, `@cache`, `@tmp`, `@log`, `@swap`)
- **CLI mode** — Headless automated installs via `aestheticos-installer-cli` (spawns via sudo from GUI)
- **Progress tracking** — Line-count-based pacstrap progress for accurate percentages
- **Dry-run mode** — Simulate the full install for UI testing (`AESTHETIC_DRY_RUN=1`)
- **Fake disks** — Synthetic disk data for frontend development (`FAKE_DISKS=1`)
- **Zram swap** — Compressed RAM-based swap, size auto-detected from live ISO RAM (2/4/8 GiB tiered)

## Desktop Environment

The installed system ships with:

| Component | Package |
|-----------|---------|
| Window Manager | Hyprland |
| Shell | DankMaterialShell (Quickshell-based) |
| Login Manager | greetd + DMS greeter |
| Terminal | Alacritty |
| Browser | Firefox |
| Audio | PipeWire + WirePlumber |
| Boot Splash | Plymouth BGRT (silent boot with firmware logo) |

## Project Structure

```
AestheticOS-Installer/
├── main.go                  # Wails app entry point
├── wails.json               # Wails config
├── build.sh / run.sh        # Build & dev scripts
├── backend/
│   ├── cli/main.go          # CLI installer
│   ├── gui/                 # Wails GUI layer (bindings, navigation, validation)
│   ├── install/             # Installation steps + orchestrator
│   │   ├── installer.go     # Installer struct + Install()/RunStep()/Rollback()
│   │   ├── step.go          # Step interface + step implementations
│   │   ├── plan.go          # Plan struct for step weights/percentages
│   │   ├── executor.go      # Executor interface (Run, RunChroot, WriteFile, AppendFile)
│   │   ├── configuration.go # Merged ConfigureSystem step
│   │   ├── errors.go        # ValidationError, InstallError types
│   │   ├── logger.go        # Logger interface + ConsoleLogger
│   │   ├── format.go        # Partitioning + formatting
│   │   ├── mount.go         # Mount filesystem + verification
│   │   ├── install_base.go  # pacstrap base packages
│   │   ├── install_desktop.go # pacstrap desktop packages
│   │   ├── install.go       # Pacstrap progress tracking
│   │   ├── fstab.go         # fstab generation + verification
│   │   ├── bootloader.go    # systemd-boot install + boot entries
│   │   ├── users.go         # User account creation + ISO copy
│   │   ├── system.go        # Hostname, locale, timezone, mirrors
│   │   ├── greetd.go        # Login manager config + DMS sync
│   │   ├── greetd/          # greetd package (config, sync, memory)
│   │   ├── plymouth.go      # Silent boot (BGRT theme)
│   │   ├── services.go      # systemd service enabling
│   │   ├── zram.go          # Compressed swap via zram-generator
│   │   ├── copy.go          # ISO-to-target file copying
│   │   ├── cleanup.go       # Mount cleanup
│   │   ├── command.go       # Run(), runChroot(), writeFile(), appendFile()
│   │   ├── rollback.go      # Rollback partial installations
│   │   ├── context.go       # InstallContext
│   │   ├── checks.go        # Validation helpers
│   │   └── progress/        # Pacstrap progress tracker
│   ├── validation/          # Centralized validation (delegates to shared)
│   └── shared/              # Types, validators, disk detection, regions
│       ├── types.go         # Data structures (DiskInfo, Region, etc.)
│       ├── config.go        # Constants + thresholds
│       ├── disks.go         # lsblk parsing + fake data
│       ├── regions.go       # 28 locale/region definitions
│       ├── locales.go
│       ├── btrfs.go         # Subvolume definitions
│       ├── mirrorlist.go    # Mirrorlist handling
│       ├── mirrorlist_gen.go # Mirrorlist refresh script
│       ├── validators.go    # Input validation
│       └── logpath.go
└── frontend/
    ├── src/
    │   ├── App.svelte       # Root component
    │   ├── main.ts          # Entry point
    │   ├── lib/
    │   │   ├── state.svelte.ts  # Svelte stores
    │   │   ├── types.ts         # TS types + constants
    │   │   ├── navigation.ts    # Nav + validation calls
    │   │   └── validate.ts      # Client validation
    │   ├── pages/           # 6 wizard pages
    │   └── components/      # Reusable UI components
    └── wailsjs/             # Auto-generated Wails bindings
```

## Building

### Prerequisites

- Go 1.25+
- Node.js 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- `webkit2gtk-4.1` (Linux) or `WebView2` (Windows)

### Build all binaries

```bash
./build.sh
```

This produces two binaries in `build/bin/`:

| Binary | Purpose |
|--------|---------|
| `aestheticos-installer` | GUI wizard (Wails app) |
| `aestheticos-installer-cli` | CLI automated installer |

The GUI spawns the CLI binary via `sudo` for all privileged operations — no separate helper binary is needed.

### Development mode

```bash
./run.sh
```

Starts the Wails dev server with hot-reload for both frontend and backend.

### Frontend only

```bash
cd frontend
npm install
npm run dev      # Dev server
npm run build    # Production build
npm run test     # Run Vitest tests
```

## Usage

### GUI

```bash
# Run as root (GUI spawns CLI via sudo for privileged ops)
sudo ./build/bin/aestheticos-installer
```

### CLI

```bash
# Full installation (password entered interactively)
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda

# With btrfs
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda --filesystem btrfs

# Dry run (simulate, no changes)
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda --dry-run

# Non-interactive: pipe password via stdin
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda <<< 'mypassword'

# Rollback a partial installation (cleans up mounts + wipes partition signatures)
sudo ./build/bin/aestheticos-installer-cli --rollback /dev/vda

# Single step
sudo ./build/bin/aestheticos-installer-cli --format-disk /dev/vda
sudo ./build/bin/aestheticos-installer-cli --mount-filesystem /dev/vda
sudo ./build/bin/aestheticos-installer-cli --install-system /dev/vda
sudo ./build/bin/aestheticos-installer-cli --setup-users /dev/vda
sudo ./build/bin/aestheticos-installer-cli --configure-system /dev/vda
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `AESTHETIC_DRY_RUN=1` | Simulate installation (steps sleep 200ms instead of executing) |
| `FAKE_DISKS=1` | Use synthetic disk data (for frontend development) |
| `SHARED_DIR` | Copy binaries to this path after build (e.g. VM shared folder) |
| `BIN_DIR` | Copy binaries to distro airootfs bin dir after build |

### Security Note

The CLI **never accepts the password as a command-line flag**. Passwords are read from the terminal (masked) or from stdin (piped). This prevents the password from appearing in `/proc`, `ps` output, or shell history.

The GUI spawns the CLI binary via `sudo` for all privileged operations — the GUI itself never runs system commands directly.

## Installation Steps

The installer runs **6 top-level steps** (12 logical sub-steps). The last step merges 7 configuration tasks into one:

| # | Step | Weight | Description |
|---|------|--------|-------------|
| 1 | Format Disk | 3% | GPT partitioning (EFI 128MB + root), format (vfat/ext4/btrfs), btrfs subvolumes |
| 2 | Mount Filesystem | 2% | Mount root + subvolumes to `/mnt`, ESP to `/mnt/boot` |
| 3 | Install Base | 45% | `pacstrap` base packages + linux-lts |
| 4 | Install Desktop | 35% | `pacstrap` Hyprland, DMS greeter, Firefox, PipeWire, etc. |
| 5 | Setup Users | 2% | Create user (wheel group), set passwords, sudoers, copy ISO skel files |
| 6 | Configure System | 13% | fstab → bootloader → hostname/locale/timezone → greetd → Plymouth BGRT → zram swap → services |

**Step 6 sub-steps:** Generate fstab, configure systemd-boot bootloader, set hostname/timezone/locale/keymap/mirrors, configure greetd + DMS greeter, configure Plymouth BGRT silent boot, configure zram compressed swap, enable systemd services (NetworkManager, greetd, bluetooth).

## Idempotency & Rollback

The installer is **idempotent** — if it fails mid-install, running it again will clean up and start fresh:

- Mounts are automatically torn down on any failure
- The `--rollback` flag manually cleans up a partial installation:
  ```bash
  sudo ./aestheticos-installer-cli --rollback /dev/vda
  ```
  This unmounts everything under `/mnt`, wipes filesystem signatures from both partitions, and re-probes the partition table. The disk is then ready for a fresh install.

## Security Design

The installer uses a **CLI-spawn pattern** for privileged operations:

- The GUI runs as root and spawns `aestheticos-installer-cli` via `sudo`
- The CLI binary handles all privileged operations directly via `exec.Command` and `arch-chroot`
- No separate helper binary is needed — the CLI is the privileged component
- Passwords are read from stdin (never CLI flags) to avoid `/proc`, `ps`, and shell history exposure
- The CLI emits JSON progress events to stderr; the GUI reads these to update the UI

### Sudoers Configuration

On the live ISO, the user must have passwordless sudo for the CLI binary. On the installed system, `/etc/sudoers.d/wheel` is written for the new user:

```
%wheel ALL=(ALL:ALL) ALL
```

## Testing

```bash
# Run all Go tests (unit + integration)
go test ./...

# Run frontend tests
cd frontend && npm run test

# Type checking
cd frontend && npm run check
```

Integration tests verify the full install flow in dry-run mode (no real commands executed) — step ordering, progress tracking, error handling, and rollback.

## Development

See [RULES.md](./RULES.md) for development conventions and [OVERVIEW.md](./OVERVIEW.md) for architecture details.

### Key Concepts

- **Live ISO vs Installed System** — Never confuse the two. The installer runs on the live ISO; it writes to `/mnt/` (the target system).
- **CLI binary** — All privileged ops run through `aestheticos-installer-cli` (spawned via `sudo` by the GUI). See `backend/cli/main.go`.
- **InstallContext** — Shared state passed through all installation steps. See `backend/install/context.go`.
- **Step interface** — Each installation step implements `Step` (Name, Label, Run). See `backend/install/step.go`.

## License

GPL-3.0 — see [LICENSE](./LICENSE) for details.

## Related Projects

- [AestheticOS](https://github.com/KornienkoDev/AestheticOS) — The live ISO and distro configuration
- [DankMaterialShell](https://github.com/maxshutyning/dms) — The desktop shell
- [DMS Greeter](https://github.com/maxshutyning/dms-greeter) — The greetd greeter
