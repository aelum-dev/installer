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
.                               — Project root
  main.go                       — Wails app entry point (embeds frontend/dist, runs gui.NewApp)
  backend/
    cli/main.go                 — CLI installer. Password read from stdin (never flags).
    cli/main_test.go            — CLI tests
    gui/
      app.go                    — Wails app struct, StartUp, OnBeforeClose
      bindings.go               — Wails API bindings (GetDisks, GetLanguages, GetFileSystem)
      installation.go           — StartInstallation (spawns CLI via sudo, reads JSON progress from stderr)
      navigation.go             — Page IDs, navigation state
      root_check.go             — Sudo verification
      validation.go             — Validate + Save methods (validate = no state mutation)
      root_check_test.go        — Root check tests
      validation_test.go        — Validation tests
    install/
      installer.go              — Installer orchestrator (shared by CLI + GUI). Install() runs 7 steps.
      step.go                   — Step interface + step implementations (FormatDiskStep, etc.) + Steps()
      configuration.go          — ConfigureSystem (merged step: GenerateFstab→bootloader→system→greetd→plymouth→zram→services)
      errors.go                 — ValidationError, InstallError types
      logger.go                 — Logger interface + ConsoleLogger (shared by CLI + GUI)
      format.go                 — FormatDisk() + setupBtrfsSubvolumes()
      mount.go                  — MountFilesystem() + verifyMount()
      install_packages.go       — installPackages(), Install(), InstallDesktop() (pacstrap with line-count progress)
      clearcache.go             — ClearCache() for package manager cache cleanup
      users.go                  — SetupUsers() (useradd, chpasswd, sudoers, ISO file copy)
      cleanup.go                — CleanupMounts() (lazy recursive unmount)
      command.go                — Run(), runChroot(), writeFile(), appendFile() helpers
      context.go                — InstallContext struct (shared state for all steps)
      greetd/
        greetd.go               — Greetd configuration type
        config.go               — Config struct
        sync.go                 — syncDmsGreeter()
        memory.go               — Memory type
        data/greetd/
          config.toml           — Embedded greetd config template
          memory.json           — Embedded greetd memory template
      progress/
        pacstrap.go             — PacstrapProgressTracker (line-count-based progress)
        pacstrap_test.go        — Progress tracker tests
      installer_test.go         — Installer unit tests
      installer_integration_test.go — Integration tests
      install_packages_test.go  — Install packages tests
      fstab_test.go             — Fstab tests
      swap_test.go              — Swap/zram tests
    shared/
      types.go                  — DiskInfo, PartitionInfo, UserAccountData, DiskSelectionData
      config.go                 — ALL constants: paths, command names, weights, thresholds, etc.
      disks.go                  — GetDisks(), GetPartitions(), DerivePartitionPath()
      disks_fake.go             — fakeDisks() for frontend dev (FAKE_DISKS=1)
      regions.go                — 28 locale/region definitions
      locales.go                — Language list
      btrfs.go                  — BtrfsSubvolume definitions + GetBtrfsSubvolumes()
      validators.go             — ValidateUserAccount(), ValidateDiskPath(), ValidateDiskSelection()
      mirrorlist.go             — Embedded mirrorlists per region
      mirrorlist_gen.go         — Mirrorlist refresh script
      logpath.go                — GetLogPath()
      mirrorlist/               — Mirrorlist data files per locale (28 locale .list files + all_https.txt, default.list)
      disks_test.go             — Disk tests
      validators_test.go        — Validator tests
    validation/
      validation.go             — Centralized validation (delegates to shared package)
    scripts/
      gen-ts-constants.go       — Generates frontend wailsjs/go models from Go constants
  frontend/
    src/
      App.svelte                — Root component (wizard page router)
      App.test.ts               — App component tests
      main.ts                   — Vite entry point
      style.css                 — Global styles
      vite-env.d.ts             — Vite type declarations
      components/
        AppHeader.svelte        — Top bar with page title
        AppFooter.svelte        — Bottom bar with back/next buttons
        Button.svelte           — Reusable button component
        ConfirmDialog.svelte    — Confirmation dialog modal
        Dropdown.svelte         — Dropdown/select component
        ErrorBanner.svelte      — Error message display
        PageLayout.svelte       — Page wrapper with header/footer
      lib/
        constants.ts            — Frontend constants (auto-generated from Go via gen-ts-constants.go)
        navigation.ts           — Page navigation state
        state.svelte.ts         — Svelte 5 store for installer state
        types.ts                — TypeScript type definitions
        validate.ts             — Frontend validation helpers
        validate.test.ts        — Validation tests
      pages/
        WelcomePage.svelte      — Language/region selection
        UserAccountPage.svelte  — Username + password
        DiskSelectionPage.svelte — Disk cards + filesystem pills
        ConfirmationPage.svelte — Summary + root check + confirm dialog
        InstallationPage.svelte — Progress bar + live events
        FinishPage.svelte       — Success screen
    wailsjs/
      go/gui/App.d.ts           — Wails-generated TypeScript bindings
      go/gui/App.js             — Wails-generated JS bindings
      go/models.ts              — Wails-generated TypeScript models
      runtime/                  — Wails runtime JS+TS
  build/
    bin/                        — Built binaries (aestheticos-installer, aestheticos-installer-cli)
    darwin/                     — macOS build output
    windows/                    — Windows build output
    appicon.png                 — Application icon
    README.md                   — Build directory documentation
  .agents/                      — Agent skills directory
  .github/                      — GitHub workflows
  wails.json                    — Wails configuration
  go.mod / go.sum               — Go module files
  Makefile                      — Build targets
  build.sh                      — Build script (builds all binaries to build/bin/)
  build-local.sh                — Local build script
  run.sh                        — Run script
```

## Installation Steps (7 top-level)
`install/step.go` `Steps()` controls the step order. Weights control progress bar:
1. **FormatDisk** (weight 3) — GPT partitioning (EFI 128MB + root), format (vfat/ext4/btrfs), btrfs subvolumes
2. **MountFilesystem** (weight 2) — Mount root + subvolumes to `/mnt`, ESP to `/mnt/boot`
3. **InstallBase** (weight 38) — `pacstrap /mnt base linux-lts linux-firmware ...` (basePackages in `install/install_packages.go`)
4. **InstallDesktop** (weight 51) — `pacstrap` Hyprland, DMS greeter, alacritty, firefox, pipewire, etc. (desktopPackages in `install/install_packages.go`)
5. **SetupUsers** (weight 2) — useradd, chpasswd, sudoers, copy ISO skel files
6. **ConfigureSystem** (weight 4) — Merged step: fstab → bootloader → system config → greetd → plymouth → zram → services (`install/configuration.go`)
7. **ClearCache** (weight 1) — Clears pacman package cache in installed system (`install/clearcache.go`)

## Build & Test
```bash
./build.sh                          # Build all binaries to build/bin/
go test ./...                       # Run all Go tests
cd frontend && npm run test         # Run frontend tests (Vitest)
cd frontend && npm run check        # TypeScript type checking
```
Binaries: `build/bin/aestheticos-installer` (GUI), `build/bin/aestheticos-installer-cli` (CLI)
The GUI spawns the CLI via `sudo` for all privileged operations.

## Key Patterns (Follow These)
- **CLI spawns via sudo**: The GUI calls `StartInstallation()` which spawns `sudo aestheticos-installer-cli --install <disk>` and reads JSON progress from stderr.
- **Direct exec for root ops**: Install steps use `Run()` (exec.Command) and `runChroot()` (arch-chroot) directly — no helper binary.
- **InstallContext**: Shared state passed through all install steps. New fields go here, not in Installer struct.
- **Progress tracking**: pacstrap uses line-count-based tracker in `install/progress/pacstrap.go`. Step weights in `install/step.go`.
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
- **Frontend constants are auto-generated** from Go via `backend/scripts/gen-ts-constants.go` → `frontend/src/lib/constants.ts`. Regenerate after changing Go constants.
- **Run `golangci-lint run ./... --fix` after every Go code change.** This lints the code and auto-fixes formatting issues. Fix any remaining lint errors before committing.

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
- Step weights: `shared.WeightFormatDisk=3`, `WeightMountFilesystem=2`, `WeightInstallBase=38`, `WeightInstallDesktop=51`, `WeightSetupUsers=2`, `WeightConfigureSystem=4`, `WeightClearCache=1`
