# AestheticOS Installer — Project Overview

## 1. Project Summary

**AestheticOS Installer** is a cross-platform desktop application for installing Arch Linux, built with [Wails](https://wails.io/) (Go backend + Svelte 5 frontend). It provides a guided wizard-style installer with 6 steps: Welcome → User Account → Disk Selection → Confirmation → Installation → Finish.

A companion privileged helper binary (`aestheticos-helper`) performs system operations (partitioning, formatting, mounting, pacstrap, bootctl, etc.) via `sudo`.

A CLI tool (`aestheticos-installer-cli`) provides the same installation logic via command-line flags for scripting and CI.

| Component | Language | Purpose |
|-----------|----------|---------|
| `aestheticos-installer` (GUI) | Go + Svelte 5 | Main installer wizard |
| `aestheticos-helper` | Go | Privileged system operations via sudo |
| `aestheticos-installer-cli` | Go | CLI installer (same logic as GUI) |

## 2. Architecture

```
main.go (Wails entry)
├── backend/gui/          — Wails bindings, page state, navigation, validation, installation orchestration
│   ├── app.go            — App struct, startup, reboot, dry-run
│   ├── bindings.go       — GetDisks, GetLanguages, GetFileSystem (delegate to shared)
│   ├── navigation.go     — PageID enum, GetCurrentPage, SetCurrentPage, CanGoNext/Back
│   ├── validation.go     — ValidateUserAccount, ValidateDiskSelection, Save* methods
│   ├── installation.go   — StartInstallation (creates Installer, emits progress events)
│   ├── root_check.go     — CheckRoot (verifies sudo configuration)
│   └── logger.go         — WriteErrorLog
│
├── backend/internal/core/
│   ├── installer.go      — Installer struct, Install() orchestration, RunStep, FindHelper
│   ├── root_check.go     — CheckRoot (backend logic)
│   └── logging.go        — WriteLog
│
├── backend/internal/shared/
│   ├── types.go          — DiskInfo, PartitionInfo, UserAccountData, DiskSelectionData
│   ├── config.go         — All constants (thresholds, paths, GPT GUIDs, etc.)
│   ├── validators.go     — ValidateUserAccount, ValidateDiskSelection
│   ├── disks.go          — GetDisks, GetDiskPartitions, FormatSize (lsblk parsing)
│   ├── locales.go        — GetLanguages, FindLanguage
│   ├── regions.go        — Region struct, Regions map, GetRegion
│   ├── mirrorlist.go     — GetEmbeddedMirrorList, EnsureMirrorLists (download + parse)
│   ├── btrfs.go          — BtrfsSubvolume, GetBtrfsSubvolumes, mount helpers
│   ├── logpath.go        — GetLogPath
│   └── mirrorlist_gen.go — Mirrorlist regeneration script
│
├── backend/internal/install/
│   ├── install.go        — Install() (pacstrap)
│   ├── context.go        — InstallContext, Logger interface
│   ├── format_disk.go    — FormatDisk (partitioning, formatting, btrfs subvolumes)
│   ├── mount_filesystem.go — MountFilesystem (root + EFI + btrfs subvolumes)
│   ├── bootloader.go     — ConfigureBootloader (systemd-boot via bootctl)
│   ├── config.go         — Installer-specific constants (command names, flags)
│   ├── fstab.go          — GenerateFstab (ext4 and btrfs)
│   ├── users.go          — SetupUsers (useradd, passwd, sudoers)
│   ├── system.go         — ConfigureSystem (hostname, timezone, locale, keymap, mirrors)
│   ├── checks.go         — validateMountedSystem, validatePartitionsExist
│   ├── cleanup.go        — CleanupMounts, FormatDiskErrorCleanup
│   └── logging.go        — ConsoleLogger, CommandLogger
│
├── backend/cmd/installer-cli/main.go  — CLI entry point
├── backend/cmd/helper/main.go         — Helper entry point (19 commands)
├── frontend/src/                      — Svelte 5 frontend (see below)
├── build.sh                           — Build script (mirrorlists + frontend + wails + helpers)
├── wails.json                         — Wails config
└── go.mod                             — Go module (wails v2.13.0)
```

## 3. Installation Steps (7-step pipeline)

The installer runs these steps sequentially in `core/installer.go::stepDefinitions`:

| # | Step | Function | Description |
|---|------|----------|-------------|
| 1 | **Format Disk** | `install.FormatDisk` | Partitions (EFI 128MB + root), formats (vfat/ext4/btrfs), creates btrfs subvolumes |
| 2 | **Mount Filesystem** | `install.MountFilesystem` | Mounts root + btrfs subvolumes (if btrfs) + EFI at /mnt/boot |
| 3 | **Install Base** | `install.Install` | `pacstrap /mnt` with packages from `packages.x86_64` |
| 4 | **Generate fstab** | `install.GenerateFstab` | Writes /mnt/etc/fstab (custom for btrfs subvolumes, standard for ext4) |
| 5 | **Configure Bootloader** | `install.ConfigureBootloader` | `bootctl install` + writes boot entry + cleans ESP |
| 6 | **Setup Users** | `install.SetupUsers` | `useradd`, `chpasswd`, sets root password, configures sudoers |
| 7 | **Configure System** | `install.ConfigureSystem` | hostname, timezone, locale, keymap, mirror list |

## 4. Partition Layout

Both ext4 and btrfs use the same partition layout:

| Partition | Size | Filesystem | Mount Point |
|-----------|------|------------|-------------|
| EFI | 128 MB | vfat | /boot (ESP) |
| Root | Rest of disk | ext4 or btrfs | / |

### Btrfs Subvolumes (when btrfs selected)

| Subvolume | Mount Point | Compression | nodatacow |
|-----------|-------------|-------------|-----------|
| @ | / | zstd | no |
| @home | /home | zstd | yes |
| @root | /root | zstd | no |
| @srv | /srv | zstd | no |
| @cache | /var/cache | zstd | yes |
| @tmp | /var/tmp | zstd | yes |
| @log | /var/log | none | yes |

## 5. Btrfs Mount Options

All subvolumes are mounted with `subvol=<name>,defaults,noatime,autodefrag[,compress=zstd]`. The @log subvolume has no compression.

## 6. Btrfs Top-Level Subvolume ID

The btrfs top-level subvolume ID is **5** (defined in `install/config.go::BtrfsTopLevelSubvolumeID`). This is the standard btrfs top-level subvolume ID.

## 7. Frontend (Svelte 5)

### Pages (6 total)

| PageID | Name | Component | Validation | Description |
|--------|------|-----------|------------|-------------|
| 0 | Welcome | `WelcomePage.svelte` | — | Language selector (dropdown), "Get Started" button |
| 1 | User Account | `UserAccountPage.svelte` | Username ≥ 3 chars, Password ≥ 6 chars, Passwords match | Form: username, password, confirm password |
| 2 | Disk Selection | `DiskSelectionPage.svelte` | Disk selected ≥ 8 GB, Filesystem selected (ext4/btrfs) | Disk list with partition details, filesystem radio buttons |
| 3 | Confirmation | `ConfirmationPage.svelte` | — | Summary of all selections, root privilege check, confirmation dialog |
| 4 | Installation | `InstallationPage.svelte` | — | Progress bar, step labels, elapsed time, dry-run badge |
| 5 | Finish | `FinishPage.svelte` | — | "Installation Complete" message, reboot button |

### State Management

- **Svelte 5 runes** (`$state`, `$derived`, `$effect`) — no external state library
- `state.svelte.ts` — global reactive state for userAccountData and diskSelectionData
- `validate.ts` — client-side validation with touched-state tracking (errors only show after blur)
- `navigation.ts` — goNext/goBack/finish functions; handles validation + commit via Wails bindings
- `types.ts` — TypeScript interfaces: DiskInfo, PartitionDetail, DiskSelectionData, FileSystem, PAGE_IDS

### Components

| Component | Purpose |
|-----------|---------|
| `AppHeader.svelte` | Page title, step indicator |
| `AppFooter.svelte` | Back/Next/Finish buttons |
| `Button.svelte` | Primary/secondary variants, sizes |
| `ConfirmDialog.svelte` | Yes/No confirmation overlay |
| `Dropdown.svelte` | Selectable dropdown menu |
| `ErrorBanner.svelte` | Auto-dismissing error message |
| `PageLayout.svelte` | Page wrapper with header/footer slots |

### Styling

- **Catppuccin Latte** color palette (light theme)
- CSS custom properties for all colors, spacing, typography, radii, shadows
- Fixed 900×600 window (configured in `main.go`)
- Nunito font (embedded)

## 8. Helper Binary (`aestheticos-helper`)

19 commands, all run as root via `sudo`:

| Command | Description |
|---------|-------------|
| `parted <disk>` | Clear disk + create partition table |
| `format <partition> <fstype>` | Format with wipefs |
| `mount <disk> <mountpoint>` | Mount a partition |
| `umount [-R] <mountpoint>` | Unmount (lazy, optionally recursive) |
| `mountpoints <device>` | List mountpoints for device |
| `lsblk-partitions <device>` | List partitions with mountpoints |
| `pacstrap <mountpoint> <packages>` | Install packages |
| `genfstab <mountpoint>` | Generate fstab |
| `bootctl-install <esp>` | Install systemd-boot |
| `passwd <username>` | Set user password |
| `useradd <username>` | Create user |
| `efibootmgr [list\|delete <num>]` | Manage EFI boot entries |
| `run <command> [args...]` | Run any command as root |
| `reboot` | Reboot the system |
| `btrfs-subvol-create <mountpoint> <subvol...>` | Create btrfs subvolumes |
| `btrfs-subvol-set-default <mountpoint> <subvol>` | Set default btrfs subvolume |
| `btrfs-set-compression <mountpoint> <subvol> <algorithm>` | Set btrfs compression |
| `udev-settle` | Wait for udev events |
| `parted` (standalone) | Clear disk (delegates to sgdisk -Z) |

## 9. CLI Tool (`aestheticos-installer-cli`)

### Full Installation
```bash
./aestheticos-installer-cli --install /dev/vda [--filesystem ext4|btrfs] [--dry-run]
```

### Individual Steps
```bash
./aestheticos-installer-cli --format-disk /dev/vda
./aestheticos-installer-cli --mount-filesystem /dev/vda
./aestheticos-installer-cli --install-system /dev/vda
./aestheticos-installer-cli --generate-fstab /dev/vda
./aestheticos-installer-cli --install-bootloader /dev/vda
./aestheticos-installer-cli --setup-users /dev/vda
./aestheticos-installer-cli --configure-system /dev/vda
```

### Defaults
- Username: `aesthetic` (defined in `shared/config.go::DefaultCLIUsername`)
- Password: `aesthetic` (defined in `shared/config.go::DefaultCLIPassword`)
- Hostname: `aestheticos` (defined in `shared/config.go::DefaultHostname`)
- Region: `en_US` (defined in `shared/config.go::DefaultLanguageCode`)
- Filesystem: `ext4` (passed via `--filesystem` flag, default)

## 10. Configuration Constants

All configuration lives in two files:

| File | Contents |
|------|----------|
| `shared/config.go` | Validation thresholds, filesystem types, language/region defaults, btrfs settings, mount options, time formats, path separators, file permissions, mirrorlist constants, EFI/GPT settings, virtual device filters |
| `install/config.go` | Installer-specific constants: command names (HelperCmd*...), sgdisk/wipefs/mkfs flags, umount flags, chroot commands, bootctl arguments, kernel/initrd naming, boot entry settings, heredoc delimiters, btrfs setup, logging, simulate duration |

## 11. Key Design Decisions

### Helper Binary Architecture
The helper binary (`aestheticos-helper`) is the privileged bridge between the unprivileged GUI/CLI and system operations. It is invoked via `sudo` and provides a command dispatcher pattern — each operation is a named command with its own argument parsing. This approach:
- Keeps the main installer binary unprivileged
- Provides a clear audit trail of what commands are available
- Makes sudoers configuration simple (just one binary to allow)

### Partition Table Refresh Strategy
`FormatDisk` calls `refreshPartitionTable` (partprobe → partx -a fallback) multiple times: before creating partitions, after creating partitions, after formatting, and before discovering UUIDs. The comment in the code notes this is "critical" — without it, the kernel keeps old device nodes and new partitions never get their device nodes, causing "device is mounted" errors.

### Btrfs Subvolume Management
Btrfs subvolumes are created by mounting the top-level subvolume (subvolid=5) to a temporary location, creating each subvolume, setting @ as the default, applying per-subvolume properties (compression via `btrfs property set`, nodatacow via `chattr +C`), then unmounting. This mirrors the approach used by archinstall.

### Mirrorlist Handling
Mirrorlists are embedded in the binary via `//go:embed`. At build time, `mirrorlist_gen.go` downloads the full Arch mirrorlist from archlinux.org, parses it by country, and extracts per-region files (en_US, ru_RU, uk_UA). The embedded mirrorlists are then used at runtime by `ConfigureSystem` to write `/etc/pacman.d/mirrorlist`.

### State Management
The frontend uses Svelte 5 runes (`$state`, `$derived`, `$effect`) with no external state library. State is shared between pages via `$lib/state.svelte.ts`. Validation uses a "touched" pattern — errors only appear after the user has interacted with a field (blur event).

### Event-Based Communication
The frontend and backend communicate via Wails events (`installation:progress`, `installation:complete`, `installation:error`) emitted from the Go goroutine running the installation. The frontend listens with `EventsOn` and updates local state accordingly.

### Error Handling
Installation errors are written to `installation.log` in the same directory as the executable. The `WriteErrorLog` function in `gui/logger.go` appends timestamped error details. The log path is displayed on the InstallationPage when an error occurs.

### Dry-Run Mode
Dry-run mode is enabled via `--dry-run` flag or `AESTHETIC_DRY_RUN=1` environment variable. In dry-run, `simulateStep()` sleeps for 200ms per step (defined in `install/config.go::SimulateStepSleepDuration`) instead of executing real commands. All UI labels are prefixed with `[DRY RUN]`.

## 12. Supported Regions/Languages

| Code | Name | Locale | Timezone | Keymap |
|------|------|--------|----------|--------|
| en_US | English (United States) | en_US.UTF-8 | America/New_York | us |
| ru_RU | Русский (Россия) | ru_RU.UTF-8 | Europe/Moscow | ru |
| uk_UA | Українська (Україна) | uk_UA.UTF-8 | Europe/Kyiv | ua |

## 13. Testing

- `backend/gui/root_check_test.go` — Root check tests
- `backend/gui/validation_test.go` — Validation tests
- `backend/internal/shared/validators_test.go` — Shared validator tests
- `backend/internal/shared/disks_test.go` — Disk detection tests
- `backend/internal/install/install_test.go` — Installation tests
- `backend/internal/install/layout_integration_test.go` — Layout integration tests
- `frontend/src/App.test.ts` — Frontend tests
- `frontend/src/lib/validate.test.ts` — Validation tests

## 14. Build Process (`build.sh`)

1. **Refresh mirrorlists** — Run `mirrorlist_gen.go` to download and parse Arch mirrorlist
2. **Build frontend** — `npm install` (if needed) + `npm run build`
3. **Build with Wails** — `wails build -clean -tags webkit2_41`
4. **Build helper** — `go build -o build/bin/aestheticos-helper ./backend/cmd/helper`
5. **Build CLI** — `go build -o build/bin/aestheticos-installer-cli ./backend/cmd/installer-cli`
6. **Copy to shared** — Copy all three executables to `$HOME/Projects/shared/`

## 15. TODOs and Known Issues

- `navigation.go::CanGoNext()` — Validation is disabled with a TODO: "Re-enable validation once UI forms are implemented." The function currently always returns true.
- `CanGoNext` validation is bypassed — the GUI calls `ValidateUserAccount`/`ValidateDiskSelection` but the results are not checked before `SetCurrentPage`.
- Default CLI password is `aesthetic` — the comment says "CHANGE THIS before any production use."
