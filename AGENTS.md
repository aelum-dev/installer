# AestheticOS Installer — Agent Context

## Project Identity
Graphical Arch Linux installer. **Wails v2** (Go backend + Svelte 5 frontend). Also ships CLI (`aestheticos-installer-cli`) and privileged helper (`aestheticos-helper`). Offline installer — all packages from the live ISO's pacman database.

## Live ISO vs Installed System (CRITICAL — #1 source of bugs)
| Term | What | Key Paths |
|------|------|-----------|
| **Live ISO** | Bootable Arch env running the installer. Read-only rootfs (overlay). | Helper: `/usr/local/bin/aestheticos-helper`. Sudoers: `/etc/sudoers.d/aestheticos` |
| **Installed System** | Arch being written to target disk. | Mounted at `/mnt/`. ESP at `/mnt/boot/`. Sudoers: `/etc/sudoers.d/wheel`. Accessed via `arch-chroot /mnt` |
| **Host** | Developer's main machine. | `~/Projects/shared` shared with VM. |
| **VM** | QEMU VM booting from ISO for testing. | `/shared` mounted from host shared dir. |

**Code rule:** `/mnt/*` or `arch-chroot` → installed system. `/dev/*` or direct execution → live ISO.

## Architecture — Where Everything Lives
```
backend/
  cmd/helper/main.go          — Privileged helper. Command registration pattern. Two whitelists: runWhitelist (host), chrootRunWhitelist (inside chroot).
  cmd/installer-cli/main.go   — CLI installer. Password read from stdin (never flags).
  gui/
    app.go                    — Wails app struct, StartUp, RebootAndClose
    bindings.go               — Wails API bindings (GetDisks, GetLanguages, GetFileSystem)
    installation.go           — StartInstallation orchestration (runs in goroutine)
    navigation.go             — Page IDs, navigation state
    root_check.go             — Sudo verification
    validation.go             — Validate + Save methods (validate = no state mutation)
    logger.go                 — Error log writer
  internal/
    core/installer.go         — Installer orchestrator (shared by CLI + GUI). Install() runs 6 top-level steps.
    core/root_check.go        — CheckRoot()
    install/
      context.go              — InstallContext struct (shared state for all steps)
      config.go               — ALL constants: paths, command names, sgdisk args, mount opts, etc.
      format.go               — FormatDisk() + setupBtrfsSubvolumes()
      mount.go                — MountFilesystem() + verifyMount()
      install.go              — installPackages() (pacstrap with line-count progress tracking)
      install_base.go         — Install() for base packages
      install_desktop.go      — InstallDesktop() for desktop packages
      fstab.go                — GenerateFstab() (custom for btrfs, standard for ext4)
      bootloader.go           — ConfigureBootloader() (systemd-boot install + boot entry)
      users.go                — SetupUsers() (useradd, chpasswd, sudoers, ISO file copy)
      system.go               — ConfigureSystemBase() (hostname, locale, timezone, keymap, mirrors)
      greetd.go               — ConfigureGreetd() + syncDmsGreeter()
      plymouth.go             — ConfigurePlymouth() (BGRT theme, mkinitcpio hook)
      services.go             — EnableServices() (NetworkManager, greetd, bluetooth)
      zram.go                 — ConfigureZram() (compressed swap via zram-generator)
      copy.go                 — copyISOFiles() (ISO → target file copying)
      cleanup.go              — CleanupMounts() (lazy recursive unmount)
      command.go              — run(), runChroot(), writeFile(), appendFile() helpers
      checks.go               — validateMountedSystem() and other validation helpers
      logging.go              — Async logger
    shared/
      types.go                — DiskInfo, PartitionInfo, UserAccountData, DiskSelectionData
      config.go               — Constants: MinDiskSizeBytes, GPTTypeGUIDs, FileSystem types, etc.
      disks.go                — GetDisks(), GetPartitions(), DerivePartitionPath(), fakeDisks()
      regions.go              — 28 locale/region definitions
      locales.go              — Language list
      btrfs.go                — BtrfsSubvolume definitions + GetBtrfsSubvolumes()
      validators.go           — ValidateUserAccount(), ValidateDiskPath(), ValidateDiskSelection()
      mirrorlist.go           — Embedded mirrorlists per region
      mirrorlist_gen.go       — Mirrorlist refresh script
      logpath.go              — GetLogPath()
```

## Installation Steps (6 top-level, 12 logical)
`core/installer.go` `stepDefinitions` slice controls the order. Weights control progress bar:
1. **FormatDisk** (weight 3) — GPT partitioning (EFI 128MB + root), format (vfat/ext4/btrfs), btrfs subvolumes
2. **MountFilesystem** (weight 2) — Mount root + subvolumes to `/mnt`, ESP to `/mnt/boot`
3. **Install** (weight 45) — `pacstrap /mnt base linux-lts linux-firmware ...`
4. **InstallDesktop** (weight 35) — `pacstrap` Hyprland, DMS greeter, alacritty, firefox, pipewire, etc.
5. **SetupUsers** (weight 2) — useradd, chpasswd, sudoers, copy ISO skel files
6. **ConfigureSystem** (weight 13) — Merged step: fstab → bootloader → system config → greetd → plymouth → zram → services

## Build & Test
```bash
./build.sh                          # Build all 3 binaries to build/bin/
go test ./...                       # Run all Go tests
cd frontend && npm run test         # Run frontend tests
cd frontend && npm run check        # TypeScript type checking
```
Binaries: `build/bin/aestheticos-installer` (GUI), `build/bin/aestheticos-helper` (privileged), `build/bin/aestheticos-installer-cli` (CLI)

## Key Patterns (Follow These)
- **Helper binary**: All privileged ops go through `aestheticos-helper` via sudo. Never call `exec.Command` directly for root ops — use `ctx.run()` or `runChroot()`.
- **InstallContext**: Shared state passed through all install steps. New fields go here, not in Installer struct.
- **Progress tracking**: pacstrap uses line-count-based tracker in `install/install.go`. Step weights in `core/installer.go`.
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

## Security
- Passwords read from stdin (never CLI flags — avoids /proc, ps, shell history).
- Helper runs via sudo with restricted command whitelists — never full root access.
- Installation log at `~/.local/share/aestheticos-installer/installation.log` is not encrypted.

## Desktop Environment
Hyprland + DankMaterialShell (Quickshell-based) + DMS Greeter (greetd) + Alacritty + Firefox + PipeWire. Boot splash: Plymouth BGRT.

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
- ESP mount: `install.BootMountPoint` = "/mnt/boot"
