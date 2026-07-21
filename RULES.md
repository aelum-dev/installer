# Project Rules

## Git & Commits
- **Do not auto-commit changes.** Always ask for approval before committing.
- Binaries go in `build/bin/` — never in project root.

## Building
- **User always runs `build.sh` before testing.** Never assume user is running old binaries or manual `go build` commands.
- When building Go binaries, always use `-o` flag to specify output location:
  ```bash
  go build -o build/bin/aestheticos-installer-cli ./backend/cli/
  go build -o build/bin/aestheticos-helper ./backend/helper/
  ```

## Research & Documentation
- **Do not guess how tools work.** If unsure:
  1. Check `man` pages or `--help` flags
  2. If no documentation available, **ask the user** for documentation on the specific tool
- Never assume behavior of external tools (parted, bootctl, pacstrap, etc.) without verifying

## Root Detection & Testing Environments
- **Host machine** = main development system (where you develop the installer)
- **VM** = QEMU virtual machine booting from ISO (testing environment)
- `/home/power007/Projects/shared` is shared between host and VM
- `/shared` is mounted on VM
- **Helper binary location**: `/shared/aestheticos-helper` (in VM) or next to main executable (on host)
- **Root detection logic**: `CheckRoot` runs `sudo -n <helperPath> run id` to verify sudo configuration
  - On **host**: fails because sudo requires password (expected - developer hasn't configured NOPASSWD)
  - On **VM**: succeeds because helper is configured with NOPASSWD in sudoers
- **Key insight**: Root detection only matters in the VM (installer runs there). On host, it's expected to fail.
- Never assume `sudo -n id` output without understanding the execution context

## Testing
- Run `go test ./...` before building to catch regressions
