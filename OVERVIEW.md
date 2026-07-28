# AestheticOS Installer — Project Overview

## Summary

**AestheticOS Installer** is a native desktop application for installing Arch Linux. It provides a guided wizard-style GUI built with [Wails v2](https://wails.io/) (Go + Svelte), plus a CLI mode for scripted/automated installations.

The project targets Arch Linux live ISOs, running as a privileged application that orchestrates the full installation pipeline: disk partitioning, formatting, base system installation via `pacstrap`, bootloader configuration, greetd/DMS greeter login manager setup, user setup, service enabling, and system configuration — all through a minimal, unprivileged Go helper binary invoked via `sudo`.

---

## Technology Stack

| Layer | Technology |
|-------|-----------|
| **GUI Framework** | [Wails v2](https://wails.io/) (Go backend + webview2 frontend) |
| **Backend Language** | Go 1.25 |
| **Frontend** | Svelte 5 (TypeScript, Vite, SvelteKit) |
| **Styling** | CSS custom properties (Catppuccin Latte palette) |
| **Testing (Go)** | `go test ./...` (standard Go testing) |
| **Testing (TS)** | Vitest |
| **Build** | `wails build`, `npm run build`, `go build` |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Wails Application                        │
│  (Go backend + Go-Webview2 native window)                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  GUI Layer (backend/gui/)                                 │  │
│  │  - App struct (page state, user account, disk selection)  │  │
│  │  - Wails bindings (validated by go generate)              │  │
│  │  - Validation (delegates to shared package)               │  │
│  │  - Installation orchestration (calls core.Installer)      │  │
│  │  - Root check, error logging, navigation                  │  │
│  └────────────────────────┬──────────────────────────────────┘  │
│                          │                                      │
│  ┌───────────────────────▼───────────────────────────────────┐  │
│  │  Core Layer (backend/internal/core/)                       │  │
│  │  - Installer (orchestrates full pipeline, 10 steps)        │  │
│  │  - RootCheck (verifies sudo setup)                        │  │
│  │  - Logging utilities (WriteLog)                           │  │
│  └────────────────────────┬──────────────────────────────────┘  │
│                          │                                      │
│  ┌───────────────────────▼───────────────────────────────────┐  │
│  │  Install Steps (backend/internal/install/)                 │  │
│  │  - FormatDisk, MountFilesystem, Install (pacstrap base)    │  │
│  │  - InstallDesktop (pacstrap desktop packages)              │  │
│  │  - GenerateFstab, ConfigureBootloader                      │  │
│  │  - SetupUsers, ConfigureSystem                             │  │
│  │  - ConfigureGreetd (greetd + DMS greeter)                  │  │
│  │  - EnableServices (NetworkManager, greetd)                 │  │
│  │  - Cleanup, Checks                                         │  │
│  │  - Logging (ConsoleLogger with async file writes)          │  │
│  │  - Command helpers (runChroot, writeFile)                  │  │
│  └────────────────────────┬──────────────────────────────────┘  │
│                          │                                      │
│  ┌───────────────────────▼───────────────────────────────────┐  │
│  │  Shared (backend/internal/shared/)                         │  │
│  │  - Types, Config constants, Validators                      │  │
│  │  - Disk discovery (lsblk), Regions, Btrfs subvolumes       │  │
│  │  - Mirrorlist management (embedded per-region .list files) │  │
│  └───────────────────────────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│  Frontend (frontend/src/) — Svelte 5 + TypeScript               │
│  - 6 wizard pages (Welcome → User Account → Disk → Confirmation │
│    → Installation → Finish)                                     │
│  - Reusable components (Button, Dropdown, PageLayout, etc.)     │
│  - Svelte 5 stores ($state) for form state                      │
│  - Wails runtime bindings for Go method calls                   │
├─────────────────────────────────────────────────────────────────┤
│  aestheticos-helper (backend/cmd/helper/) — Privileged binary   │
│  - Runs via sudo (NOPASSWD)                                     │
│  - Whitelisted commands: parted, format, mount, umount, run,    │
│    mountpoints, lsblk-partitions, pacstrap, genfstab,           │
│    bootctl-install, passwd, useradd, efibootmgr,                │
│    btrfs-subvol-create, btrfs-subvol-set-default,               │
│    btrfs-set-compression, udev-settle, reboot                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Binary Targets

| Binary | Purpose | Build Output |
|--------|---------|-------------|
| `aestheticos-installer` | Main GUI application (Wails) | `build/bin/aestheticos-installer` |
| `aestheticos-helper` | Privileged helper (sudo NOPASSWD) | `build/bin/aestheticos-helper` |
| `aestheticos-installer-cli` | CLI mode (same core, no GUI) | `build/bin/aestheticos-installer-cli` |

All three are copied to `/home/power007/Projects/shared` for VM testing and to the distro ISO directory (`airootfs/usr/local/bin`).

---

## Installation Pipeline (10 Steps)

The `core.Installer.Install()` method orchestrates these steps in order:

| # | Step | Function | Description |
|---|------|----------|-------------|
| 1 | **FormatDisk** | `install.FormatDisk` | Clears disk, creates GPT partition table (128MB EFI + rest root), formats with ext4 or btrfs. For btrfs: creates subvolumes (@, @home, @root, @srv, @cache, @tmp, @log) with compression and nodatacow properties. |
| 2 | **MountFilesystem** | `install.MountFilesystem` | Mounts root partition at `/mnt`, EFI at `/mnt/boot`. For btrfs, mounts each subvolume to its designated path. |
| 3 | **Install** | `install.Install` | Runs `pacstrap` to install base packages from `packages.x86_64` (base, linux-lts, linux-firmware, etc.). Minimal set for fast progress bar movement. |
| 4 | **InstallDesktop** | `install.InstallDesktop` | Runs `pacstrap` a second time for desktop environment packages (greetd, hyprland, quickshell, dms-greeter, firefox, nautilus, pipewire, etc.). Split from base to keep progress bar responsive. |
| 5 | **GenerateFstab** | `install.GenerateFstab` | Writes `/etc/fstab` with UUID-based entries. For btrfs, includes per-subvolume mount options. |
| 6 | **ConfigureBootloader** | `install.ConfigureBootloader` | Installs systemd-boot via `bootctl install`, writes boot entry referencing kernel/initrd by filename (flavor-agnostic), sets default entry, cleans orphaned ESP files, generates random seed, verifies installation. |
| 7 | **SetupUsers** | `install.SetupUsers` | Copies ISO skel to installed system, creates user account via `arch-chroot useradd`, sets password (user + root via openssl SHA-512 hash), configures sudoers for wheel group. |
| 8 | **ConfigureSystem** | `install.ConfigureSystem` | Sets hostname, timezone, locale (locale.gen + locale.conf), console keymap (vconsole.conf), copies region-specific mirrorlist. |
| 9 | **ConfigureGreetd** | `install.ConfigureGreetd` | Sets up greetd as login manager with DMS greeter: runs `systemd-sysusers`/`systemd-tmpfiles` for greeter user, writes embedded `config.toml`. |
| 10 | **EnableServices** | `install.EnableServices` | Enables systemd services: NetworkManager, greetd. |

---

## Frontend Pages (Wizard Flow)

```
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│   Welcome    │ → │ User Account │ → │Disk Selection│
│  (Language)  │   │ (Username/   │   │ (Disk + FS)  │
│              │   │  Password)   │   │              │
└──────────────┘   └──────────────┘   └──────┬───────┘
                                             │
                                             ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│    Finish    │ ← │Installation  │ ← │ Confirmation │
│  (Complete)  │   │  (Progress)  │   │  (Review +   │
│              │   │              │   │   Confirm)   │
└──────────────┘   └──────────────┘   └──────────────┘
```

| Page | ID | Validation | State |
|------|----|-----------|-------|
| Welcome | 0 | Language selection | `selectedLanguage` |
| User Account | 1 | Username ≥3 chars, password ≥6 chars, match | `userAccountData` |
| Disk Selection | 2 | Disk selected, valid filesystem | `diskSelectionData` |
| Confirmation | 3 | Review summary, root check | — |
| Installation | 4 | Auto-starts on page entry | Progress events |
| Finish | 5 | Reboot & close | — |

### Page Details

| Page | Key Features |
|------|-------------|
| **Welcome** | Dropdown language selector, auto-selects first language, "Get Started" button |
| **User Account** | Username/password/confirm fields with real-time validation, touch-state tracking |
| **Disk Selection** | Filesystem pill selector (ext4/btrfs), disk cards with HDD/SSD icon, inline tags (fstype/mounted), expandable partition table |
| **Confirmation** | Root privilege status display, installation summary (language, user, disk details), confirmation dialog (Cancel / "Yes, Erase & Install") |
| **Installation** | Progress bar with percentage + elapsed timer, dry-run badge, current step label, log file path on error |
| **Finish** | Checkmark icon, "Installation Complete" message, Reboot button |

---

## Key Design Decisions

### Helper Binary Pattern
The installer runs unprivileged and delegates all root operations to `aestheticos-helper`, invoked via `sudo` (NOPASSWD). This limits the attack surface of the GUI to a small set of whitelisted commands.

### Partition Layout
Both ext4 and btrfs use the same 2-partition layout:
- **EFI**: 128MB, FAT32, GPT type `C12A7328-F81F-11D2-BA4B-00A0C93EC93B`
- **Root**: Remaining space, GPT type `4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709`

### Btrfs Subvolume Scheme
When btrfs is selected:
| Subvolume | Mount Point | Compression | nodatacow |
|-----------|-------------|-------------|----------|
| @ | / | zstd | No |
| @home | /home | zstd | Yes |
| @root | /root | zstd | No |
| @srv | /srv | zstd | No |
| @cache | /var/cache | zstd | Yes |
| @tmp | /var/tmp | zstd | Yes |
| @log | /var/log | — | Yes |

### Bootloader Strategy
systemd-boot is configured to be **flavor-agnostic**: it discovers kernel/initrd filenames from `/mnt/boot/` at boot time, so installations work with `linux`, `linux-lts`, `linux-zen`, etc. without customizing the boot entry. The boot entry file is `aestheticos.conf` with title "Aesthetic OS".

Additional bootloader features:
- **ESP cleanup**: Removes orphaned files from previous installations
- **Random seed**: Generates entropy seed for the new system's entropy pool
- **Verification**: Post-install checks (is-installed, loader.conf, entry file, bootctl status)

### Greetd + DMS Greeter
greetd is used as the login manager with the DMS (Display Manager Shell) greeter. The greeter user is created via `systemd-sysusers`/`systemd-tmpfiles` after package install. ISO `/etc/skel` is copied to the installed system so DMS Hyprland configs are auto-copied to user homes.

### Two-Phase Pacstrap
The package installation is split into two `pacstrap` calls:
1. **Base**: Minimal system packages (base, linux-lts, firmware, utilities) — fast
2. **Desktop**: Full desktop environment (hyprland, greetd, firefox, pipewire, etc.) — slower

This keeps the progress bar moving smoothly instead of stalling during the long desktop package install.

### Dry-Run Mode
Both GUI and CLI support `--dry-run` (GUI: `AESTHETIC_DRY_RUN=1` env or `-dry-run` flag). In dry-run mode, installation steps sleep 200ms instead of executing real commands, allowing UI/testing without disk modifications.

---

## Package List (packages.x86_64)

### Base Packages (install.Install)
- **Base**: `base`, `linux-lts`, `linux-firmware`, `mkinitcpio`, `arch-install-scripts`, `mkinitcpio-archiso`
- **VM Support**: `qemu-guest-agent`
- **UEFI**: `edk2-shell`, `memtest86+-efi`
- **Utilities**: `util-linux`, `gptfdisk`, `parted`, `dosfstools`, `e2fsprogs`, `btrfs-progs`, `pv`
- **System**: `sudo`, `networkmanager`
- **Editor**: `vim`, `nano`

### Desktop Packages (install.InstallDesktop)
- **Login manager**: `greetd`, `greetd-agreety`, `greetd-dms-greeter-bin`, `uwsm`
- **Desktop environment**: `hyprland`, `quickshell`, `dms-shell-hyprland`, `cava`, `dgop`, `matugen`, `dsearch-bin`, `dankcalendar-bin`, `qt6ct-kde`, `nautilus`, `firefox`
- **Terminal & UI**: `alacritty`, `adwaita-cursors`, `webkit2gtk-4.1`
- **Utilities**: `btop`, `fastfetch`, `brightnessctl`, `ddcutil`
- **Fonts**: `noto-fonts`, `noto-fonts-cjk`, `noto-fonts-emoji`, `terminus-font`
- **GPU drivers**: `vulkan-radeon`, `vulkan-intel`, `vulkan-nouveau`
- **Audio**: `pipewire`, `pipewire-pulse`, `pipewire-alsa`, `pipewire-jack`, `wireplumber`

---

## Regions / Languages

| Code | Name | Locale | Timezone | Keymap |
|------|------|--------|----------|--------|
| en_US | English (United States) | en_US.UTF-8 | America/New_York | us |
| de_DE | Deutsch (Deutschland) | de_DE.UTF-8 | Europe/Berlin | de-latin1 |
| el_GR | Ελληνικά (Ελλάδα) | el_GR.UTF-8 | Europe/Athens | us |
| eo | Esperanto | eo.UTF-8 | Europe/Berlin | us |
| es_ES | Español (España) | es_ES.UTF-8 | Europe/Madrid | es |
| fi_FI | Suomi (Suomi) | fi_FI.UTF-8 | Europe/Helsinki | fi |
| fr_FR | Français (France) | fr_FR.UTF-8 | Europe/Paris | fr |
| hu_HU | Magyar (Magyarország) | hu_HU.UTF-8 | Europe/Budapest | hu |
| it_IT | Italiano (Italia) | it_IT.UTF-8 | Europe/Rome | it |
| nb_NO | Norsk bokmål (Norge) | nb_NO.UTF-8 | Europe/Oslo | no |
| nl_NL | Nederlands (Nederland) | nl_NL.UTF-8 | Europe/Amsterdam | us |
| pl_PL | Polski (Polska) | pl_PL.UTF-8 | Europe/Warsaw | pl |
| pt_BR | Português (Brasil) | pt_BR.UTF-8 | America/Sao_Paulo | br-abnt2 |
| ru_RU | Русский (Россия) | ru_RU.UTF-8 | Europe/Moscow | us |
| sv_SE | Svenska (Sverige) | sv_SE.UTF-8 | Europe/Stockholm | sv-latin1 |
| tr_TR | Türkçe (Türkiye) | tr_TR.UTF-8 | Europe/Istanbul | trq |
| uk_UA | Українська (Україна) | uk_UA.UTF-8 | Europe/Kyiv | us |
| bg_BG | Български (България) | bg_BG.UTF-8 | Europe/Sofia | us |
| ca_ES | Català (Catalunya) | ca_ES.UTF-8 | Europe/Madrid | es |
| ar_SA | العربية (السعودية) | ar_SA.UTF-8 | Asia/Riyadh | us |
| fa_IR | فارسی (ایران) | fa_IR.UTF-8 | Asia/Tehran | us |
| he_IL | עברית (ישראל) | he_IL.UTF-8 | Asia/Jerusalem | us |
| id_ID | Bahasa Indonesia (Indonesia) | id_ID.UTF-8 | Asia/Jakarta | us |
| ja_JP | 日本語 (日本) | ja_JP.UTF-8 | Asia/Tokyo | jp106 |
| ko_KR | 한국어 (대한민국) | ko_KR.UTF-8 | Asia/Seoul | us |
| vi_VN | Tiếng Việt (Việt Nam) | vi_VN.UTF-8 | Asia/Ho_Chi_Minh | us |
| zh_CN | 中文 (简体) | zh_CN.UTF-8 | Asia/Shanghai | us |
| zh_TW | 中文 (繁體) | zh_TW.UTF-8 | Asia/Taipei | us |

Each region has an embedded mirrorlist file for pacman. Mirrorlist files are generated at build time from the Arch Linux mirrorlist API.

---

## Test Coverage

### Go Tests
| File | Coverage |
|------|----------|
| `backend/internal/shared/validators_test.go` | User account & disk validation logic |
| `backend/internal/shared/disks_test.go` | Disk/partition discovery |
| `backend/gui/validation_test.go` | GUI validation delegation |
| `backend/gui/root_check_test.go` | Root privilege checking |
| `backend/internal/core/installer_test.go` | Installer orchestration |
| `backend/internal/install/install_test.go` | Installation step tests |

### TypeScript Tests
| File | Coverage |
|------|----------|
| `frontend/src/lib/validate.test.ts` | Frontend validation logic |
| `frontend/src/App.test.ts` | App-level tests |

---

## File Structure

```
├── main.go                          # Wails app entry point
├── wails.json                       # Wails configuration
├── go.mod / go.sum                  # Go dependencies
├── build.sh                         # Full build script
├── run.sh                           # Quick run script
├── RULES.md                         # Project rules
├── OVERVIEW.md                      # This file
│
├── backend/
│   ├── cmd/
│   │   ├── helper/main.go           # Privileged helper binary
│   │   └── installer-cli/main.go    # CLI installer entry point
│   ├── gui/
│   │   ├── app.go                   # App struct, lifecycle, reboot
│   │   ├── bindings.go              # Wails API bindings (GetDisks, etc.)
│   │   ├── installation.go          # StartInstallation orchestration
│   │   ├── logger.go                # Error log writing
│   │   ├── navigation.go            # Page state management
│   │   ├── root_check.go            # Root privilege verification
│   │   ├── root_check_test.go       # Root check tests
│   │   ├── validation.go            # GUI validation methods
│   │   └── validation_test.go       # Validation tests
│   └── internal/
│       ├── core/
│       │   ├── installer.go         # Installation orchestration (10 steps)
│       │   ├── installer_test.go    # Installer tests
│       │   ├── logging.go           # WriteLog utility
│       │   └── root_check.go        # Root check implementation
│       ├── install/
│       │   ├── install_base.go      # Base package pacstrap
│       │   ├── install_desktop.go   # Desktop package pacstrap
│       │   ├── install_test.go      # Install step tests
│       │   ├── format.go            # Partitioning & formatting
│       │   ├── mount.go             # Mount management
│       │   ├── fstab.go             # fstab generation
│       │   ├── bootloader.go        # systemd-boot configuration
│       │   ├── system.go            # System configuration (hostname, locale, etc.)
│       │   ├── users.go             # User account creation
│       │   ├── greetd.go            # greetd + DMS greeter setup
│       │   ├── services.go          # Systemd service enabling
│       │   ├── cleanup.go           # Mount cleanup
│       │   ├── checks.go            # Prerequisite validation
│       │   ├── command.go           # runChroot, writeFile helpers
│       │   ├── config.go            # Install step constants
│       │   ├── context.go           # InstallContext & Logger interface
│       │   ├── logging.go           # ConsoleLogger (async file writes)
│       │   └── data/
│       │       └── greetd/
│       │           └── config.toml  # Embedded greetd config
│       └── shared/
│           ├── types.go             # Data types (DiskInfo, etc.)
│           ├── config.go            # Constants & defaults
│           ├── validators.go        # Validation functions
│           ├── validators_test.go   # Validator tests
│           ├── disks.go             # Disk/partition discovery via lsblk
│           ├── disks_test.go        # Disk discovery tests
│           ├── locales.go           # Language list
│           ├── regions.go           # Regional configuration
│           ├── btrfs.go             # Btrfs subvolume definitions
│           ├── mirrorlist.go        # Mirrorlist download & parsing
│           ├── mirrorlist_gen.go    # Mirrorlist build-time generator
│           ├── logpath.go           # Log file path resolution
│           └── mirrorlist/          # Embedded per-region mirrorlist files
│               ├── all_https.txt    # Raw mirrorlist from Arch
│               ├── default.list     # Default worldwide mirrors
│               ├── bg_BG.list       # Per-region mirrorlists...
│               ├── de_DE.list
│               ├── el_GR.list
│               ├── en_US.list
│               ├── es_ES.list
│               ├── fa_IR.list
│               ├── fi_FI.list
│               ├── fr_FR.list
│               ├── he_IL.list
│               ├── hu_HU.list
│               ├── id_ID.list
│               ├── it_IT.list
│               ├── ja_JP.list
│               ├── ko_KR.list
│               ├── nb_NO.list
│               ├── nl_NL.list
│               ├── pl_PL.list
│               ├── pt_BR.list
│               ├── ru_RU.list
│               ├── sv_SE.list
│               ├── tr_TR.list
│               ├── uk_UA.list
│               ├── vi_VN.list
│               ├── zh_CN.list
│               ├── zh_Hans.list     # Simplified Chinese (extra)
│               ├── zh_Hant.list     # Traditional Chinese (extra)
│               └── zh_TW.list
│
└── frontend/
    ├── package.json                 # Node dependencies
    ├── vite.config.ts               # Vite configuration
    ├── svelte.config.js             # Svelte configuration
    ├── tsconfig.json                # TypeScript configuration
    ├── index.html                   # HTML entry point
    ├── AGENTS.md                    # Frontend agent instructions
    ├── src/
    │   ├── main.ts                  # App bootstrap
    │   ├── App.svelte               # Root component (page router)
    │   ├── App.test.ts              # App tests
    │   ├── style.css                # Catppuccin Latte theme
    │   ├── vite-env.d.ts            # Vite env types
    │   ├── assets/
    │   │   ├── fonts/               # Nunito font
    │   │   └── images/
    │   │       └── logo-universal.png
    │   ├── lib/
    │   │   ├── types.ts             # TypeScript types & constants
    │   │   ├── state.svelte.ts      # Svelte 5 stores ($state)
    │   │   ├── navigation.ts        # Page navigation logic
    │   │   └── validate.ts          # Frontend validation
    │   │   ├── validate.test.ts     # Validation tests
    │   ├── pages/
    │   │   ├── WelcomePage.svelte   # Language selection
    │   │   ├── UserAccountPage.svelte # User credentials form
    │   │   ├── DiskSelectionPage.svelte # Disk + filesystem selection
    │   │   ├── ConfirmationPage.svelte  # Review, root check, confirm
    │   │   ├── InstallationPage.svelte  # Progress & events
    │   │   └── FinishPage.svelte      # Completion
    │   └── components/
    │       ├── AppHeader.svelte     # Brand header (no progress bar)
    │       ├── AppFooter.svelte     # Back/Next/Reboot buttons
    │       ├── PageLayout.svelte    # Page wrapper
    │       ├── Button.svelte        # Button component
    │       ├── Dropdown.svelte      # Dropdown selector
    │       ├── ConfirmDialog.svelte # Confirmation modal
    │       └── ErrorBanner.svelte   # Error display banner
    └── wailsjs/                     # Auto-generated Wails bindings
```

---

## Build Process (`build.sh`)

1. **Refresh mirrorlists** — Downloads full Arch mirrorlist, extracts per-region files via `mirrorlist_gen.go`
2. **Build frontend** — `npm install` (if needed) → `npm run build` → assets embedded into Go binary via `//go:embed`
3. **Build GUI binary** — `wails build -clean -tags webkit2_41`
4. **Build helper** — `go build -o build/bin/aestheticos-helper ./backend/cmd/helper`
5. **Build CLI** — `go build -o build/bin/aestheticos-installer-cli ./backend/cmd/installer-cli`
6. **Copy to shared dir** — All binaries copied to `/home/power007/Projects/shared` for VM access
7. **Copy to distro** — Helper + CLI copied to `distro/airootfs/usr/local/bin/` for ISO building
