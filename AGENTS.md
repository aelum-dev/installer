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
| **Host** | Developer's main machine. | `../shared` (the distro workspace) is the host side of the VM's `/shared`. |
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
        root_snapshot.go        — the history under @ and the snapshot the system is installed into (@/.snapshots/1/snapshot + its info.xml + the /.snapshots mount point); btrfs only
      lvm/
        lvm.go                  — per-install volume group (PV → VG → root/home LV), PARTUUID-derived name, device-scoped teardown (+ lvm_test.go)
      mount/
        mount.go                — MountFilesystem() (activates the VG, mounts root LV + ext4 home LV) + verifyMount() (+ mount_test.go)
      installbase/
        install.go              — base pacstrap (basePackages + btrfsOnlyBasePackages on btrfs) with phase-banded progress (+ install_test.go)
      installdesktop/
        install.go              — desktop pacstrap (desktopPackages + btrfsOnlyDesktopPackages on btrfs) with phase-banded progress (+ install_test.go)
      users/
        users.go                — SetupUsers() (useradd, chpasswd, sudoers, ISO skel copy) (+ users_test.go)
      configure/
        configure.go            — Run() step orchestrator (GenerateFstab→system→initramfs→bootloader→greetd→plymouth→zram→services→snapper)
        fstab.go                — fstab generation + verification (ESP at /efi, per-install /boot bind; btrfs/ext4 variants, ext4+LVM /home)
        bootloader.go           — rEFInd install + branded theme + per-install menu fragment (loads aelum/<id>/ kernel; LUKS-aware root options) + the empty snapshot fragment a btrfs install's refind.conf includes
        system.go               — hostname/timezone/locale/keymap/NTP/mirrorlist + initramfs HOOKS
        greetd.go               — greetd + DMS greeter executor (greetdExecutor)
        plymouth.go             — Aelum LUKS unlock Plymouth theme + .hushlogin + aelum-boot-progress.service (finishes the splash bar)
        services.go             — zram swap config + systemd service enablement
        snapper.go              — snapper configs for @ and @home (quotas, timeline units), the @home .snapshots subvolume, the "after installation" snapshot and the pacman pre/post transaction snapshot hooks; btrfs only
        refind_snapshots.go     — the installed system's snapshot boot entries: aelum-refind-snapshots generator, its systemd unit, and the snapper plugin that runs it; btrfs only
        snapshot_state.go       — the systemd generator that gives a snapshot boot writable state directories (aelum-snapshot-state → tmpfs .mount units), so a snapshot comes up at the greeter; btrfs only
        (+ fstab_test.go, swap_test.go, snapper_test.go, refind_snapshots_test.go, snapshot_state_test.go)
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
      mirrorlist.go             — Embedded mirrorlists per region (falls back to default.list)
      mirrorlist_gen.go         — Mirrorlist refresh script
      logpath.go                — GetLogPath()
      mirrorlist/               — 28 .list files: 25 region files (of the 28 Regions) + zh_Hans/zh_Hant aliases + default.list; ar_SA, ca_ES and eo use default.list. all_https.txt is the raw Arch mirrorlist they are generated from.
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
6. **ConfigureSystem** (weight 4) — Merged step: fstab → system config → initramfs → bootloader → greetd → plymouth → zram → services → snapper (configs + pacman hooks + snapshot boot entries + snapshot-boot state mounts, btrfs only, **last** so the "after installation" snapshot is the finished system and contains everything written here) (`install/configure/`)
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
| `SHARED_DIR` | Copy binaries to this path after build — use `../shared`, the dir the test VM mounts at `/shared` (`build-local.sh` does) |
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

Installed packages include: btop, fastfetch, brightnessctl, ddcutil, cava, matugen, mpv, imv, gnome-text-editor, nautilus, and Noto fonts (CJK + Emoji) + Terminus + JetBrains Mono Nerd. On btrfs installs only, the desktop set also carries `snapper` (base step) and `btrfs-assistant` (desktop step): the layout's snapshots and their GUI. Both must also be listed in `live-iso/packages.x86_64`, since the installer installs from the mirror embedded in the ISO.

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
- Partition table: manual mode needs a GPT. `shared.PartitionTableType` reads it (`blkid -p -o value -s PTTYPE`) and `shared.ValidateManualPartitionTable` rejects any other table in `ComputeLayout`, `ValidateFeasible` and `Apply` — sgdisk refuses to save changes on a non-GPT disk (exit 3), so the rejection must come before anything is deleted. An empty type (no table, or an unreadable device) is allowed: sgdisk writes a GPT there, and Apply re-checks as root. Automatic mode is unaffected — `sgdisk -Z` replaces any table.
- EFI partition: 512MB, vfat, label "EFI", type GUID `C12A7328...`
- Boot floor: every Boot partition (new or existing) must be at least `shared.MinESPSizeBytes` = 512 MiB. Manual mode always formats the selected Boot partition as FAT32, so it never reuses another OS's ESP.
- System floor: `shared.MinSystemPartitionBytes` = 7680 MiB (the 8 GiB disk minimum minus the 512 MiB Boot partition). With LVM it rises to `shared.MinLvmSystemPartitionBytes` (16 GiB + 16 MiB metadata), plus `shared.LuksHeaderReserveBytes` when LUKS sits under it — use `shared.SystemPartitionFloorBytes(lvm, luks)`.
- Root partition: rest of disk, ext4 or btrfs, label "aelum", type GUID `4F68BCE3...` (Linux LVM `E6D6D379...` when it becomes a PV without LUKS)
- LVM: one PV per install (the System partition or its LUKS mapper). The VG is named after the install id — the PV's PARTUUID-derived `<8 hex>`, with no `aelum-` prefix and no hyphen, so device-mapper names carry a single separator (`1a2b3c4d-root`, never `aelum--1a2b3c4d-root`); LVs `root`, plus `home` for ext4 only (btrfs keeps `@home` on the single root LV). LUKS always sits below LVM; fstab/boot reference filesystem UUIDs only, never `/dev/mapper`. Teardown resolves the VG from the PV (`pvs`) and refuses a multi-PV group.
- Btrfs subvolumes: `@` (zstd), `@home` (zstd), `@srv` (zstd), `@cache` (nodatacow), `@tmp` (nodatacow), `@log` (zstd). Swap is provided by zram, not a subvolume.
- **The system is not `@`.** `format.root_snapshot` creates `@/.snapshots` (a subvolume), `@/.snapshots/1/snapshot` (a subvolume, with the info.xml that makes snapper recognise it) and the plain `.snapshots` mount point inside it, then sets the **default subvolume** to that snapshot; the installer mounts and installs into it (`shared.LiveSubvolumeName`). `@` is never mounted or booted — it is the container that keeps the history at one fixed path whatever the live root is. This is the layout `snapper rollback` requires (it names the system by the default subvolume and only accepts a numbered snapshot: `Btrfs::getDefault` → `idToNum`), and it is the sequence snapper's own `installation-helper` runs. `shared.BtrfsSubvolume.SnapshotRoot` marks the one subvolume with this treatment.
- **Nothing pins `/` to a subvolume.** `primitives.BuildFstabMountOpts` writes no `subvol=` for the root mount, `configure.bootEntryOptions` adds no `rootflags=`, and the fstab mounts `@/.snapshots` at `/.snapshots` (`shared.BtrfsSnapshotsMountPoint`). `verify.checkFstab` and `verify.checkBootloader` reject the pinned forms. A rollback then changes what boots without fstab or the menu entry being rewritten — and a pinned `subvol=@` is what made `snapper rollback` inert on the old layout.
- Snapper (btrfs only): configs `root` (@) and `home` (@home) written by `configure.ConfigureSnapper`, each with its own qgroup (`1/0`, `1/1` — `SPACE_LIMIT` is per config, so a shared one would let `@home` snapshots evict system restore points), every limit written as a **range** (`NUMBER_LIMIT="2-10"`: snapper degenerates a single number to `min/max`, and a degenerate range makes `is_quota_aware()`/`is_free_aware()` report *no limits*, silently disabling `SPACE_LIMIT`, `FREE_LIMIT` and `EMPTY_PRE_POST_CLEANUP` together), `TIMELINE_CREATE=no` in both configs (a snapshot is a state someone chose; timeline *cleanup* stays configured so a history that once held one can shed it), `SNAPPER_CONFIGS` in `/etc/conf.d/snapper`, snapperd + the timeline timers enabled, and the `after installation` snapshot (`important=yes`) that "restore to what the installer produced" goes back to. The `@home` history is created by `configure.ensureSnapshotSubvolume`; the root one belongs to the layout and is only checked there — the live root is a snapshot and a snapshot cannot contain a subvolume, so `@/.snapshots` is mounted at `/.snapshots` instead (fstab + `mount.mountRootHistory`). The list lives in `configure.snapperTargets` and is checked against `GetBtrfsSubvolumes()`.
- Transaction snapshots (btrfs only): `/etc/pacman.d/hooks/90-aelum-snapper-pre.hook` + `91-aelum-snapper-post.hook` take a pre/post pair of the `root` config around every pacman transaction, and `92-aelum-snapper-important-post.hook` marks that pair `important=yes` when the transaction touched something a rollback would want (kernels, `glibc`, `systemd`, `btrfs-progs`, `snapper`, `refind`, `mkinitcpio`, firmware, Mesa/Vulkan) — `important` is what survives `NUMBER_LIMIT`, what the menu ranks second, and what a rollback records on its undo snapshot. Paths, triggers and the Exec lines live in `configure.pacman*`; the pre snapshot's number is handed to the post hooks through `/run/aelum-snapper/pacman.pre`, and the pre hook **removes both files before creating them** (a half-written pair from a transaction that died would otherwise be read as this one's). All three call `snapper --no-dbus`: a hook runs as root, snapperd runs as root, and D-Bus only adds a service that can fail. All end in `|| true` — a PreTransaction hook failure aborts the transaction, and a missing snapshot is not a reason to refuse an update.
- Snapshot boot entries (btrfs only): rEFInd shows exactly the stanzas its config contains, so one generator owns them — `/usr/local/bin/aelum-refind-snapshots` (writes `EFI/refind/aelum/<id>-snapshots.conf`), `aelum-refind-snapshots.service` (runs it at every boot) and the snapper plugin `/usr/lib/snapper/plugins/10-aelum-refind-snapshots` (starts the service after every snapshot create/delete). The generator takes `--root <mount>` so it can be run against a system it is not booted in: `configure.writeInitialSnapshotMenu` runs it at the end of the snapper step, after the "after installation" snapshot exists, which is the only way an install's **first** boot has a snapshot entry (rEFInd reads its config before any part of the installed system runs). The installer still writes the fragment **empty** from `writeRefindConfig`, before the include list is collected: refind.conf can only include a file that exists, and the placeholder is what a boot falls back on if that run fails. Names live in `shared.RefindSnapshotEntry*`.
- Snapshot menu shape: **one** entry per install, with the snapshots inside it as `submenuentry` blocks (rEFInd shows those on F2 / Insert / Tab / +). Snapper keeps far more than a menu can hold — `NUMBER_LIMIT` 2–10 plus `NUMBER_LIMIT_IMPORTANT` 4–10 counted **independently**, plus a timeline history cleaned to a handful, plus anything the user takes by hand — so a flat row of icons is not an option. The entry boots the newest snapshot taken *before* a change, else the newest one marked `important=yes`, else the newest one; a `timeline` snapshot is never the entry (a timer that fired is not a state anyone chose to return to) and the states a transaction or a person created are listed before the automatic history fills the remaining slots. The snapshot that **is** the system — the default subvolume, which this install's own entry already boots — is not offered at all: a listed snapshot is a state to go *back to*. Every entry copies loader/initrd/options from the install's own fragment and adds only `rootflags=subvol=`, so LUKS and LVM options stay the installer's business. The install's own entry names **no** subvolume (see the next bullet), and the generator derives the path of the history from `btrfs subvolume get-default /`, not from how `/` is mounted — including when that path is the filesystem tree root, which is what a rollback leaves behind. `refindEntryIncludes` lists an entry before its own snapshots — a filename sort would not, because `-` sorts before `.` — and the entry carries the theme's `os_snapshot.png` history glyph (`shared.RefindSnapshotIconPath`) instead of the Aelum mark, so a snapshot never reads as a second copy of the system.
- Resolver in a snapshot boot (`configure/network.go`, `ConfigureNetworkDNS`, all filesystems, run before the services and **before the "after installation" snapshot**): `/etc/resolv.conf` is replaced with a link to `/run/NetworkManager/resolv.conf`. A snapshot root is read-only and NetworkManager updates resolv.conf by creating a temp file in the same directory and renaming it over, so a plain file there makes a snapshot boot run with **the resolver the snapshot captured** — the empty placeholder for the factory snapshot (taken before the installed system ever had a network), the previous network's nameserver for any other. A link is part of the root and its target is a tmpfs NetworkManager writes in every boot, so DNS belongs to the boot. The target is absolute on purpose: NetworkManager's rc-manager (built as "symlink" on Arch) treats a link to exactly its own path as its own and refreshes it, which is what emits the inotify event other programs watch. Non-fatal if the link cannot be created. Caveat worth knowing: `/etc/resolv.conf` is owned by the `filesystem` package and is not in pacman's backup list, so a `filesystem` upgrade can restore the plain file — that reverts to the old behaviour, it does not break the system.
- A snapshot boot reaches the greeter (`configure/snapshot_state.go`, btrfs only): a snapshot entry mounts a read-only subvolume as `/`, and `systemd-logind`'s state directories are inside that root — without them logind fails at `STATE_DIRECTORY`, hits its start limit and the boot lands in a text login, which is not a recovery entry anyone can use. The installer writes a **systemd generator**, `/usr/lib/systemd/system-generators/aelum-snapshot-state`, which emits a tmpfs `.mount` unit for `/var/lib/systemd` and `/var/lib/greeter`, ordered like a fstab mount, before `systemd-logind`, and **linked from `local-fs.target.requires/`** — a mount unit is started by whatever requires it, and nothing requires a unit nobody has heard of; measured, `/var/lib/systemd` came up only because logind/rfkill/timesyncd/shadow.timer have `RequiresMountsFor=` on it while the greeter's stayed inactive and tmpfiles failed to `fchmod` it. It decides by **subvolume id**: `subvolid=` from the mountinfo entry for `/` against the `ID` from `btrfs subvolume get-default /` — equal is an ordinary boot (write nothing), different is a snapshot boot. Nothing else works at generator time, all measured: `ConditionPathIsReadWrite=!/` is unmet in a real snapshot boot (a btrfs read-only subvolume mounted with `rw` looks read-write to the mount table); and **an ordinary boot's root is mounted read-only for the whole generator phase too** (`… / ro,relatime … - btrfs /dev/vda2 rw,…`, writable only after `systemd-remount-fs`), so a writability probe is not a detector. Two shell traps from the same boot: systemd passes generators **three** arguments (a bare `$2` under `set -u` fails every boot), and `: > file` is a redirection on a special builtin, which **exits** the shell when the write fails — that is how the first version died with status 1. A generator is used rather than a unit because it runs before units are loaded, so what it writes lands in the same window as a fstab mount. It is installed **before** the "after installation" snapshot: a snapshot boots the generators and units the snapshot contains. Not `@var`: `/var/lib/pacman` is the package DB, and a rollback that left it current would make pacman claim the removed packages are installed.
- Default username: `shared.DefaultCLIUsername` = "aelum"
- Default hostname: `shared.DefaultHostname` = "aelum"
- Mount root: `shared.MountRoot` = "/mnt"
- ESP mount during install: `shared.ESPMountPoint` = "/mnt/efi"
- Boot mount: `shared.BootMountPoint` = "/mnt/boot" (bind mount of `<ESP>/aelum/<id>`)
- Installed ESP mount: `shared.InstalledESPMountPoint` = "/efi"
- Per-install ESP boot directory: `shared.EspInstallDirName` = "aelum"; each install uses `aelum/<id>` where `<id>` is the first 8 alphanumeric characters of the root filesystem UUID
- Step weights: `shared.WeightFormatDisk=3`, `WeightMountFilesystem=2`, `WeightInstallBase=37`, `WeightInstallDesktop=50`, `WeightSetupUsers=2`, `WeightConfigureSystem=4`, `WeightClearCache=1`, `WeightVerifySystem=1` (total 100)
