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

## Testing Environments
- **Host machine** = main development system (where you develop the installer)
- **VM** = QEMU virtual machine booting from ISO (testing environment)
- `/home/power007/Projects/shared` is shared between host and VM
- `/shared` is mounted on a VM

## Testing
- Run `go test ./...` before building to catch regressions
