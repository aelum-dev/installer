# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security
- Fix injection vulnerability in `write-file` and `append-file` helper commands — replaced `bash -c` with native Go `os` package operations
- Change password storage from `string` to `[]byte` with explicit zeroing after use
- Remove automatic root password setting (root account stays locked, standard Arch practice)

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

### Fixed
- svelte-check error: `logInfo` called with 2 arguments instead of 1
