# AestheticOS Installer

[![CI](https://github.com/KornienkoDev/AestheticOS-Installer/actions/workflows/ci.yml/badge.svg)](https://github.com/KornienkoDev/AestheticOS-Installer/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.25+-blue)](https://go.dev/dl/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-yellow.svg)](https://www.gnu.org/licenses/gpl-3.0)

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
├── main.go                  # Wails app entry point (embeds frontend/dist, runs gui.NewApp)
├── README.md                # This file
├── AGENTS.md                # Agent context (architecture, conventions, rules)
├── OVERVIEW.md              # Architecture details
├── RULES.md                 # Development conventions
├── EXECUTION_FLOW.md        # Execution flow diagrams
├── CONTRIBUTING.md          # Code style, commit conventions, PR template
├── CODE_OF_CONDUCT.md       # Contributor code of conduct
├── SECURITY.md              # Vulnerability reporting policy
├── TROUBLESHOOTING.md       # Common failures, log inspection, recovery
├── CHANGELOG.md             # Version history
├── LICENSE                  # GPL-3.0
├── docs/
│   └── i18n-plan.md         # i18n migration/implementation plan
├── backend/
│   ├── cli/main.go          # CLI installer (password from stdin, never flags)
│   ├── cli/main_test.go
│   ├── gui/
│   │   ├── app.go           # Wails app struct, StartUp, OnBeforeClose
│   │   ├── bindings.go      # Wails API bindings (GetDisks, GetLanguages, GetFileSystem)
│   │   ├── installation.go  # StartInstallation (spawns CLI via sudo, reads JSON from stderr)
│   │   ├── navigation.go    # Page IDs, navigation state
│   │   ├── root_check.go    # Sudo verification
│   │   ├── validation.go    # Validate + Save methods (validate = no state mutation)
│   │   ├── root_check_test.go
│   │   └── validation_test.go
│   ├── install/
│   │   ├── installer.go     # Installer orchestrator (shared by CLI + GUI). Install() runs 7 steps.
│   │   ├── step.go          # Step interface + stepFunc + Steps()/stepWeights()
│   │   ├── context.go       # InstallContext struct (shared state, Cancel channel, Logger)
│   │   ├── command.go       # Run(), runChroot(), writeFile(), appendFile(), GetLogger()
│   │   ├── errors.go        # ValidationError, InstallError types
│   │   ├── logger.go        # Logger interface + ConsoleLogger (shared by CLI + GUI)
│   │   ├── cleanup.go       # CleanupMounts() (lazy recursive unmount)
│   │   ├── primitives/      # Low-level disk primitives (partition/format/mount helpers)
│   │   ├── format/          # FormatDisk() + setupBtrfsSubvolumes()
│   │   ├── mount/           # MountFilesystem() + verifyMount()
│   │   ├── installbase/     # base pacstrap (basePackages) with line-count progress
│   │   ├── installdesktop/  # desktop pacstrap (desktopPackages) with line-count progress
│   │   ├── users/           # SetupUsers() (useradd, chpasswd, sudoers, ISO skel copy)
│   │   ├── configure/       # ConfigureSystem (merged: fstab→bootloader→system→greetd→plymouth→zram→services)
│   │   ├── clearcache/      # ClearCache() (pacman -Scc in chroot)
│   │   ├── greetd/
│   │   │   ├── greetd.go    # Greetd configuration type
│   │   │   ├── config.go    # Config struct
│   │   │   ├── sync.go      # syncDmsGreeter() (per-user slot copies)
│   │   │   ├── memory.go    # Memory type
│   │   │   └── data/greetd/
│   │   │       ├── config.toml   # Embedded greetd config template (includes --cache-dir)
│   │   │       └── memory.json   # Embedded greetd memory template
│   │   ├── progress/
│   │   │   ├── pacstrap.go    # PacstrapProgressTracker (line-count-based progress)
│   │   │   └── pacstrap_test.go
│   │   └── *_test.go        # Unit + integration tests
│   ├── validation/
│   │   └── validation.go    # Centralized validation (delegates to shared package)
│   ├── shared/
│   │   ├── types.go         # DiskInfo, PartitionInfo, UserAccountData, DiskSelectionData
│   │   ├── config.go        # ALL constants: paths, command names, weights, thresholds, etc.
│   │   ├── disks.go         # GetDisks(), GetPartitions(), DerivePartitionPath()
│   │   ├── disks_fake.go    # fakeDisks() for frontend dev (FAKE_DISKS=1)
│   │   ├── regions.go       # 28 locale/region definitions
│   │   ├── locales.go       # Language list
│   │   ├── btrfs.go         # BtrfsSubvolume definitions + GetBtrfsSubvolumes()
│   │   ├── validators.go    # ValidateUserAccount(), ValidateDiskPath(), ValidateDiskSelection()
│   │   ├── mirrorlist.go    # Embedded mirrorlists per region
│   │   ├── mirrorlist_gen.go# Mirrorlist refresh script
│   │   ├── logpath.go       # GetLogPath()
│   │   ├── mirrorlist/      # Mirrorlist data files per locale (29 files: en_US, de_DE, etc.)
│   │   ├── disks_test.go
│   │   └── validators_test.go
│   └── scripts/
│       └── gen-ts-constants.go # Generates frontend wailsjs/go models from Go constants
├── frontend/
│   ├── src/
│   │   ├── App.svelte       # Root component (wizard page router)
│   │   ├── App.test.ts      # App component tests
│   │   ├── main.ts          # Vite entry point
│   │   ├── style.css        # Global styles
│   │   ├── vite-env.d.ts    # Vite type declarations
│   │   ├── lib/
│   │   │   ├── constants.ts       # Frontend constants (auto-generated from Go)
│   │   │   ├── navigation.ts      # Page navigation state
│   │   │   ├── store.svelte.ts    # Svelte 5 store (installState, navState, isPageValid())
│   │   │   ├── i18n/              # @svelte-intl runtime, locale registry, supported locales
│   │   │   ├── types.ts           # TypeScript type definitions
│   │   │   ├── validate.ts        # Frontend validation helpers
│   │   │   ├── validate-messages.ts # Validation message strings
│   │   │   └── validate.test.ts   # Validation tests
│   │   ├── translations/          # i18n locale JSON files (28 locales; RTL for ar-SA/he-IL)
│   │   ├── assets/                # Nunito fonts (Latin + Cyrillic), app logo
│   │   ├── components/
│   │   │   ├── AppHeader.svelte   # Top bar with page title (+ DRY RUN badge)
│   │   │   ├── AppFooter.svelte   # Bottom bar with back/next buttons (+ step dots)
│   │   │   ├── Button.svelte      # Reusable button component
│   │   │   ├── ConfirmDialog.svelte
│   │   │   └── ScrollBox.svelte   # Scrollable container component
│   │   ├── pages/
│   │   │   ├── WelcomePage.svelte         # Language/region selection
│   │   │   ├── UserAccountPage.svelte     # Username + password
│   │   │   ├── DiskSelectionPage.svelte   # Disk cards + filesystem pills
│   │   │   ├── ConfirmationPage.svelte    # Summary + root check + confirm dialog
│   │   │   ├── InstallationPage.svelte    # Progress bar + live events
│   │   │   └── FinishPage.svelte          # Success screen
│   └── wailsjs/             # Auto-generated Wails bindings
├── build/
│   └── bin/               # Built binaries (aestheticos-installer, aestheticos-installer-cli)
└── docs/
    └── i18n-plan.md         # i18n migration/implementation plan
```

## Building

### Prerequisites

- Go 1.25+
- Node.js 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- `webkit2gtk-4.1` (Linux)

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
npm run check    # TypeScript type checking
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
| `AESTHETIC_START_PAGE=1` | Select which page will be starting page |
| `FAKE_DISKS=1` | Use synthetic disk data (for frontend development) |
| `SHARED_DIR` | Copy binaries to this path after build (e.g. VM shared folder) |
| `BIN_DIR` | Copy binaries to distro airootfs bin dir after build |

### Security Note

The CLI **never accepts the password as a command-line flag**. Passwords are read from the terminal (masked) or from stdin (piped). This prevents the password from appearing in `/proc`, `ps` output, or shell history.

The GUI spawns the CLI binary via `sudo` for all privileged operations — the GUI itself never runs system commands directly.

## Installation Steps

The installer runs **7 top-level steps** (the last two merge several sub-tasks into one):

| # | Step | Weight | Description |
|---|------|--------|-------------|
| 1 | Format Disk | 3% | GPT partitioning (EFI 128MB + root), format (vfat/ext4/btrfs), btrfs subvolumes |
| 2 | Mount Filesystem | 2% | Mount root + subvolumes to `/mnt`, ESP to `/mnt/boot` |
| 3 | Install Base | 38% | `pacstrap` base packages + linux-lts (+ linux-firmware) |
| 4 | Install Desktop | 51% | `pacstrap` Hyprland, DMS greeter, Firefox, PipeWire, etc. |
| 5 | Setup Users | 2% | Create user (wheel group), set passwords, sudoers, copy ISO skel files |
| 6 | Configure System | 4% | fstab → bootloader → hostname/locale/timezone → greetd → Plymouth BGRT → zram swap → services |
| 7 | Clear Cache | 1% | `pacman -Scc` in the chroot to free disk space |

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

See [AGENTS.md](./AGENTS.md) for architecture details and conventions, [RULES.md](./RULES.md) for development conventions, [OVERVIEW.md](./OVERVIEW.md) for architecture deep-dive, and [EXECUTION_FLOW.md](./EXECUTION_FLOW.md) for execution flow diagrams.

### Key Concepts

- **Live ISO vs Installed System** — Never confuse the two. The installer runs on the live ISO; it writes to `/mnt/` (the target system).
- **CLI binary** — All privileged ops run through `aestheticos-installer-cli` (spawned via `sudo` by the GUI). See `backend/cli/main.go`.
- **InstallContext** — Shared state passed through all installation steps. See `backend/install/context.go`.
- **Step interface** — Each installation step implements `Step` (Name, Label, Run). See `backend/install/step.go`.
- **Constants in one place** — All constants live in `backend/shared/config.go`. Frontend constants are auto-generated via `backend/scripts/gen-ts-constants.go` → `frontend/src/lib/constants.ts`.

## License

GPL-3.0 — see [LICENSE](./LICENSE) for details.

## Related Projects

- [AestheticOS](https://github.com/KornienkoDev/AestheticOS) — The live ISO and distro configuration
- [DankMaterialShell](https://github.com/AvengeMedia/DankMaterialShell) — The desktop shell
- [DMS Greeter](https://github.com/AvengeMedia/dank-greeter) — The greetd greeter
