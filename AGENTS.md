# AestheticOS Installer — Agent Context

## Project Identity
Graphical Arch Linux installer. **Wails v2** (Go backend + Svelte 5 frontend). Also ships CLI (`aestheticos-installer-cli`) for headless use. The GUI spawns the CLI via `sudo` for privileged ops. Offline installer — all packages from the live ISO's pacman database.

## Live ISO vs Installed System (CRITICAL — #1 source of bugs)
| Term | What | Key Paths |
|------|------|-----------|
| **Live ISO** | Bootable Arch env running the installer. Read-only rootfs (overlay). | Sudoers: user needs passwordless sudo for CLI binary. |
| **Installed System** | Arch being written to target disk. | Mounted at `/mnt/`. ESP at `/mnt/boot/`. Sudoers: `/etc/sudoers.d/wheel`. Accessed via `arch-chroot /mnt` |
| **Host** | Developer's main machine. | `~/Projects/shared` shared with VM. |
| **VM** | QEMU VM booting from ISO for testing. | `/shared` mounted from host shared dir. |

**Code rule:** `/mnt/*` or `arch-chroot` → installed system. `/dev/*` or direct execution → live ISO.

## Architecture — Where Everything Lives
```
backend/
  cli/main.go                 — CLI installer. Password read from stdin (never flags).
  gui/
    app.go                    — Wails app struct, StartUp, RebootAndClose
    bindings.go               — Wails API bindings (GetDisks, GetLanguages, GetFileSystem)
    installation.go           — StartInstallation (spawns CLI via sudo, reads JSON progress from stderr)
    navigation.go             — Page IDs, navigation state
    root_check.go             — Sudo verification
    validation.go             — Validate + Save methods (validate = no state mutation)
    logger.go                 — GUILogger (GUI logs → stderr+log; CLI output → log only)
  install/
    installer.go              — Installer orchestrator (shared by CLI + GUI). Install() runs 6 steps.
    step.go                   — Step interface + step implementations (FormatDiskStep, etc.)
    plan.go                   — Plan struct for step weights and cumulative percentages
    executor.go               — Executor interface (Run, RunChroot, WriteFile, AppendFile)
    configuration.go          — ConfigureSystem (merged step: fstab→bootloader→system→greetd→plymouth→zram→services)
    errors.go                 — ValidationError, InstallError types
    logger.go                 — Logger interface + ConsoleLogger (shared by CLI + GUI)
    format.go                 — FormatDisk() + setupBtrfsSubvolumes()
    mount.go                  — MountFilesystem() + verifyMount()
    install.go                — installPackages() (pacstrap with line-count progress tracking)
    install_base.go           — Install() for base packages
    install_desktop.go        — InstallDesktop() for desktop packages
    fstab.go                  — GenerateFstab() (custom for btrfs, standard for ext4)
    bootloader.go             — ConfigureBootloader() (systemd-boot install + boot entry)
    users.go                  — SetupUsers() (useradd, chpasswd, sudoers, ISO file copy)
    system.go                 — ConfigureSystemBase() (hostname, locale, timezone, keymap, mirrors)
    greetd.go                 — ConfigureGreetd() + syncDmsGreeter()
    greetd/                   — greetd package (config.go, sync.go, memory.go, data/)
    plymouth.go               — ConfigurePlymouth() (BGRT theme, mkinitcpio hook)
    services.go               — EnableServices() (NetworkManager, greetd, bluetooth)
    zram.go                   — ConfigureZram() (compressed swap via zram-generator)
    copy.go                   — copyISOFiles() (ISO → target file copying)
    cleanup.go                — CleanupMounts() (lazy recursive unmount)
    command.go                — Run(), runChroot(), writeFile(), appendFile() helpers
    rollback.go               — Rollback() (unmounts + wipes partition signatures)
    context.go                — InstallContext struct (shared state for all steps)
    checks.go                 — validateMountedSystem() and other validation helpers
    root_check.go             — RootCheckResult type
    progress/
      pacstrap.go             — PacstrapProgressTracker (line-count-based progress)
  validation/
    validation.go             — Centralized validation (delegates to shared package)
  shared/
    types.go                  — DiskInfo, PartitionInfo, UserAccountData, DiskSelectionData
    config.go                 — ALL constants: paths, command names, weights, thresholds, etc.
    disks.go                  — GetDisks(), GetPartitions(), DerivePartitionPath(), fakeDisks()
    regions.go                — 28 locale/region definitions
    locales.go                — Language list
    btrfs.go                  — BtrfsSubvolume definitions + GetBtrfsSubvolumes()
    validators.go             — ValidateUserAccount(), ValidateDiskPath(), ValidateDiskSelection()
    mirrorlist.go             — Embedded mirrorlists per region
    mirrorlist_gen.go         — Mirrorlist refresh script
    logpath.go                — GetLogPath()
```

## Installation Steps (6 top-level, 12 logical)
`install/installer.go` `DefaultPlan()` controls the step order. Weights control progress bar:
1. **FormatDisk** (weight 3) — GPT partitioning (EFI 128MB + root), format (vfat/ext4/btrfs), btrfs subvolumes
2. **MountFilesystem** (weight 2) — Mount root + subvolumes to `/mnt`, ESP to `/mnt/boot`
3. **Install** (weight 45) — `pacstrap /mnt base linux-lts linux-firmware ...`
4. **InstallDesktop** (weight 35) — `pacstrap` Hyprland, DMS greeter, alacritty, firefox, pipewire, etc.
5. **SetupUsers** (weight 2) — useradd, chpasswd, sudoers, copy ISO skel files
6. **ConfigureSystem** (weight 13) — Merged step: fstab → bootloader → system config → greetd → plymouth → zram → services

## Build & Test
```bash
./build.sh                          # Build all binaries to build/bin/
go test ./...                       # Run all Go tests
cd frontend && npm run test         # Run frontend tests
cd frontend && npm run check        # TypeScript type checking
```
Binaries: `build/bin/aestheticos-installer` (GUI), `build/bin/aestheticos-installer-cli` (CLI)
The GUI spawns the CLI via `sudo` for all privileged operations.

## Key Patterns (Follow These)
- **CLI spawns via sudo**: The GUI calls `StartInstallation()` which spawns `sudo aestheticos-installer-cli --install <disk>` and reads JSON progress from stderr.
- **Direct exec for root ops**: Install steps use `Run()` (exec.Command) and `runChroot()` (arch-chroot) directly — no helper binary.
- **InstallContext**: Shared state passed through all install steps. New fields go here, not in Installer struct.
- **Progress tracking**: pacstrap uses line-count-based tracker in `install/progress/pacstrap.go`. Step weights in `install/plan.go`.
- **Dry run**: `AESTHETIC_DRY_RUN=1` — steps sleep 200ms instead of executing.
- **Fake disks**: `FAKE_DISKS=1` — synthetic disk data for frontend dev.
- **Rollback**: `--rollback /dev/vda` — unmounts + wipes partition signatures.
- **Password**: CLI reads from stdin (masked in terminal, piped for automation). Never a flag.

## Rules
- Do not auto-commit. Always ask before committing.
- Update CHANGELOG.md under `[Unreleased]` with every meaningful change.
- Do not guess tool behavior — check man pages or ask.
- Follow existing code patterns before introducing new ones.
- Single responsibility: validate functions don't save, navigation doesn't mutate state.
- Test on the VM (QEMU), not the host.
- Run `go test ./...` before building.
- Binaries go in `build/bin/` — never in project root.
- **Run git commands one at a time.** Never chain multiple git operations in a single bash call — each can acquire locks and batching causes `index.lock` errors.
- **Constants live in `backend/shared/config.go`** — never duplicate constants across packages.

## Security
- Passwords read from stdin (never CLI flags — avoids /proc, ps, shell history).
- Helper runs via sudo with restricted command whitelists — never full root access.
- Installation log at `~/.local/share/aestheticos-installer/installation.log` is not encrypted.

## Desktop Environment
Hyprland + DankMaterialShell (Quickshell-based) + DMS Greeter (greetd) + Alacritty + Firefox + PipeWire. Boot splash: Plymouth BGRT.

Installed packages include: btop, fastfetch, brightnessctl, ddcutil, cava, matugen, mpv, imv, gnome-text-editor, nautilus, and Noto fonts (CJK + Emoji) + Terminus + JetBrains Mono Nerd.

## Frontend Pages (Wizard Flow)
```
Welcome (language select)
  → User Account (username + password)
    → Disk Selection (disk cards + filesystem pills)
      → Confirmation (summary + root check + confirm dialog)
        → Installation (progress bar + live events)
          → Finish (success)
```

## Constants Reference
- Min disk size: `shared.MinDiskSizeBytes` = 16 GiB
- EFI partition: 128MB, vfat, label "EFI", type GUID `C12A7328...`
- Root partition: rest of disk, ext4 or btrfs, label "aestheticos", type GUID `4F68BCE3...`
- Btrfs subvolumes: `@`, `@home`, `@srv`, `@cache`, `@tmp`, `@log`, `@swap` (compression: zstd)
- Default username: `shared.DefaultCLIUsername` = "aesthetic"
- Default hostname: `shared.DefaultHostname` = "aestheticos"
- Mount root: `shared.MountRoot` = "/mnt"
- ESP mount: `shared.BootMountPoint` = "/mnt/boot"
- Step weights: `shared.WeightFormatDisk=3`, `WeightMountFilesystem=2`, `WeightInstallBase=45`, `WeightInstallDesktop=35`, `WeightSetupUsers=2`, `WeightConfigureSystem=13`
