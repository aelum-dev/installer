# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
