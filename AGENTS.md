# AestheticOS Installer — Agent Context

## Project Identity
Graphical Arch Linux installer built with **Wails v2** (Go backend + Svelte 5 frontend). Also ships a CLI (`aestheticos-installer-cli`) and privileged helper (`aestheticos-helper`). Offline installer — all packages from the live ISO's pacman database.

## Live ISO vs Installed System (CRITICAL — #1 source of bugs)
| Term | What | Key Paths |
|------|------|-----------|
| **Live ISO** | Bootable Arch env running the installer. Read-only rootfs (overlay). | Helper: `/usr/local/bin/aestheticos-helper`. Sudoers: `/etc/sudoers.d/aestheticos` |
| **Installed System** | Arch being written to target disk. | Mounted at `/mnt/`. ESP at `/mnt/boot/`. Sudoers: `/etc/sudoers.d/wheel`. Accessed via `arch-chroot /mnt` |
| **Host** | Developer's main machine. | `/home/power007/Projects/shared` shared with VM. |
| **VM** | QEMU VM booting from ISO for testing. | `/shared` mounted from host shared dir. |

**Code rule:** `/mnt/*` or `arch-chroot` → installed system. `/dev/*` or direct execution → live ISO.

## Architecture
```
backend/
  cmd/helper/main.go          — Privileged helper (command registration pattern, whitelists)
  cmd/installer-cli/main.go   — CLI installer
  gui/                        — Wails GUI layer (bindings, navigation, validation)
  internal/
    core/installer.go         — Installer orchestrator (shared by CLI + GUI)
    install/                  — 12 install steps (format, mount, pacstrap, bootloader, etc.)
    shared/                   — Types, validators, disk detection, regions, mirrors
frontend/src/
  pages/                      — 6 wizard pages (Welcome, User Account, Disk, Confirmation, Installation, Finish)
  components/                 — Reusable UI components
  lib/                        — State, navigation, validation
```

## Building & Testing
```bash
./build.sh                          # Build all 3 binaries to build/bin/
go test ./...                       # Run all Go tests
cd frontend && npm run test         # Run frontend tests
```
Binaries: `build/bin/aestheticos-installer` (GUI), `build/bin/aestheticos-helper` (privileged), `build/bin/aestheticos-installer-cli` (CLI)

## Key Patterns
- **Helper binary**: All privileged ops go through `aestheticos-helper` via sudo. Two whitelists: `runWhitelist` (host) and `chrootRunWhitelist` (inside chroot).
- **InstallContext**: Shared state passed through all install steps. See `backend/internal/install/context.go`.
- **Progress tracking**: Line-count-based pacstrap tracker + weighted step percentages.
- **Dry run**: `AESTHETIC_DRY_RUN=1` simulates install (steps sleep 200ms).
- **Fake disks**: `FAKE_DISKS=1` uses synthetic disk data for frontend dev.
- **Rollback**: `--rollback /dev/vda` cleans up partial installs.

## Rules
- Do not auto-commit. Always ask before committing.
- Update CHANGELOG.md under `[Unreleased]` with every meaningful change.
- Do not guess tool behavior — check man pages or ask.
- Follow existing code patterns before introducing new ones.
- Single responsibility: validate functions don't save, navigation doesn't mutate state.
- Test on the VM (QEMU), not the host.
- Run `go test ./...` before building.

## Security
- Passwords are read from stdin (masked in terminal, or piped). Never accepted as CLI flags.
- Helper runs via sudo with restricted command whitelists — never full root access.
- Installation log at `~/.local/share/aestheticos-installer/installation.log` is not encrypted.

## Desktop Environment
Hyprland + DankMaterialShell (Quickshell-based) + DMS Greeter (greetd) + Alacritty + Firefox + PipeWire. Boot splash: Plymouth BGRT.
