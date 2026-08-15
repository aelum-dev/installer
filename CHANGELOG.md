# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **Frontend store refactored** — `$state.svelte.ts` renamed to `store.svelte.ts` with Svelte 5 runes. Installation state and navigation state consolidated into `installState` and `navState` objects. `isPageValid()` derived function added for page-level validation.
- **Layout components removed** — `Dropdown.svelte`, `ErrorBanner.svelte`, and `PageLayout.svelte` deleted. Layout styles moved to `lib/layout.css` for shared use across pages.
- **Navigation updated** — All navigation functions now reference the new store (`store.svelte.ts`). Confirmation dialog tracking added with `confirmInstallation()`/`cancelInstallation()` helpers.

### Added
- **AppDataDir constant** — Backend constant for XDG data directory path (`~/.local/share/aestheticos-installer`).
- **Full internationalization (i18n)** — All UI strings are now translated via `@svelte-intl/svelte-intl`. Supports 16 locales (en-US, de-DE, es-ES, fr-FR, it-IT, pt-BR, ru-RU, zh-CN, zh-TW, ja-JP, ko-KR, uk-UA, pl-PL, tr-TR, ar-SA, he-IL). German translation shipped. RTL support for Arabic and Hebrew.
- **Auto locale detection** — Installer auto-detects OS locale on first launch and persists selection via backend `GetUILocale()`/`SetUILocale()`.
- **Cache clearing step** — `pacman -Scc` runs in the chroot after system configuration to clear the package cache and free disk space.

### Changed
- **Root check moved to app startup** — `CheckRoot()` now runs in `StartUp()` instead of `StartInstallation()`. If sudo is not available, dry-run mode is enabled automatically so the user can still explore the installer. A "DRY RUN" badge appears in the header on all pages.
- **Fake disks loaded in dry-run mode** — When dry-run is active, `GetDisks()` and `GetDiskPartitions()` return synthetic disk data instead of querying `lsblk`. This lets the UI be explored without reading real block devices.

### Fixed
- **WelcomePage layout no longer shifts when switching languages** — Uses CSS Grid with fixed 56px subtitle row and integer pixel heights (48px title, 56px subtitle) to prevent font metric differences from causing layout shifts. Language list scrolls independently within its max-height.
- **Bootloader config works when booted via GRUB/Ventoy** — Replaced `bootctl set-default` with direct `loader.conf` file write. `bootctl set-default` fails when the live ISO is booted through GRUB (e.g. Ventoy) because it checks current firmware EFI variables. The file write achieves the same result regardless of boot method.
- **Dry-run mode now properly skips all side effects** — `--cleanup` and `--rollback` CLI flags now skip operations in dry-run mode. `OnBeforeClose` in the GUI skips the sudo cleanup call in dry-run mode. Previously these would still execute disk operations even with `--dry-run`.
- **GUI transitions to FinishPage after successful installation** — `StartInstallation()` now emits `installation:complete` (success) or `installation:error` (failure) events to the frontend when the CLI exits. Previously the frontend listened for these events but they were never emitted, leaving the UI stuck at the last step.
- **Ctrl+C during GUI installation now triggers cleanup** — `CancelInstallation()` was sending SIGINT *and* canceling the `exec.CommandContext`, which SIGKILL'd the CLI before its `defer RunCleanup()` could unmount `/mnt`. Now only SIGINT is sent, letting the CLI's own signal handler run the cleanup defer chain. A 10s safety timeout in the GUI signal handler force-unmounts `/mnt` if the CLI ever fails to exit.
- **Logging system rewritten** — Terminal output is now byte-identical to `installation.log`. A single atomic `writeLine()` path formats each message once with a full timestamp and writes the exact same bytes to both stdout and the log file. The tick goroutine no longer emits standalone progress lines during pacstrap (only updates the context), eliminating duplicate percentage lines.

### Removed
- **Privileged helper binary** — Removed `aestheticos-helper` entirely. All privileged operations now run directly via `exec.Command` (running as root on the live ISO) or `arch-chroot` for installed-system operations. This eliminates the sudo wrapper layer, simplifies the build pipeline (one fewer binary), and removes the command whitelist/security surface area. The helper source (`backend/cmd/helper/`) and binary have been deleted.
- `FindHelper()` and `CheckRoot(helperPath)` — replaced with direct sudo checks (`sudo -n true`)
- `HelperCmd*` constants and `runHelper()` function
- `helperPath` field from `Installer` struct — `NewInstaller()` signature simplified

### Changed
- **GUI now spawns CLI binary** — `StartInstallation()` spawns `sudo aestheticos-installer-cli --install <disk> --filesystem <type>` and reads JSON progress events from stderr. This ensures all privileged operations run as root via the CLI.
- `writeFile()` / `appendFile()` now use native Go `os.WriteFile` / `os.OpenFile` instead of helper commands
- `runChroot()` now calls `arch-chroot /mnt <args...>` directly
- `formatPartition()` uses explicit `mkfs.fat` / `mkfs.ext4` / `mkfs.btrfs` instead of generic `mkfs`
- `setupBtrfsSubvolumes()` uses `btrfs subvolume create/set-default/property set` directly
- `ConfigureSystemBase()` uses `ln -sf` directly
- `validateMountedSystem()` no longer checks for helper path
- Root check now verifies `sudo -n true`
- `build.sh` no longer builds or copies `aestheticos-helper`
- Integration tests updated: fake disk data includes QEMU VirtIO disk (`vda`)
- Replaced `interrupted` flag coordination with `sync.Once` for cleanup — `RunCleanup()` is now the single entry point, called from defer, `OnBeforeClose`, SIGTERM handler, and panic recovery.
- Renamed `Cleanup()` to `RunCleanup()` (exported) for consistency
- **Restructured backend layout**: moved from `backend/internal/{core,install,shared}/` to flat `backend/{install,shared,validation}/`. CLI moved from `backend/cmd/installer-cli/` to `backend/cli/`.
- Extracted `Step` interface and step implementations into `backend/install/step.go`.
- Extracted `Plan` struct for step weights/percentages into `backend/install/plan.go`.
- Extracted `Executor` interface into `backend/install/executor.go`.
- Merged configuration sub-steps into `backend/install/configuration.go`.
- Added `ValidationError` and `InstallError` types in `backend/install/errors.go`.
- Moved `Logger` interface and `ConsoleLogger` to `backend/install/logger.go` (shared by CLI + GUI).
- Extracted `greetd` package into `backend/install/greetd/` (config, sync, memory).
- Extracted `PacstrapProgressTracker` into `backend/install/progress/pacstrap.go`.
- Added centralized `backend/validation/` package.
- Added `go mod tidy` to `build.sh` — fails the build early if dependencies drift.
- Make `SHARED_DIR` and `BIN_DIR` in `build.sh` optional environment variables instead of hardcoded paths.
- Removed 63 empty `if ctx.Logger != nil {}` dead-code blocks across the install package.
- Change password storage from `string` to `[]byte` with explicit zeroing after use.
- **Breaking**: Remove `--password` CLI flag — password is now read interactively from the terminal (masked) or via stdin pipe.

### Fixed
- **GUI error handling** — `StartInstallation` now correctly emits `installation:error` when the CLI fails. This ensures the GUI displays "Install failed" and stops the timer.
- **Progress logging** — `tickProgress` now only emits JSON progress events during dry-run mode, preventing redundant JSON output during real installations where progress is driven by `pacstrap`.
- **Wallpaper path** — Corrected the source path for the wallpaper in `SetupUsers`.
- **Disk validation** — Refined disk validation logic to handle permission/exit status errors more gracefully.
- Remove useless `parted` call in `FormatDisk` (was starting interactive mode and doing nothing)
- Add `sgdisk -Z` to clear partition table before creating partitions in `FormatDisk`
- Fix `chpasswd` in `SetupUsers` — now passes `username:password` format (was only passing password)
- Fix pre-format cleanup — uses `CleanupMounts()` instead of duplicating logic; fails clearly if unmount fails instead of silently continuing
- CLI Ctrl+C now cleanly cancels the child process (pacstrap) and runs full cleanup before exiting
- GUI window close during installation now triggers mount cleanup via `OnBeforeClose` hook
- Double cleanup on window close — `sync.Once` ensures cleanup runs exactly once
- Robustify `efibootmgr` output parsing — replaced fragile char-by-char digit collection with regex matching
- Pin Go toolchain to `go1.25.0` in `go.mod`
- Cleanup/rollback race conditions — `CleanupMounts` now returns an error, `Rollback` fails fast if cleanup fails, GPT partition table is cleared with `sgdisk -Z`, and `discoverPartitions` guards against double `/dev/` prefix
- Double cleanup on SIGINT — added `isInterruptError()` to detect Ctrl+C via the child process error message (`"signal: interrupt"`)
- SIGINT handling in CLI — signal handler sets interrupted flag, calls cleanup, and exits with code 130
- `CleanupMounts` now returns `error` instead of silently ignoring failures
- `RunStep()` now defers cleanup on failure (was missing, leaving mounts stuck after single-step failures)
- Removed redundant second `CleanupMounts` call on GUI success path
- `generateRandomSeed` in bootloader now uses `ctx.Logger`
- svelte-check error: `logInfo` called with 2 arguments instead of 1
- Rollback silent failures — `wipefs` and `partprobe` errors now returned
- CLI panic recovery — `defer recover()` ensures mount cleanup on panic
- Removed 6 `[DEBUG]` `fmt.Printf` statements leaking to stdout on live ISO
- Removed dead btrfs subvolume summary code in `setupBtrfsSubvolumes`

### Added
- `Installer.Cancel()` method and `InstallContext.Cancel` channel for clean interrupt propagation
- `App.installationRunning` atomic flag to track in-progress GUI installations
- `Installer.Rollback()` method
- `--rollback` CLI flag — cleans up partial installations (unmounts + wipes partition signatures)
- Deferred mount cleanup in `Install()` — ensures retryability on failure
- `DerivePartitionPath()` in shared package
- Integration tests for full install flow (dry-run, ext4/btrfs, progress tracking, error handling)
- `ValidateDiskPath()` — regex-sanitized disk path validation
- `ValidateDiskSize()` — minimum 16 GiB disk size check
- `--version` flag with ldflags version injection
- Named `Weight*` constants for installation step progress weights
- GitHub Actions CI workflow (`go test`, `go vet`, `svelte-check`, frontend tests)
- `SECURITY.md` — vulnerability reporting policy and security design
- `TROUBLESHOOTING.md` — common failures, log inspection, recovery steps
- Complete `CONTRIBUTING.md` with code style, commit conventions, and PR template
