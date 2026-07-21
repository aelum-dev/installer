# AestheticOS-installer — Project Overview

## What Is This?

GUI installer for [Aesthetic OS](https://github.com/KornienkoDev/AestheticOS), a custom Arch Linux-based desktop OS shipped with Dank Material Shell + Hyprland.

There is only **one edition** — no customisation of the resulting OS is supported. The installer's priority is **maximum simplicity for new users** and **speed of installation**.

The installer is **fully offline** — no internet connection is required to install.

---

## Architecture

```
aesthetic-os-installer/
├── main.go                 # Wails app entry point (embeds frontend dist)
├── build.sh                # Build script (frontend + Wails + binaries)
├── run.sh                  # Dev runner
├── wails.json              # Wails config (900×600, fixed size)
│
├── backend/
│   ├── cli/main.go         # CLI entry point (single/full step execution)
│   ├── helper/main.go      # Privileged helper (runs via sudo)
│   ├── core/
│   │   └── installer.go    # Core orchestrator — shared by CLI & GUI
│   ├── gui/
│   │   ├── app.go          # Wails app struct + state
│   │   ├── pages.go        # Page management (6 pages)
│   │   ├── api.go          # Wails API bindings
│   │   ├── validation.go   # Form validation
│   │   └── root_check.go   # Sudo/root privilege check
│   ├── steps/              # Installation step implementations
│   │   ├── format_disk.go
│   │   ├── mount_filesystem.go
│   │   ├── install.go
│   │   ├── generate_fstab.go
│   │   ├── configure_bootloader.go
│   │   ├── setup_users.go
│   │   ├── configure_system.go
│   │   ├── cleanup.go
│   │   ├── validation.go
│   │   ├── context.go      # InstallContext (shared state)
│   │   └── logger.go
│   └── shared/
│       ├── types.go        # Go types (DiskInfo, PartitionInfo, etc.)
│       ├── validators.go   # Input validation
│       ├── disks.go        # Disk discovery
│       ├── languages.go    # Language list
│       ├── regions.go      # Timezone regions
│       ├── mirrorlist.go   # Mirrorlist management
│       └── mirrorlist_gen.go # Mirrorlist generator
│
└── frontend/              # Svelte + TypeScript (Wails frontend)
    ├── src/
    │   ├── App.svelte
    │   ├── pages/          # 6 wizard pages
    │   │   ├── WelcomePage.svelte
    │   │   ├── UserAccountPage.svelte
    │   │   ├── DiskSelectionPage.svelte
    │   │   ├── ConfirmationPage.svelte
    │   │   ├── InstallationPage.svelte
    │   │   └── FinishPage.svelte
    │   └── lib/
    │       ├── state.svelte.ts   # Svelte 5 $state
    │       ├── types.ts          # TS types + navigation constants
    │       ├── validate.ts       # Client-side validation
    │       └── navigation.ts     # Page navigation logic
    └── dist/             # Built frontend (embedded by Go)
```

---

## Key Components

| Component | Role |
|-----------|------|
| **`backend/core/installer.go`** | Single source of truth — orchestrates all 7 installation steps. Used by both GUI and CLI. |
| **`backend/helper/main.go`** | Privileged helper binary (runs via sudo) — wraps low-level commands: `sgdisk`, `mkfs.fat/ext4`, `pacstrap`, `bootctl`, `passwd`, `useradd`, `efibootmgr`, etc. |
| **`backend/gui/`** | Wails app — manages 6-page wizard state, validation, navigation |
| **`backend/cli/main.go`** | CLI tool — supports `--install` (full) or individual step flags |
| **`backend/steps/`** | 7 installation steps: Format → Mount → Install → fstab → Bootloader → Users → Configure |
| **`frontend/`** | Svelte 5 + TypeScript frontend, fixed 900×600 window |

---

## Installation Flow (7 Steps)

1. **Format Disk** — GPT partition table, 128MB EFI (FAT32) + rest ext4
2. **Mount Filesystem** — Mount root to `/mnt`, ESP to `/mnt/boot`
3. **Install Base** — `pacstrap` with embedded package list
4. **Generate fstab** — `genfstab -U /mnt > /mnt/etc/fstab`
5. **Configure Bootloader** — `bootctl install` (systemd-boot)
6. **Setup Users** — Create user + set password
7. **Configure System** — Locale, timezone, hostname, etc.

---

## Testing & Development

- **Tests:** `go test ./...` (run before building)
- **Build:** `./build.sh` (builds frontend, Wails binary, helper, CLI)
- **Dev:** `./run.sh` (Wails dev server with dry-run)
- **Shared dir:** `/home/power007/Projects/shared` ↔ VM mounted at `/shared`
- **Dry-run mode:** `--dry-run` flag or `AESTHETIC_DRY_RUN=1` env var

---

## Design Decisions

- **Helper binary pattern:** All privileged ops go through `aestheticos-helper` (sudo whitelist)
- **Partition auto-discovery:** Steps auto-guess partition paths from disk ID (`/dev/sda` → `/dev/sda1`, `/dev/sda2`)
- **Embedded packages list:** `packages.x86_64` is embedded in the binary
- **Wails app is fixed size:** 900×600, not resizable
- **No auto-commit:** Per RULES.md, commits require approval
- **Binary output:** Always to `build/bin/`, never project root
