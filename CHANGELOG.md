# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Replace `interrupted` flag coordination with `sync.Once` for cleanup — `RunCleanup()` is now the single entry point, called from defer, `OnBeforeClose`, SIGTERM handler, and panic recovery. Eliminates race conditions from flag synchronization.
- `OnBeforeClose` now calls `installer.RunCleanup()` instead of running `CleanupMounts` directly
- SIGTERM handler in `main.go` now calls `installer.RunCleanup()` instead of running cleanup directly
- Removed `wasInterrupted` local variable, `isInterruptError()` call inside `Install()`, and duplicate cleanup logic from `OnBeforeClose` and SIGTERM handler
- Renamed `Cleanup()` to `RunCleanup()` (exported) for consistency

### Fixed
- CLI Ctrl+C now cleanly cancels the child process (pacstrap) and runs full cleanup before exiting — no more interleaved output or race between signal handler and defer
- GUI window close during installation now triggers mount cleanup via `OnBeforeClose` hook
- Double cleanup on window close — `sync.Once` ensures cleanup runs exactly once regardless of which path triggers it

### Added
- `Installer.Cancel()` method and `InstallContext.Cancel` channel for clean interrupt propagation to long-running child processes
- `App.installationRunning` atomic flag to track in-progress GUI installations

### Changed
- Add `go mod tidy` to `build.sh` and CI — fails the build early if dependencies drift

### Security
- Fix injection vulnerability in `write-file` and `append-file` helper commands — replaced `bash -c` with native Go `os` package operations
- Change password storage from `string` to `[]byte` with explicit zeroing after use
- Remove automatic root password setting (root account stays locked, standard Arch practice)
- **Breaking**: Remove `--password` CLI flag — password is now read interactively from the terminal (masked) or via stdin pipe. This prevents passwords from appearing in `/proc`, `ps`, and shell history.

### Changed
- Make `SHARED_DIR` and `BIN_DIR` in `build.sh` optional environment variables instead of hardcoded paths
- Remove 63 empty `if ctx.Logger != nil {}` dead-code blocks across the install package

### Fixed
- Robustify `efibootmgr` output parsing in helper — replaced fragile char-by-char digit collection with regex matching, fixing broken boot numbers on labels containing digits and hex entry IDs
- Pin Go toolchain to `go1.25.0` in `go.mod` to prevent confusing version errors on older Go installations

### Added
- GitHub Actions CI workflow (`go test`, `go vet`, `svelte-check`, frontend tests)
- `README.md` with project overview, build instructions, and usage
- `LICENSE` (GPL-3.0)
- `CHANGELOG.md`
- `--rollback` CLI flag — cleans up partial installations (unmounts + wipes partition signatures)
- Deferred mount cleanup in `Install()` — ensures retryability on failure
- `DerivePartitionPath()` in shared package
- Integration tests for full install flow (dry-run, ext4/btrfs, progress tracking, error handling)
- `ValidateDiskPath()` — regex-sanitized disk path validation
- `ValidateDiskSize()` — minimum 16 GiB disk size check
- `--version` flag with ldflags version injection
- Named `Weight*` constants for installation step progress weights
- `SECURITY.md` — vulnerability reporting policy and security design
- `TROUBLESHOOTING.md` — common failures, log inspection, recovery steps
- Complete `CONTRIBUTING.md` with code style, commit conventions, and PR template

### Fixed
- Cleanup/rollback race conditions — `CleanupMounts` now returns an error, `Rollback` fails fast if cleanup fails, GPT partition table is cleared with `sgdisk -Z`, and `discoverPartitions` guards against double `/dev/` prefix
- Double cleanup on SIGINT — added `isInterruptError()` to detect Ctrl+C via the child process error message (`"signal: interrupt"`) rather than relying on a flag that races with `pacstrap`'s termination; defer skips cleanup on interrupt to avoid duplicate unmount and duplicate log output
- SIGINT handling in CLI — signal handler sets interrupted flag, calls cleanup, and exits with code 130; CLI suppresses duplicate error output on interrupt
- `CleanupMounts` now returns `error` instead of silently ignoring helper failures
- `cmdCleanup` in helper now checks and returns errors from `sync`, `umount`, `partprobe`, and `dmsetup`
- `RunStep()` now defers cleanup on failure (was missing, leaving mounts stuck after single-step failures)
- Removed redundant second `CleanupMounts` call on GUI success path (Install() already defers it)
- `generateRandomSeed` in bootloader now uses `ctx.Logger` instead of `fmt.Fprintf(os.Stderr, ...)`
- svelte-check error: `logInfo` called with 2 arguments instead of 1
- Rollback silent failures — `wipefs` and `partprobe` errors now returned
- CLI panic recovery — `defer recover()` ensures mount cleanup on panic
- Removed 6 `[DEBUG]` `fmt.Printf` statements leaking to stdout on live ISO
- Removed dead btrfs subvolume summary code in `setupBtrfsSubvolumes`
- Fixed bare `exec.Command("sudo", "partprobe")` in `format.go` to use helper
