# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security
- Fix injection vulnerability in `write-file` and `append-file` helper commands — replaced `bash -c` with native Go `os` package operations
- Change password storage from `string` to `[]byte` with explicit zeroing after use
- Remove automatic root password setting (root account stays locked, standard Arch practice)
- **Breaking**: Remove `--password` CLI flag — password is now read interactively from the terminal (masked) or via stdin pipe. This prevents passwords from appearing in `/proc`, `ps`, and shell history.

### Changed
- Make `SHARED_DIR` and `BIN_DIR` in `build.sh` optional environment variables instead of hardcoded paths
- Remove 63 empty `if ctx.Logger != nil {}` dead-code blocks across the install package

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
- svelte-check error: `logInfo` called with 2 arguments instead of 1
- Rollback silent failures — `wipefs` and `partprobe` errors now returned
- CLI panic recovery — `defer recover()` ensures mount cleanup on panic
- Removed 6 `[DEBUG]` `fmt.Printf` statements leaking to stdout on live ISO
- Removed dead btrfs subvolume summary code in `setupBtrfsSubvolumes`
- Fixed bare `exec.Command("sudo", "partprobe")` in `format.go` to use helper
