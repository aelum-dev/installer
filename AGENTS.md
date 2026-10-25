# Aelum Installer — Agent Context

## Project Identity
Graphical Arch Linux installer. **Wails v2** (Go backend + Svelte 5 frontend). Also ships CLI (`aelum-installer-cli`) for headless use. The GUI spawns the CLI via `sudo` for privileged ops. Offline installer — all packages from the live ISO's pacman database.

## Connection to the Aelum ISO repo

This repo is the **installer**; the [Aelum](https://github.com/aelum-dev/live-iso) repo builds the **live ISO**. They are two halves of one product:

- The ISO is barebones — on boot, greetd starts a minimal Hyprland that auto-launches `aelum-installer`.
- Built binaries are baked into the ISO at `airootfs/usr/local/bin/` (`aelum-installer`, `aelum-installer-cli`), via `BIN_DIR` in `build.sh`.
- Every install package comes from the ISO's embedded offline mirror (`airootfs/var/cache/aelum/offline-mirror/`); the install needs no network.
- After install, the target is a normal Arch machine and uses the internet normally. The per-region mirrorlist written here (`/etc/pacman.d/mirrorlist`) is only for future online updates — it does **not** feed the install.

In short: this repo = what installs onto the disk; the Aelum repo = the ISO that boots and launches this repo.

## Live ISO vs Installed System (CRITICAL — #1 source of bugs)
| Term | What | Key Paths |
|------|------|-----------|
| **Live ISO** | Bootable Arch env running the installer. Read-only rootfs (overlay). | Sudoers: user needs passwordless sudo for CLI binary. |
| **Installed System** | Arch being written to target disk. | Mounted at `/mnt/`. ESP at `/mnt/efi`; this install's ESP boot dir (`aelum/<id>/`) bind-mounted at `/mnt/boot`. Sudoers: `/etc/sudoers.d/wheel`. Accessed via `arch-chroot /mnt` |
| **Host** | Developer's main machine. | `~/Projects/shared` shared with VM. |
| **VM** | QEMU VM booting from ISO for testing. | `/shared` mounted from host shared dir. |

**Code rule:** `/mnt/*` or `arch-chroot` → installed system. `/dev/*` or direct execution → live ISO.

## Architecture — Where Everything Lives
```
.                               — Project root
  main.go                       — Wails app entry point (embeds frontend/dist, runs gui.NewApp). Handles AELUM_START_PAGE, catches SIGINT/SIGTERM/SIGHUP to cancel install + force-cleanup mounts.
  backend/
    cli/main.go                 — CLI installer. Password read from stdin (never flags).
    cli/main_test.go            — CLI tests
    gui/
      app.go                    — Wails app struct, NewApp, StartUp (CheckRoot→dry-run fallback, GetUILocale), OnBeforeClose, CancelInstallation, WaitForCleanup, SetStartPage
      bindings.go               — Wails API bindings (GetDisks, GetLanguages, GetFileSystem, GetUILocale, SetUILocale)
      installation.go           — StartInstallation (spawns CLI via sudo, reads JSON progress from stderr)
      navigation.go             — Page IDs, navigation state, GetUILocale/SetUILocale
      root_check.go             — Sudo verification
      validation.go             — Validate + Save methods (validate = no state mutation)
      root_check_test.go        — Root check tests
      validation_test.go        — Validation tests
    install/
      installer.go              — Installer orchestrator (shared by CLI + GUI). Install() runs 7 steps.
      step.go                   — Step interface + stepFunc + Steps()/stepWeights() (order + weights)
      context.go                — InstallContext struct (shared state, Cancel channel, Logger)
      errors.go                 — ValidationError, InstallError types
      cleanup.go                — Rollback() + partition discovery (exposed for --rollback)
      primitives/
        primitives.go           — Low-level primitives shared by all steps: Run(), RunChroot(),
                                  WriteFile(), AppendFile(), ReadFile(), Stat(), RemoveAll(), Chmod(),
                                  ValidateMountedSystem(), CleanupMounts()
                                  (+ CommandRunner seam: SetCommandRunner() injects a fake)
                                  (+ FileSystem seam: SetFileSystem() injects a fake)
        primitivestest/
          recorder.go           — Scripted primitives.CommandRunner for unit tests (no root needed)
          memfs.go              — In-memory primitives.FileSystem for unit tests (no /mnt needed)
      format/
        format.go               — FormatDisk() + PrepareRoot() (LUKS → LVM → mkfs stack) + setupBtrfsSubvolumes() (+ format_test.go)
      lvm/
        lvm.go                  — per-install volume group (PV → VG → root/home LV), PARTUUID-derived name, device-scoped teardown (+ lvm_test.go)
      mount/
        mount.go                — MountFilesystem() (activates the VG, mounts root LV + ext4 home LV) + verifyMount() (+ mount_test.go)
      installbase/
        install.go              — base pacstrap (basePackages) with phase-banded progress (+ install_test.go)
      installdesktop/
        install.go              — desktop pacstrap (desktopPackages) with phase-banded progress (+ install_test.go)
      users/
        users.go                — SetupUsers() (useradd, chpasswd, sudoers, ISO skel copy) (+ users_test.go)
      configure/
        configure.go            — Run() step orchestrator (GenerateFstab→system→initramfs→bootloader→greetd→plymouth→zram→services)
        fstab.go                — fstab generation + verification (ESP at /efi, per-install /boot bind; btrfs/ext4 variants, ext4+LVM /home)
        bootloader.go           — rEFInd install + branded theme + per-install menu fragment (loads aelum/<id>/ kernel; LUKS-aware root options)
        system.go               — hostname/timezone/locale/keymap/NTP/mirrorlist + initramfs HOOKS
        greetd.go               — greetd + DMS greeter executor (greetdExecutor)
        plymouth.go             — Aelum LUKS unlock Plymouth theme + .hushlogin + aelum-boot-progress.service (finishes the splash bar)
        services.go             — zram swap config + systemd service enablement
        (+ fstab_test.go, swap_test.go)
      partition/
        partition.go            — ComputeLayout(), Apply() (manual partitioning: formats Boot + System)
        validate.go             — Validate()/ValidateFeasible() invariants
        layout.go               — free-space placement math (buildActions, allocateFree, …)
        sectors.go              — GPT sector geometry helpers + sgdisk -p parsing
      clearcache/
        clearcache.go           — ClearCache() (pacman -Scc in chroot) (+ clearcache_test.go)
      greetd/
        greetd.go               — Greetd configuration type
        config.go               — Config struct
        sync.go                 — syncDmsGreeter() (per-user slot copies of settings/session/colors + wallpaper) (+ sync_test.go)
        memory.go               — Memory type
        data/greetd/
          config.toml           — Embedded greetd config template (includes --cache-dir)
          memory.json           — Embedded greetd memory template
      progress/
        pacstrap.go             — PacstrapProgressTracker (phase-banded progress)
        pacstrap_test.go        — Progress tracker tests
      *_test.go                 — Unit + integration tests (installer, installer_integration)
    shared/
      types.go                  — DiskInfo, PartitionInfo, UserAccountData, DiskSelectionData
      config.go                 — ALL constants: paths, command names, weights, thresholds, wallpaper, progress-tick, etc.
      logger.go                 — Logger interface + ConsoleLogger (shared by CLI + GUI)
      disks.go                  — GetDisks(), GetPartitions(), DerivePartitionPath()
      disks_fake.go             — fakeDisks() for frontend dev (FAKE_DISKS=1, also used in dry-run)
      regions.go                — Locale/region definitions
      locales.go                — Language list (28 languages)
      btrfs.go                  — BtrfsSubvolume definitions + GetBtrfsSubvolumes()
      validators.go             — ValidateUserAccount(), ValidateDiskPath(), ValidateDiskSelection(), ValidateDiskSize()
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
      style.css                 — Global styles (+ shared layout)
      vite-env.d.ts             — Vite type declarations
      assets/
        fonts/                  — Nunito (Latin + Cyrillic woff2), OFL.txt
        images/                 — logo-universal.png
      translations/             — i18n locale JSON files (28 locales, en-US + RTL for ar-SA/he-IL)
      components/
        AppHeader.svelte        — Top bar with page title (+ DRY RUN badge)
        AppFooter.svelte        — Bottom bar with back/next buttons (+ step indicator dots)
        Button.svelte           — Reusable button component
        ConfirmDialog.svelte    — Confirmation/validation modal (uses Dialog)
        Dialog.svelte           — Shared modal chrome: overlay, Escape, focus restore
        PageLayout.svelte       — Shared two-column wizard page shell (info rail + content)
        ScrollBox.svelte        — Scrollable container component
      lib/
        constants.ts            — Frontend constants (auto-generated from Go via gen-ts-constants.go)
        disks.ts                — Disk filesystem inference helper
        manual-layout.ts        — Pure manual-partition geometry + structural validation
        navigation.ts           — Page navigation state
        page-validation.ts      — `isPageValid()`/`getPageValidationReasons()` (shares validate.ts rules)
        store.svelte.ts         — Svelte 5 store (`installState`, `navState`, LUKS actions)
        i18n/
          index.svelte.ts       — Translation runtime (glob-imported locales) + locale loading
          index.ts              — Re-exports for .ts/.svelte consumers
          locales.ts            — Supported locale list + RTL set
        types.ts                — TypeScript type definitions
        validate.ts             — Single user-account rule engine + inline validation
        validate-messages.ts    — Validation message strings
        validate.test.ts        — Validation tests
      pages/
        WelcomePage.svelte      — Language/region selection
        UserAccountPage.svelte  — Username + password (interactive validation hints)
        DiskSelectionPage.svelte — Disk cards + filesystem pills
        ManualPartitionPage.svelte — Manual partition editor
        ConfirmationPage.svelte — Summary + root check + confirm dialog
        InstallationPage.svelte — Progress bar + live events
        FinishPage.svelte       — Success screen
    wailsjs/
      go/gui/App.d.ts           — Wails-generated TypeScript bindings
      go/gui/App.js             — Wails-generated JS bindings
      go/models.ts              — Wails-generated TypeScript models
      runtime/                  — Wails runtime JS+TS
  build/
    bin/                        — Built binaries (aelum-installer, aelum-installer-cli)
    darwin/                     — macOS build output
    windows/                    — Windows build output
    appicon.png                 — Application icon
    README.md                   — Build directory documentation
  .agents/                      — Agent skills directory
  wails.json                    — Wails configuration
  go.mod / go.sum               — Go module files
  Makefile                      — Build targets
  build.sh                      — Build script (builds all binaries to build/bin/)
  build-local.sh                — Local build script
  run.sh                        — Run script
```

## Installation Steps (8 top-level)
`install/step.go` `Steps()` controls the step order. Weights control progress bar:
1. **FormatDisk** (weight 3) — GPT partitioning (EFI 512MB + root), format (vfat/ext4/btrfs), optional LUKS2 → LVM → mkfs stack, btrfs subvolumes (`install/format/`)
2. **MountFilesystem** (weight 2) — Mount root + subvolumes to `/mnt` (activating the volume group first when LVM is on), the ESP to `/mnt/efi`, then bind this install's `aelum/<id>/` ESP directory at `/mnt/boot` (`install/mount/`)
3. **InstallBase** (weight 37) — `pacstrap /mnt base linux-lts linux-firmware ...` (base packages, phase-banded progress, `install/installbase/`)
4. **InstallDesktop** (weight 50) — `pacstrap` Hyprland, DMS greeter, kitty, firefox, pipewire, etc. (desktop packages, `install/installdesktop/`)
5. **SetupUsers** (weight 2) — useradd, chpasswd, sudoers, copy ISO skel files (`install/users/`)
6. **ConfigureSystem** (weight 4) — Merged step: fstab → bootloader → system config → greetd → plymouth → zram → services (`install/configure/`)
7. **ClearCache** (weight 1) — Clears pacman package cache in installed system (`pacman -Scc`, `install/clearcache/`)
8. **VerifySystem** (weight 1) — Read-only post-install verification (`install/verify/`): checks fstab (root/EFI UUIDs, ESP at `/efi`, per-install `/boot` bind), kernel + initrd in this install's ESP boot directory (via the `/mnt/boot` bind), the rEFInd binary and its Aelum menu entry, required packages via `pacman -Q`, the user account, and hostname. Logs every check and fails with a combined error if anything is missing. Total weight stays at 100 (base/desktop each reduced by 1 to make room).

## Build & Test
```bash
./build.sh                          # Build all binaries to build/bin/
# SHARED_DIR=/shared BIN_DIR=/path also copies binaries after build
go test ./...                       # Run all Go tests
cd frontend && npm run test         # Run frontend tests (Vitest)
cd frontend && npm run check        # TypeScript type checking
```
Binaries: `build/bin/aelum-installer` (GUI), `build/bin/aelum-installer-cli` (CLI)
The GUI spawns the CLI via `sudo` for all privileged operations.

## Environment Variables
| Variable | Purpose |
|----------|---------|
| `AELUM_DRY_RUN=1` | Simulate installation (steps sleep 200ms); also loads fake disks in `GetDisks()`/`GetDiskPartitions()`; auto-enabled when not root |
| `AELUM_START_PAGE=N` | Start on page N (0-5) for debugging (see `main.go`) |
| `FAKE_DISKS=1` | Use synthetic disk data (for frontend development) |
| `SHARED_DIR` | Copy binaries to this path after build (e.g. VM shared folder) |
| `BIN_DIR` | Copy binaries to distro airootfs bin dir after build |

## Internationalization (i18n)
All UI strings are translated by a small custom flat-key runtime in `frontend/src/lib/i18n/` (`t()` with English fallback, glob-imported locale JSON). Frontend locales live in `frontend/src/translations/*.json` (28 locales); locale display names + the RTL set are in `frontend/src/lib/i18n/locales.ts`. Backend locale codes (`de_DE.UTF-8`) map mechanically to frontend codes (`de-DE`). RTL is supported for Arabic (`ar-SA`), Hebrew (`he-IL`), and Persian (`fa-IR`). Backend exposes `GetUILocale()`/`SetUILocale()` for auto-detection + persistence.

## Key Patterns (Follow These)
- **CLI spawns via sudo**: The GUI calls `StartInstallation()` which spawns `sudo aelum-installer-cli --install <disk>` and reads JSON progress from stderr.
- **Direct exec for root ops**: Install steps use `Run()` (exec.Command) and `RunChroot()` (arch-chroot) directly — no helper binary. Both go through the `primitives.CommandRunner` seam: tests inject a fake with `primitives.SetCommandRunner(rec)()` (see `primitives/primitivestest`) to assert argv and simulate output/failure without root. Commands are `*primitives.Command` (mirrors `exec.Cmd`'s `Args`/`Stdin`/`Run`/`Output`/`CombinedOutput`). Note: `installbase` and the GUI spawn streaming processes (`Start`/`Wait`/pipes) via `os/exec` directly — those are not behind the seam.
- **File I/O for root ops**: Install steps write/read the target via `primitives.WriteFile()`/`AppendFile()`/`ReadFile()`/`Stat()`/`RemoveAll()`/`Chmod()` rather than calling `os` directly. Those go through the `primitives.FileSystem` seam: tests inject `primitivestest.NewMemFS()` with `primitives.SetFileSystem(fs)()` to exercise file-driven logic without writing under `/mnt`. `WriteFile`/`AppendFile` prepend `MountRoot` for chroot paths and leave host paths (`/mnt/...`) as-is; `WriteFileMode`/`ReadFile`/`Stat`/`RemoveAll`/`Chmod` take host paths verbatim. `os/exec`, `shared/` (logger, mirrorlist), and the GUI binary lookup still use `os` directly.
- **InstallContext**: Shared state passed through all install steps. New fields go here, not in Installer struct.
- **Progress tracking**: pacstrap uses a phase-banded tracker in `install/progress/pacstrap.go` (download/install/hook bands; see `PacstrapDownloadBandEnd`/`PacstrapInstallBandEnd` in `shared/config.go`). Step weights in `install/step.go`.
- **Dry run**: `AELUM_DRY_RUN=1` — steps sleep 200ms instead of executing.
- **Fake disks**: `FAKE_DISKS=1` — synthetic disk data for frontend dev.
- **Rollback**: `--rollback /dev/vda` — unmounts + wipes partition signatures.
- **Password**: CLI reads from stdin (masked in terminal, piped for automation). Never a flag.

## Rules
- Do not auto-commit. Always ask before committing.
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
- Installation log at `/var/log/aelum-install-<timestamp>.log` (mirrored to `/shared` when it exists) is not encrypted.

## Desktop Environment
Hyprland + DankMaterialShell (Quickshell-based) + DMS Greeter (greetd) + Kitty + Firefox + PipeWire. Boot splash: Aelum LUKS unlock Plymouth theme (branded logo + passphrase splash for encrypted boot), configured in `install/configure/` to match the live ISO.

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
- Min disk size: `shared.MinDiskSizeBytes` = 8 GiB (reduced from 16 GiB after removing the swap file in favor of zram); manual mode uses the same `shared.MinManualDiskSizeBytes` = 8 GiB floor
- EFI partition: 512MB, vfat, label "EFI", type GUID `C12A7328...`
- Boot floor: every Boot partition (new or existing) must be at least `shared.MinESPSizeBytes` = 512 MiB. Manual mode always formats the selected Boot partition as FAT32, so it never reuses another OS's ESP.
- System floor: `shared.MinSystemPartitionBytes` = 7680 MiB (the 8 GiB disk minimum minus the 512 MiB Boot partition). With LVM it rises to `shared.MinLvmSystemPartitionBytes` (16 GiB + 16 MiB metadata), plus `shared.LuksHeaderReserveBytes` when LUKS sits under it — use `shared.SystemPartitionFloorBytes(lvm, luks)`.
- Root partition: rest of disk, ext4 or btrfs, label "aelum", type GUID `4F68BCE3...` (Linux LVM `E6D6D379...` when it becomes a PV without LUKS)
- LVM: one PV per install (the System partition or its LUKS mapper). The VG is named after the install id — the PV's PARTUUID-derived `<8 hex>`, with no `aelum-` prefix and no hyphen, so device-mapper names carry a single separator (`1a2b3c4d-root`, never `aelum--1a2b3c4d-root`); LVs `root`, plus `home` for ext4 only (btrfs keeps `@home` on the single root LV). LUKS always sits below LVM; fstab/boot reference filesystem UUIDs only, never `/dev/mapper`. Teardown resolves the VG from the PV (`pvs`) and refuses a multi-PV group.
- Btrfs subvolumes: `@` (default, zstd), `@home` (zstd), `@srv` (zstd), `@cache` (nodatacow), `@tmp` (nodatacow), `@log` (zstd). Swap is provided by zram, not a subvolume.
- Default username: `shared.DefaultCLIUsername` = "aelum"
- Default hostname: `shared.DefaultHostname` = "aelum"
- Mount root: `shared.MountRoot` = "/mnt"
- ESP mount during install: `shared.ESPMountPoint` = "/mnt/efi"
- Boot mount: `shared.BootMountPoint` = "/mnt/boot" (bind mount of `<ESP>/aelum/<id>`)
- Installed ESP mount: `shared.InstalledESPMountPoint` = "/efi"
- Per-install ESP boot directory: `shared.EspInstallDirName` = "aelum"; each install uses `aelum/<id>` where `<id>` is the first 8 alphanumeric characters of the root filesystem UUID
- Step weights: `shared.WeightFormatDisk=3`, `WeightMountFilesystem=2`, `WeightInstallBase=37`, `WeightInstallDesktop=50`, `WeightSetupUsers=2`, `WeightConfigureSystem=4`, `WeightClearCache=1`, `WeightVerifySystem=1` (total 100)
