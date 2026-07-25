# AestheticOS Installer — Project Overview

## Summary

**AestheticOS Installer** is a native desktop application for installing Arch Linux. It provides a guided wizard-style GUI built with [Wails v2](https://wails.io/) (Go + Svelte), plus a CLI mode for scripted/automated installations.

The project targets Arch Linux live ISOs, running as a privileged application that orchestrates the full installation pipeline: disk partitioning, formatting, base system installation via `pacstrap`, bootloader configuration, user setup, and system configuration — all through a minimal, unprivileged Go helper binary invoked via `sudo`.

---

## Technology Stack

| Layer | Technology |
|-------|-----------|
| **GUI Framework** | [Wails v2](https://wails.io/) (Go backend + webview frontend) |
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
│  └────────────────────────┬──────────────────────────────────┘  │
│                          │                                      │
│  ┌───────────────────────▼───────────────────────────────────┐  │
│  │  Core Layer (backend/internal/core/)                       │  │
│  │  - Installer (orchestrates full pipeline)                  │  │
│  │  - RootCheck (verifies sudo setup)                        │  │
│  │  - Logging utilities                                      │  │
│  └────────────────────────┬──────────────────────────────────┘  │
│                          │                                      │
│  ┌───────────────────────▼───────────────────────────────────┐  │
│  │  Install Steps (backend/internal/install/)                 │  │
│  │  - FormatDisk, MountFilesystem, Install (pacstrap)         │  │
│  │  - GenerateFstab, ConfigureBootloader                      │  │
│  │  - SetupUsers, ConfigureSystem                             │  │
│  │  - Cleanup, Checks                                         │  │
│  └────────────────────────┬──────────────────────────────────┘  │
│                          │                                      │
│  ┌───────────────────────▼───────────────────────────────────┐  │
│  │  Shared (backend/internal/shared/)                         │  │
│  │  - Types, Config constants, Validators                      │  │
│  │  - Disk discovery (lsblk), Regions, Btrfs subvolumes       │  │
│  │  - Mirrorlist management                                   │  │
│  └───────────────────────────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│  Frontend (frontend/src/) — Svelte 5 + TypeScript               │
│  - 6 wizard pages (Welcome → User Account → Disk → Confirmation │
│    → Installation → Finish)                                     │
│  - Reusable components (Button, Dropdown, PageLayout, etc.)     │
│  - Svelte store for form state                                  │
│  - Wails runtime bindings for Go method calls                   │
├─────────────────────────────────────────────────────────────────┤
│  aestheticos-helper (backend/cmd/helper/) — Privileged binary   │
│  - Runs via sudo (NOPASSWD)                                     │
│  - Whitelisted commands: parted, format, mount, umount, run,    │
│    pacstrap, bootctl, btrfs, udev, useradd, passwd, etc.        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Binary Targets

| Binary | Purpose | Build Output |
|--------|---------|-------------|
| `aestheticos-installer` | Main GUI application (Wails) | `build/bin/aestheticos-installer` |
| `aestheticos-helper` | Privileged helper (sudo NOPASSWD) | `build/bin/aestheticos-helper` |
| `aestheticos-installer-cli` | CLI mode (same core, no GUI) | `build/bin/aestheticos-installer-cli` |

All three are copied to `/home/power007/Projects/shared` for VM testing.

---

## Installation Pipeline (7 Steps)

The `core.Installer.Install()` method orchestrates these steps in order:

| # | Step | Function | Description |
|---|------|----------|-------------|
| 1 | **FormatDisk** | `install.FormatDisk` | Clears disk, creates GPT partition table (128MB EFI + rest root), formats with ext4 or btrfs. For btrfs: creates subvolumes (@, @home, @root, @srv, @cache, @tmp, @log) with compression and nodatacow properties. |
| 2 | **MountFilesystem** | `install.MountFilesystem` | Mounts root partition at `/mnt`, EFI at `/mnt/boot`. For btrfs, mounts each subvolume to its designated path. |
| 3 | **Install** | `install.Install` | Runs `pacstrap` to install base packages from `packages.x86_64` (base, linux-lts, hyprland, etc.) |
| 4 | **GenerateFstab** | `install.GenerateFstab` | Writes `/etc/fstab` with UUID-based entries. For btrfs, includes per-subvolume mount options. |
| 5 | **ConfigureBootloader** | `install.ConfigureBootloader` | Installs systemd-boot via `bootctl install`, writes boot entry referencing kernel/initrd by filename (flavor-agnostic), sets default entry, cleans orphaned ESP files. |
| 6 | **SetupUsers** | `install.SetupUsers` | Creates user account via `arch-chroot useradd`, sets password, configures sudoers for wheel group, sets root password. |
| 7 | **ConfigureSystem** | `install.ConfigureSystem` | Sets hostname, timezone, locale, console keymap, copies region-specific mirrorlist. |

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
| Disk Selection | 2 | Disk selected, ≥8GB, valid filesystem | `diskSelectionData` |
| Confirmation | 3 | Review summary, root check | — |
| Installation | 4 | Auto-starts on page entry | Progress events |
| Finish | 5 | Reboot & close | — |

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
|-----------|-------------|-------------|-----------|
| @ | / | zstd | No |
| @home | /home | zstd | Yes |
| @root | /root | zstd | No |
| @srv | /srv | zstd | No |
| @cache | /var/cache | zstd | Yes |
| @tmp | /var/tmp | zstd | Yes |
| @log | /var/log | — | Yes |

### Bootloader Strategy
systemd-boot is configured to be **flavor-agnostic**: it discovers kernel/initrd filenames from `/mnt/boot/` at boot time, so installations work with `linux`, `linux-lts`, `linux-zen`, etc. without customizing the boot entry.

### Dry-Run Mode
Both GUI and CLI support `--dry-run` (GUI: `AESTHETIC_DRY_RUN=1` env or `-dry-run` flag). In dry-run mode, installation steps sleep 200ms instead of executing real commands, allowing UI/testing without disk modifications.

---

## Package List (packages.x86_64)

The installed system includes:

- **Base**: `base`, `linux-lts`, `linux-firmware`, `mkinitcpio`, `arch-install-scripts`, `mkinitcpio-archiso`
- **VM Support**: `qemu-guest-agent`
- **UEFI**: `edk2-shell`, `memtest86+-efi`
- **Utilities**: `util-linux`, `gptfdisk`, `parted`, `dosfstools`, `e2fsprogs`, `btrfs-progs`, `pv`
- **System**: `sudo`
- **Editor**: `vim`, `nano`
- **Desktop (Hyprland)**: `hyprland`, `btop`, `fastfetch`, `noto-fonts`, `adwaita-cursors`, `alacritty`, `webkit2gtk-4.1`
- **GPU Drivers**: `vulkan-radeon`, `vulkan-intel`, `vulkan-nouveau`

---

## Regions / Languages

| Code | Name | Locale | Timezone | Keymap |
|------|------|--------|----------|--------|
| en_US | English (United States) | en_US.UTF-8 | America/New_York | us |
| de_DE | Deutsch (Deutschland) | de_DE.UTF-8 | Europe/Berlin | de |
| el_GR | Ελληνικά (Ελλάδα) | el_GR.UTF-8 | Europe/Athens | gr |
| eo | Esperanto | eo.UTF-8 | Europe/Berlin | us |
| es_ES | Español (España) | es_ES.UTF-8 | Europe/Madrid | es |
| fi_FI | Suomi (Suomi) | fi_FI.UTF-8 | Europe/Helsinki | fi |
| fr_FR | Français (France) | fr_FR.UTF-8 | Europe/Paris | fr |
| hu_HU | Magyar (Magyarország) | hu_HU.UTF-8 | Europe/Budapest | hu |
| it_IT | Italiano (Italia) | it_IT.UTF-8 | Europe/Rome | it |
| nb_NO | Norsk bokmål (Norge) | nb_NO.UTF-8 | Europe/Oslo | no |
| nl_NL | Nederlands (Nederland) | nl_NL.UTF-8 | Europe/Amsterdam | nl |
| pl_PL | Polski (Polska) | pl_PL.UTF-8 | Europe/Warsaw | pl |
| pt_BR | Português (Brasil) | pt_BR.UTF-8 | America/Sao_Paulo | br-abnt2 |
| ru_RU | Русский (Россия) | ru_RU.UTF-8 | Europe/Moscow | ru |
| sv_SE | Svenska (Sverige) | sv_SE.UTF-8 | Europe/Stockholm | sv |
| tr_TR | Türkçe (Türkiye) | tr_TR.UTF-8 | Europe/Istanbul | tr |
| uk_UA | Українська (Україна) | uk_UA.UTF-8 | Europe/Kyiv | ua |
| bg_BG | Български (България) | bg_BG.UTF-8 | Europe/Sofia | bg |
| ca_ES | Català (Catalunya) | ca_ES.UTF-8 | Europe/Madrid | es |
| ar | العربية | ar.UTF-8 | Asia/Baghdad | ar |
| fa_IR | فارسی (ایران) | fa_IR.UTF-8 | Asia/Tehran | ir |
| he_IL | עברית (ישראל) | he_IL.UTF-8 | Asia/Jerusalem | il |
| id_ID | Bahasa Indonesia (Indonesia) | id_ID.UTF-8 | Asia/Jakarta | id |
| ja_JP | 日本語 (日本) | ja_JP.UTF-8 | Asia/Tokyo | jp |
| ko_KR | 한국어 (대한민국) | ko_KR.UTF-8 | Asia/Seoul | kr |
| vi_VN | Tiếng Việt (Việt Nam) | vi_VN.UTF-8 | Asia/Ho_Chi_Minh | vn |
| zh_CN | 中文 (简体) | zh_CN.UTF-8 | Asia/Shanghai | us |
| zh_TW | 中文 (繁體) | zh_TW.UTF-8 | Asia/Taipei | us |

Each region has an embedded mirrorlist file for pacman.

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
├── RULES.md                         # Project rules
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
│   │   └── validation.go            # GUI validation methods
│   └── internal/
│       ├── core/
│       │   ├── installer.go         # Installation orchestration
│       │   ├── logging.go           # Log utilities
│       │   └── root_check.go        # Root check implementation
│       ├── install/
│       │   ├── install.go           # pacstrap installation
│       │   ├── format.go            # Partitioning & formatting
│       │   ├── mount.go           # Mount management
│       │   ├── fstab.go             # fstab generation
│       │   ├── bootloader.go        # systemd-boot configuration
│       │   ├── system.go            # System configuration (hostname, locale, etc.)
│       │   ├── users.go             # User account creation
│       │   ├── cleanup.go           # Mount cleanup
│       │   ├── checks.go            # Prerequisite validation
│       │   ├── config.go            # Install step constants
│       │   ├── context.go           # InstallContext & Logger interface
│       │   ├── logging.go           # ConsoleLogger implementation
│       │   ├── data/
│       │   │   └── packages.x86_64  # Package list
│       └── shared/
│           ├── types.go             # Data types (DiskInfo, etc.)
│           ├── config.go            # Constants & defaults
│           ├── validators.go        # Validation functions
│           ├── disks.go             # Disk/partition discovery via lsblk
│           ├── locales.go           # Language list
│           ├── regions.go           # Regional configuration
│           ├── btrfs.go             # Btrfs subvolume definitions
│           ├── mirrorlist.go        # Mirrorlist download & parsing
│           ├── mirrorlist_gen.go    # Mirrorlist build-time generator
│           └── logpath.go           # Log file path resolution
│
└── frontend/
    ├── package.json                 # Node dependencies
    ├── vite.config.ts               # Vite configuration
    ├── svelte.config.js             # Svelte configuration
    ├── index.html                   # HTML entry point
    ├── src/
    │   ├── main.ts                  # App bootstrap
    │   ├── App.svelte               # Root component (page router)
    │   ├── style.css                # Catppuccin Latte theme
    │   ├── lib/
    │   │   ├── types.ts             # TypeScript types & constants
    │   │   ├── state.svelte.ts      # Svelte 5 stores ($state)
    │   │   ├── navigation.ts        # Page navigation logic
    │   │   └── validate.ts          # Frontend validation
    │   ├── pages/
    │   │   ├── WelcomePage.svelte   # Language selection
    │   │   ├── UserAccountPage.svelte # User credentials form
    │   │   ├── DiskSelectionPage.svelte # Disk + filesystem selection
    │   │   ├── ConfirmationPage.svelte  # Review & confirm
    │   │   ├── InstallationPage.svelte  # Progress & events
    │   │   └── FinishPage.svelte      # Completion
    │   └── components/
    │       ├── AppHeader.svelte     # Wizard progress header
    │       ├── AppFooter.svelte     # Back/Next buttons
    │       ├── PageLayout.svelte    # Page wrapper
    │       ├── Button.svelte        # Button component
    │       ├── Dropdown.svelte      # Dropdown selector
    │       ├── ConfirmDialog.svelte # Confirmation modal
    │       └── ErrorBanner.svelte   # Error display banner
    └── wailsjs/                     # Auto-generated Wails bindings
```

---

## Build Process (`build.sh`)

1. **Refresh mirrorlists** — Downloads full Arch mirrorlist, extracts per-region files
2. **Build frontend** — `npm install` (if needed) → `npm run build` → assets embedded into Go binary via `//go:embed`
3. **Build GUI binary** — `wails build -clean -tags webkit2_41`
4. **Build helper** — `go build -o build/bin/aestheticos-helper ./backend/cmd/helper`
5. **Build CLI** — `go build -o build/bin/aestheticos-installer-cli ./backend/cmd/installer-cli`
6. **Copy to shared dir** — All binaries copied to `/home/power007/Projects/shared` for VM access

---

## Testing Environment

- **Host**: Development machine where the installer is built and tested
- **VM**: QEMU virtual machine booting from the AestheticOS ISO
- **Shared directory**: `/home/power007/Projects/shared` (host) ↔ `/shared` (VM)
- Binaries are placed in the shared directory and mounted into the VM for testing

---

## Dependencies

- `github.com/wailsapp/wails/v2` — GUI framework (Go + webview2)
- `svelte` v5.55 — Frontend framework
- `vite` v6.3 — Frontend build tool
- `vitest` — TypeScript testing
- Standard Go 1.25 standard library for all system operations

No external Go dependencies beyond Wails itself — all system operations (lsblk, sgdisk, mkfs, pacstrap, bootctl, etc.) are executed via `exec.Command` through the helper binary.
