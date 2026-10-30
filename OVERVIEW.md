# Aelum Installer — Project Overview

## Live ISO vs Installed System

**Never confuse these two. It is the #1 source of bugs.**

| Term | What It Is | Key Paths |
|------|-----------|-----------|
| **Live ISO** | Bootable Arch environment running the installer. Read-only rootfs (overlay). | Sudoers: user needs passwordless sudo for CLI binary. |
| **Installed System** | Arch Linux being written to the target disk. | Mounted at `/mnt/` during install. ESP at `/mnt/efi`; this install's `aelum/<id>/` boot directory bind-mounted at `/mnt/boot`. `/etc/sudoers.d/wheel` written here for the new user. Accessed via `arch-chroot /mnt`. |
| **Host** | Developer's main machine. | Where `build.sh` runs, git lives. `../shared` (the distro workspace) is the host side of the VM's `/shared`. |
| **VM** | QEMU VM booting from ISO for testing. | `/shared` mounted from host shared dir. |

**Code rule:** If it touches `/mnt/*` or runs via `arch-chroot` → installed system. If it touches `/dev/*` or runs directly → live ISO.

---

## What It Is

Aelum Installer is a **graphical Arch Linux installer** built with [Wails v2](https://wails.io/) (Go backend + Svelte 5 frontend). It provides a step-by-step wizard to install a custom Arch Linux desktop environment ("Aelum") onto a target disk.

It also ships a **CLI installer** (`aelum-installer-cli`) for headless/automated use. The GUI spawns the CLI binary via `sudo` for all privileged operations — no separate helper binary is needed.

**This is an offline installer.** All packages are pulled from the live ISO's pacman database — no network connectivity is required or used during installation.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **GUI Framework** | [Wails v2](https://wails.io/) (Go + WebView2/WebKit) |
| **Backend** | Go 1.26 |
| **Frontend** | Svelte 5 (TypeScript), Vite, Tailwind-like CSS variables |
| **Target OS** | Arch Linux (installed via `pacstrap`) |
| **Desktop** | Hyprland + Dux Shell (based on Quickshell) + Dux Greeter (greetd) |
| **Bootloader** | rEFInd (auto-detects other OSes for dualboot) |
| **Filesystems** | ext4 or btrfs (with subvolumes) |
| **Boot Splash** | Plymouth, branded Aelum theme (silent boot; LUKS passphrase unlock) |

---

## Architecture

```
┌───────────────────────────────────────────────────────────┐
│                    Wails Application                      │
├──────────────────────┬────────────────────────────────────┤
│   Frontend (Svelte)  │   Backend (Go)                     │
│                      │                                    │
│  6 Wizard Pages      │  GUI Layer (gui/)                  │
│  • Welcome           │    • App struct (state mgmt)       │
│  • User Account      │    • Validation (delegate)         │
│  • Disk Selection    │    • Navigation                    │
│  • Confirmation      │    • Root check                    │
│  • Installation      │    • CLI spawning + progress       │
│  • Finish            │      (JSON events from CLI stderr) │
│                      │  Install Layer (install/)          │
│                      │    • Installer (orchestrator)      │
│                      │    • Step interface + impls        │
│                      │    • Plan (weights/percentages)    │
│                      │    • Executor interface            │
│                      │    • Logger interface              │
│                      │    • format, mount, fstab          │
│                      │    • install_base, install_desktop │
│                      │    • bootloader, users, system     │
│                      │    • greetd, plymouth, services    │
│                      │    • zram, copy (ISO files)        │
│                      │    • cleanup, rollback             │
│                      │    • command (Run/runChroot)       │
│                      │    • config (constants)            │
│                      │    • context (InstallContext)      │
│                      │    • checks (validation helpers)   │
│                      │    • progress (pacstrap tracker)   │
│                      │                                    │
│                      │  Shared Layer (shared/)            │
│                      │    • types, config, validators     │
│                      │    • disks (lsblk), regions,       │
│                      │      locales, btrfs, mirrors       │
└──────────────────────┴────────────────────────────────────┘
                          │
                          ▼
              ┌──────────────────────────┐
              │ aelum-installer-cli│
              │ (sudo-privileged)        │
              │ • Direct exec.Command    │
              │ • arch-chroot /mnt       │
              │ • JSON progress → stderr │
              └──────────────────────────┘
```

### Two Binaries

| Binary | Purpose | Entry Point |
|--------|---------|-------------|
| `aelum-installer` | GUI wizard (Wails) | `main.go` |
| `aelum-installer-cli` | CLI automated install | `backend/cli/main.go` |

The GUI spawns the CLI via `sudo` — all privileged operations run inside the CLI process.

---

## Installation Steps (6 top-level, 12 logical)

The `Installer.Install()` method runs 6 top-level steps sequentially. The last step merges 7 configuration sub-steps into one:

| # | Step | Weight | Function | Description |
|---|------|--------|----------|-------------|
| 1 | Format Disk | 3 | `FormatDisk` | Partitions (GPT: EFI 512MB + Root rest), formats (vfat/ext4/btrfs), btrfs subvolumes |
| 2 | Mount Filesystem | 2 | `MountFilesystem` | Mounts root + btrfs subvolumes to `/mnt`, the ESP to `/mnt/efi`, and this install's `aelum/<id>/` ESP directory at `/mnt/boot` |
| 3 | Install Base | 45 | `Install` | `pacstrap /mnt base linux-lts linux-firmware ...` |
| 4 | Install Desktop | 35 | `InstallDesktop` | `pacstrap` Hyprland, Dux Greeter, kitty, firefox, pipewire, etc. |
| 5 | Setup Users | 2 | `SetupUsers` | Creates user (wheel group), sets passwords, copies ISO skel files, sudoers |
| 6 | Configure System | 13 | `ConfigureSystem` | Merged: fstab → system config → initramfs → bootloader → greetd → Plymouth → zram → resolver link → services → snapper (configs, pacman hooks, snapshot boot entries, snapshot-boot state mounts) — snapper last, see 6.9 |

**Step 6 sub-steps (in order):**

| # | Sub-step | Function | Description |
|---|----------|----------|-------------|
| 6.1 | Generate fstab | `GenerateFstab` | Writes `/etc/fstab` (custom for btrfs subvolumes): ESP at `/efi`, this install's `aelum/<id>/` boot dir bind-mounted at `/boot` |
| 6.2 | Configure System | `ConfigureSystemBase` | Hostname, timezone, locale, keymap, NTP, mirrorlist |
| 6.3 | Configure initramfs | `configureInitramfs` | HOOKS: plymouth always, `sd-encrypt` with LUKS, `lvm2` with LVM (after the encrypt hook); regenerates the images the boot entry will reference |
| 6.4 | Configure Bootloader | `ConfigureBootloader` | rEFInd install onto the ESP, per-install `aelum/<id>.conf` menu fragment (loads this install's own `aelum/<id>/` kernel), the empty `aelum/<id>-snapshots.conf` a btrfs install's refind.conf includes, NVRAM boot entry |
| 6.5 | Configure greetd | `ConfigureGreetd` | Writes `/etc/greetd/config.toml`, creates greeter user, syncs Dux Greeter |
| 6.6 | Configure Plymouth | `ConfigurePlymouth` | Aelum theme, mkinitcpio hook |
| 6.7 | Configure zram | `ConfigureZram` | Compressed swap via zram-generator |
| 6.8 | Enable Services | `EnableServices` | Enables NetworkManager, greetd, bluetooth — **before** the snapshot below: an enable step that runs after it is simply absent from the state a user restores to |
| 6.9 | Configure Snapper | `ConfigureSnapper` | btrfs only, and last on purpose: snapper configs for `@` and `@home`, their `.snapshots` subvolumes, quotas, `SNAPPER_CONFIGS`, snapperd + timeline timers, the pacman pre/post transaction snapshot hooks, the snapshot boot entries (generator + unit + snapper plugin), the generator that gives a snapshot boot writable state directories, then the "after installation" snapshot — a snapshot only contains what was written before it was taken |

### Progress Tracking

The two pacstrap steps (base + desktop) together account for ~80% of total installation time. Progress is tracked via:
- **Line-count-based pacstrap tracker**: parses actual pacstrap output lines (downloading, verifying, installing, hooks) for smooth, accurate progress
- **Weighted step percentages**: base pacstrap (45), desktop pacstrap (35), remaining steps (20 combined)
- **Tick goroutine**: advances percentage during long steps to keep the UI feeling alive

---

## Frontend Pages (Wizard Flow)

```
Welcome (language select)
    → User Account (username + password)
        → Disk Selection (disk cards + filesystem pills)
            → Confirmation (summary + root check + confirm dialog)
                → Installation (progress bar + live events)
                    → Finish (success)
```

### Key Frontend Files

| File | Purpose |
|------|---------|
| `src/App.svelte` | Root component, page routing, navigation events, start page support |
| `src/lib/state.svelte.ts` | Svelte stores for user account, disk selection, page state |
| `src/lib/types.ts` | TypeScript interfaces, page IDs, constants |
| `src/lib/navigation.ts` | Frontend validation calls to Go backend |
| `src/lib/validate.ts` | Client-side form validation |
| `src/pages/*.svelte` | 6 wizard pages |
| `src/components/*.svelte` | Reusable components (Button, Dropdown, PageLayout, ConfirmDialog, ErrorBanner, etc.) |

---

## Key Design Decisions

### CLI Spawning Pattern
The GUI spawns `aelum-installer-cli` via `sudo` for all privileged operations. The CLI runs as root and:
- Executes system commands directly via `exec.Command`
- Runs commands inside the installed system via `arch-chroot /mnt`
- Emits JSON progress events to stderr (read by the GUI)
- Writes human-readable output to stdout (forwarded to terminal by the GUI)

This eliminates the need for a separate helper binary and its command whitelist security surface.

### Executor Interface
The `Executor` interface in `backend/install/executor.go` abstracts command execution:
- `Run(args...)` — executes a command directly
- `RunChroot(args...)` — executes inside the installed system via `arch-chroot`
- `WriteFile(path, content)` — writes a file inside the installed system
- `AppendFile(path, content)` — appends to a file inside the installed system

The default implementation is `HostExecutor`. This interface enables testing with mock implementations.

### Chroot via arch-chroot
System configuration inside the installed system uses `arch-chroot /mnt` directly. This handles `/proc`, `/sys`, `/dev` mounting automatically.

### ISO File Copying
Static files from the live ISO are copied to the installed system during `SetupUsers`:
- `/etc/skel/` → user home directory
- `/etc/dconf/profile/user` and `01-icons`
- Dux Shell wallpaper
- `/etc/environment`
- `/etc/greetd/hypr.lua`

### Dry Run Mode
Set `AELUM_DRY_RUN=1` environment variable to simulate installation (steps sleep 200ms instead of executing). Useful for UI testing.

### Fake Disks
Set `FAKE_DISKS=1` to get synthetic disk data for frontend development without real hardware.

### Start Page
The installer supports starting on any page via the `StartPage` constant in `main.go` (0=Welcome, 1=User Account, etc.). Set to `-1` for default Welcome page. The GUI exposes `GetStartPage()` and `SetStartPage()` Wails bindings.

### btrfs Subvolumes
When btrfs is selected, 6 subvolumes are created: `@`, `@home`, `@srv`, `@cache`, `@tmp`, `@log`. Each has configurable compression (zstd) and mount options. `@cache` and `@tmp` use `nodatacow` to avoid CoW overhead. Swap is provided by zram, not a subvolume.

**The system is installed into `@/.snapshots/1/snapshot`, not into `@`.** `format.root_snapshot` creates the history (`@/.snapshots`, a subvolume), the initial snapshot inside it, the `info.xml` that makes snapper recognise that snapshot, and the mount point `/.snapshots` inside it; the filesystem's **default subvolume** is then set to the snapshot, and the installer mounts and installs into it. `@` is never mounted and never booted — it is the container that keeps the history at one fixed path whatever the live root is. Nothing pins `/` to a subvolume: the fstab and the rEFInd entry name none, so the boot follows the default subvolume, and that pointer is exactly what a rollback moves. This is the layout `snapper rollback` requires and the sequence snapper's own `installation-helper` runs; with `@` as the live root snapper cannot name the system at all and `snapper rollback` refuses to run ("Cannot detect ambit since default subvolume is unknown").

`@` and `@home` get a snapper config (`root`, `home`) written by `configure.ConfigureSnapper`, each with its own `.snapshots` subvolume, its own btrfs qgroup (`1/0` for the root, `1/1` for `@home` — `SPACE_LIMIT` is per config, so a shared group would let `@home` snapshots evict system restore points), and the `timeline` **cleanup** ranges; `TIMELINE_CREATE` is `no` in both configs, so a snapshot exists because something changed or because a user asked, never because a timer fired. Every limit is written as a range (`NUMBER_LIMIT="2-10"`), because snapper turns a single number into `min/max` internally and a degenerate range makes `is_quota_aware()` and `is_free_aware()` report the config as having no limits at all — which silently disables `SPACE_LIMIT`, `FREE_LIMIT` and `EMPTY_PRE_POST_CLEANUP` together. `SNAPPER_CONFIGS`, snapperd and the timeline timers are enabled, and an `after installation` snapshot marked `important=yes` makes "back to the state the installer produced" a thing that always exists. The other four are deliberately not snapshotted: they are separate subvolumes, so a snapshot of `@` never contains them, and no restore wants an old package cache, scratch file, journal record or server data directory.

### Snapshots You Can Boot and Undo (btrfs only)

**Around every pacman transaction**, `/etc/pacman.d/hooks/90-aelum-snapper-pre.hook` takes a pre snapshot of `@` and `91-aelum-snapper-post.hook` takes the post half, so `snapper undochange` can show exactly what an update changed. Both hooks end in `|| true`: a PreTransaction hook failure aborts the transaction, and a snapshot that cannot be taken is not a reason to refuse an update. The pair is tagged with the `number` cleanup algorithm, so an update history is bounded by `NUMBER_LIMIT` instead of growing forever, and pairs whose transaction changed nothing are dropped by `EMPTY_PRE_POST_CLEANUP`. The pre hook removes the pair files it is about to write (`/run/aelum-snapper/pacman.{pre,pair}`) before creating them: a half-written pair from a transaction that died would otherwise be read as this transaction's. All three hooks call `snapper --no-dbus` — a hook runs as root, snapperd runs as root, and the D-Bus path only adds a service that can fail. A **third** hook, `92-aelum-snapper-important-post.hook`, marks that pair `important=yes` when the transaction touched something a rollback would want (the kernel packages, `glibc`, `systemd`, `btrfs-progs`, `snapper`, `refind`, `mkinitcpio`, the firmware, the Mesa and Vulkan drivers): `important` is what keeps a snapshot past `NUMBER_LIMIT`, what the menu ranks second, and what `snapper rollback` records on its own undo snapshot — without it, a kernel update that broke the system is evicted by the next three updates.

**In the boot menu**, rEFInd shows exactly the stanzas its configuration contains, so one generator owns them: `/usr/local/bin/aelum-refind-snapshots` writes this install's snapshot menu fragment (`EFI/refind/aelum/<id>-snapshots.conf`), `aelum-refind-snapshots.service` runs it at every boot, and `/usr/lib/snapper/plugins/10-aelum-refind-snapshots` starts it after every snapshot snapper creates or deletes. The installer writes that fragment **empty** (refind.conf can only include a file that exists) and then runs the script itself against the system it is still installing — `aelum-refind-snapshots --root /mnt`, from `configure.writeInitialSnapshotMenu`, after the `after installation` snapshot exists. That call is the whole reason an install's **first** boot has a recovery entry: rEFInd reads its configuration before any part of the installed system runs, so a menu first written at boot is one boot behind the menu that boot could have booted. It is not a second implementation: the same file the installed system runs is the one the installer runs, and every path it resolves is under the root it was given.

The fragment holds **one** menu entry, with the snapshots inside it as `submenuentry` blocks that rEFInd shows on F2 / Insert / Tab / +. That is the only shape that scales: snapper's limits keep between 2 and 10 snapshots by `number`, another 4 to 10 *important* ones counted independently, and a timeline history cleaned down to a handful — before counting anything the user takes by hand. So the list is ranked, not truncated: the states a transaction or a person created come first and the automatic history fills whatever room is left.

The entry itself boots the newest snapshot taken **before** a change, else the newest one marked `important=yes`, else the newest one — because the newest snapshot is usually the state that already contains the change. A `timeline` snapshot is never the entry: a timer that fired at 03:00 is not a state anyone chose to be able to return to, though it stays in the submenu, which is what the history is for. And the snapshot that **is** the system is not offered at all: the default subvolume is what this install's own entry already boots and what a rollback moves, so listing it would offer the user the state they are already in. Every title names exactly which snapshot it is. `refindEntryIncludes` keeps an entry and its snapshots adjacent in that order (a filename sort would not: `-` sorts before `.`), and the entry carries the theme's history glyph (`icons/os_snapshot.png`) rather than the Aelum mark.

An entry copies this install's own loader, initramfs and kernel options and **adds** only `rootflags=subvol=` — the install's own entry names no subvolume at all — so LUKS and LVM options stay correct without the generator knowing anything about them, and the current kernel on the ESP boots the older root. A `submenuentry` inherits loader, initrd and icon from the entry it sits in, so it only states which subvolume becomes root. Snapper's snapshots are read-only subvolumes, so the system comes up with `/` read-only — enough to log in, compare and copy; `snapper rollback` (or btrfs-assistant) is what makes one the system again: it points the default subvolume at a writable copy of the chosen snapshot, and the install's own entry boots that copy unchanged. `/home`, `/srv`, `/var/cache`, `/var/tmp` and `/var/log` are separate subvolumes and stay the live ones, and `@/.snapshots` is mounted at `/.snapshots` so the history is readable from a snapshot and from a restored system alike. With no snapshots at all the fragment holds no entry — an icon that boots nothing is worse than no icon.

**A snapshot boot resolves on the network it is running on** (`configure/network.go`). `/etc/resolv.conf` is a regular file inside the root — Arch ships it as a placeholder, NetworkManager fills it at boot, `systemd-resolved` is not installed — and NetworkManager updates it the way any careful program updates a file: create a temp file in the same directory, rename it over. A read-only root refuses that, so a plain file there makes a snapshot boot run with **the resolver the snapshot happened to capture**: measured before this existed, the factory snapshot ("after installation", taken before the installed system ever had a network boot) held the empty placeholder and resolved nothing, while the nameserver NetworkManager had just computed sat in `/run/NetworkManager/resolv.conf` with nothing pointing at it. The installer replaces the file with a **link to that path**, before the services are enabled and before the factory snapshot is taken, so the link is inside every snapshot and what it points at is written fresh in every boot on a filesystem that is never read-only. The target is absolute rather than the relative form systemd uses for its own resolved link: NetworkManager on Arch is built with `rc-manager` defaulting to `symlink`, and it treats a link to exactly its own path as its own and refreshes it — which is what emits the inotify notification other programs watch. Measured on a real snapshot boot of an install built with it: the factory snapshot carries the link, and the snapshot boot fetches a page (`curl https://archlinux.org → HTTP 200`). It is non-fatal: an install that could not create the link is an install with the old behaviour. The one caveat, documented rather than engineered around: `/etc/resolv.conf` is owned by the `filesystem` package and is not in pacman's config backup list, so a `filesystem` upgrade can put the plain file back — which restores the old behaviour, it does not break anything. A tmpfiles `L+` rule to re-create the link was measured and rejected: it logs a warning on every snapshot boot to repair something that is already correct.

**A snapshot boot reaches the greeter, not a text login** (`configure/snapshot_state.go`). What a snapshot boot does *not* keep separate is `/var/lib`, which is inside the root — and `systemd-logind` keeps its state directories there. Measured on a real snapshot boot, logind fails at `STATE_DIRECTORY` on the read-only root, retries to its start limit and the boot lands on `getty@tty1`: a recovery entry a user cannot log into. The installer therefore installs a **systemd generator**, `/usr/lib/systemd/system-generators/aelum-snapshot-state`, which emits a tmpfs `.mount` unit for each of `/var/lib/systemd` and `/var/lib/greeter` — ordered like a fstab mount, and before `systemd-logind`, which is the unit all of this exists for. It decides by **subvolume id**: `subvolid=` from the `/proc/self/mountinfo` entry for `/`, against the `ID` from `btrfs subvolume get-default /`. Equal means an ordinary boot and it writes nothing; different means the root is a snapshot and it writes the mounts. Nothing subtler works at generator time, all of it measured on a real boot: `ConditionPathIsReadWrite=!/` is unmet in a snapshot boot (a btrfs read-only subvolume mounted with `rw` looks read-write to the mount table), and an **ordinary boot has its root mounted read-only for the whole generator phase too** (`… / ro,relatime … - btrfs /dev/vda2 rw,…`, writable only once `systemd-remount-fs` has run), so a writability probe cannot tell the two apart. A generator is used rather than a unit because it runs before units are loaded, which is the only window in which a mount can land before logind. It is installed *before* the "after installation" snapshot, because a snapshot boots the generators and units the snapshot contains. Verified in real boots of the test disk: a snapshot entry (`rootflags=subvol=@/.snapshots/10/snapshot`) mounts `var-lib-systemd.mount`, starts logind and comes up at greetd/Hyprland; an ordinary boot gets no unit at all and its real directories stay mounted. The alternative — a separate `@var` subvolume, which is what SUSE does — is deliberately not taken: `/var/lib/pacman` is the package database, and a root rollback that left it current would make pacman claim the packages the rollback removed are installed.

**A snapshot boot asks whether to keep it** (`configure/snapshot_restore.go` + `configure/snapshot_prompt.go`).
The generator is the only thing that knows a snapshot boot is one at the moment it happens, so it also
writes `/run/aelum-snapshot-boot` with `SNAPSHOT=<number>`, cut from the same mountinfo entry it detects
with - a tmpfs file that belongs to one boot, is never written into the snapshot, and is gone at the next
one. The dialog and its unit are system-side, because nothing about them is per-user and a file in one
account's home belongs to that account - delete the user and create a new one, and the new user has no
prompt. (The session pull-in is the deliberate exception - per-home, see below.) `/etc/xdg/quickshell/aelum-snapshot/shell.qml` - Quickshell scans every XDG config directory,
`/etc/xdg` included, and its distribution guide tells packaged configs to live there - holds a
plain-QtQuick dialog - "You booted snapshot N. Restore this state as the system?" with **OK** and
**CANCEL**. Its colors are the user's live DMS scheme: the QML reads
`~/.cache/dms/dms-colors.json` - the file DMS writes whenever matugen recolors the desktop - watches it for changes, and maps
the dialog to Material color roles (`surface_container`, `primary`/`on_primary`, `outline_variant`, `error`), so the prompt is
always the current theme in whichever mode the desktop is in and recolors live if the theme changes while it is open; only a
missing or unreadable file falls back, per role, to the seeded brand palette. It is a real window - a
Quickshell `FloatingWindow` titled `aelum-snapshot-prompt`, not a layer surface: a layer surface is
anchored to the screen edges and the compositor cannot move one, which is what made the first version
impossible to drag. The seeded `hyprland.lua` matches that title with a `float` + `center` window rule,
and pressing the card body calls `startSystemMove()` (xdg_toplevel's `start_move`, honored by Hyprland
0.56+), so the dialog is centered when it opens and draggable from there with no compositor-side mouse
bind. It is run by
`/etc/systemd/user/aelum-snapshot-prompt.service`, a unit in a load path of every user manager, but
pulled in **per-home**: the wants link seeded into
`/etc/skel/.config/systemd/user/hyprland-session.target.wants/` - the same mechanism that starts
`dms.service` - is copied into every real home, including one created after the install, and no
system-side link is written: the greeter runs a Hyprland session too, is not in `wheel`, and did not
choose a snapshot, so it is not asked. It is gated with
`ConditionPathExists=/run/aelum-snapshot-boot`, so an ordinary boot never starts it, and the snapshot
number arrives through that same file. It is a standalone Quickshell instance (`QS_INSTANCE_ID`,
`quickshell -c aelum-snapshot`), not a Dux Shell widget: DMS has no modal-dialog API and its
`settings.json` is a seeded baseline, while Quickshell is DMS's own engine and ships as its dependency.
OK runs `pkexec /usr/local/bin/aelum-snapshot-restore` - the rollback wrapped once as
`snapper --no-dbus -c root rollback`, with no number because from inside the snapshot that means exactly
"the state I am booted in", which is why the helper refuses to run when the marker is absent. The root
access is polkit's, not sudo's: the installer writes
`/usr/share/polkit-1/actions/dev.aelum.snapshot.restore.policy`, whose defaults refuse the action
everywhere and whose `exec.path` annotation is what makes `pkexec <helper>` resolve to it, and
`/etc/polkit-1/rules.d/aelum-snapshot-restore.rules`, the only grant - `subject.local && subject.active
&& subject.isInGroup("wheel")`, the session of the person who pressed OK (Arch's admin group is `wheel`;
polkit's rule language has no `user_is_admin` helper - a rule calling it throws and the policy's refusal
answers).
An SSH session, the greeter, and every other user get the refusal. (A NOPASSWD sudoers drop-in was the
first version and failed on the real install: sudoers is last-match-wins, and the installer's own
`/etc/sudoers.d/wheel` sorts after `aelum-snapshot-restore`.) After a
successful rollback the dialog offers the reboot that boots the restored state. CANCEL closes and changes
nothing, which is the point of a snapshot boot. The helper, its polkit files and the prompt files are
root-side, written *before* the factory snapshot - a snapshot runs the binaries, rules and prompt it
contains - and
`docker/scripts/test-snapshot-rollback.sh` asserts the polkit scope and runs the exact command pkexec
execs, against the real disk.

### Partition Layout
Both ext4 and btrfs use the same 2-partition GPT layout:
- **Partition 1**: 512MB EFI (vfat, type GUID `C12A7328...`)
- **Partition 2**: Rest of disk (ext4/btrfs, type GUID `4F68BCE3...`)

### Mirrorlists
Region-specific mirrorlists are embedded in the binary. A `mirrorlist_gen.go` script refreshes them from `archlinux.org/mirrorlist/all/https/`.

---

## Shared Host/VM Setup

- **Host**: `../shared` — the distro workspace, one level above this repo
- **VM**: `/shared` (the 9p mount of that directory, set up by `docker/scripts/test-install.sh`)
- **ISO bin dir**: `../live-iso/airootfs/usr/local/bin`
- `build-local.sh` builds and publishes the binaries into `../shared`; `SHARED_INSTALLER=1 make -C docker test-one` then drives the install with them instead of the CLI baked into the ISO.

---

## Building

```bash
./build.sh
```

This runs:
1. Mirrorlist refresh (`mirrorlist_gen.go`)
2. TypeScript constants generation (`gen-ts-constants.go`)
3. Frontend `npm install` + `npm run build`
4. `wails build -clean -tags webkit2_41` (GUI binary)
5. `go build` for CLI binary
6. Copies binaries to shared dir and ISO bin dir (if `SHARED_DIR`/`BIN_DIR` set)

---

## Testing

```bash
go test ./...          # Run all Go tests (unit + integration)
./run.sh               # Start Wails dev server (hot-reload)
```

### Test Types

| Type | Location | What it covers |
|------|----------|----------------|
| Unit | `backend/*/ *_test.go` | Validators, progress tracker, disk parsing, context |
| Integration | `backend/install/installer_integration_test.go` | Full install flow (dry-run), step ordering, progress, error handling, rollback |
| GUI | `backend/gui/*_test.go` | Validation logic, root check |
| Frontend | `frontend/` (Vitest) | Component tests |

---

## Project Structure

```
Aelum-Installer/
├── main.go                      # Wails app entry point (StartPage support)
├── wails.json                   # Wails config
├── build.sh / run.sh            # Build & dev scripts
├── RULES.md                     # Development rules
├── backend/
│   ├── cli/main.go              # CLI installer
│   ├── gui/
│   │   ├── app.go               # Wails app struct, reboot, start page
│   │   ├── bindings.go          # Wails API bindings (disks, languages, fs)
│   │   ├── installation.go      # StartInstallation (spawns CLI via sudo)
│   │   ├── navigation.go        # Page IDs, navigation state
│   │   ├── root_check.go        # Sudo verification
│   │   ├── validation.go        # Validate + Save methods
│   │   └── logger.go            # GUILogger
│   ├── install/
│   │   ├── installer.go         # Installer orchestrator (shared by CLI/GUI)
│   │   ├── step.go              # Step interface + step implementations
│   │   ├── plan.go              # Plan struct for weights/percentages
│   │   ├── executor.go          # Executor interface
│   │   ├── configuration.go     # Merged ConfigureSystem
│   │   ├── errors.go            # ValidationError, InstallError
│   │   ├── logger.go            # Logger interface + ConsoleLogger
│   │   ├── format.go            # Partitioning + formatting
│   │   ├── mount.go             # Mount filesystem + verification
│   │   ├── install_base.go      # pacstrap base packages
│   │   ├── install_desktop.go   # pacstrap desktop packages
│   │   ├── install.go           # Pacstrap progress tracking
│   │   ├── fstab.go             # fstab generation + verification
│   │   ├── bootloader.go        # rEFInd install + branded theme + per-install menu fragment
│   │   ├── users.go             # User account creation + ISO copy
│   │   ├── system.go            # Hostname, locale, timezone, mirrors
│   │   ├── greetd.go            # Login manager config + Dux Greeter sync
│   │   ├── greetd/              # greetd package
│   │   │   ├── greetd.go        # ConfigureGreetd + SyncDmsGreeter
│   │   │   ├── config.go        # Embedded config.toml
│   │   │   ├── sync.go          # Dux Greeter sync
│   │   │   └── memory.go        # Embedded memory.json
│   │   ├── plymouth.go          # Silent boot (Aelum theme)
│   │   ├── services.go          # systemd service enabling
│   │   ├── snapper.go           # Snapper configs + pacman transaction snapshot hooks
│   │   ├── refind_snapshots.go  # Snapshot boot entries: generator, unit, snapper plugin
│   │   ├── network.go           # /etc/resolv.conf linked into /run, so a snapshot boot has DNS
│   │   ├── snapshot_state.go    # tmpfs over read-only state dirs, so a snapshot boots to the greeter;
│   │   │                        # also announces the boot + snapshot number in /run
│   │   ├── snapshot_restore.go  # aelum-snapshot-restore + its polkit policy/rule (the OK button, via pkexec)
│   │   ├── snapshot_prompt.go   # the Quickshell restore prompt + its gated user unit
│   │   ├── zram.go              # Compressed swap via zram-generator
│   │   ├── copy.go              # ISO-to-target file copying
│   │   ├── cleanup.go           # Rollback + mount cleanup
│   │   ├── primitives.go        # Run/RunChroot/WriteFile/AppendFile/CleanupMounts
│   │   ├── rollback.go          # Rollback partial installations
│   │   ├── context.go           # InstallContext
│   │   ├── checks.go            # Validation helpers
│   │   ├── root_check.go        # RootCheckResult
│   │   └── progress/
│   │       └── pacstrap.go      # PacstrapProgressTracker
│   ├── validation/
│   │   └── validation.go        # Centralized validation
│   └── shared/
│       ├── types.go             # Data structures (DiskInfo, Region, etc.)
│       ├── config.go            # Constants + thresholds
│       ├── disks.go             # lsblk parsing + fake data
│       ├── regions.go           # 28 locale/region definitions
│       ├── locales.go
│       ├── btrfs.go             # Subvolume definitions
│       ├── mirrorlist.go        # Mirrorlist handling
│       ├── mirrorlist_gen.go    # Mirrorlist refresh script
│       ├── validators.go        # Input validation
│       └── logpath.go
└── frontend/
    ├── src/
    │   ├── App.svelte           # Root component
    │   ├── main.ts              # Entry point
    │   ├── lib/
    │   │   ├── state.svelte.ts  # Svelte stores
    │   │   ├── types.ts         # TS types + constants
    │   │   ├── navigation.ts    # Nav + validation calls
    │   │   └── validate.ts      # Client validation
    │   ├── pages/               # 6 wizard pages
    │   └── components/          # Reusable UI components
    └── wailsjs/                 # Auto-generated Wails bindings
```

---

## Desktop Environment (Aelum)

The installed system includes:
- **Window Manager**: Hyprland (Wayland compositor)
- **Shell**: Dux Shell (based on Quickshell, package `dux-shell`)
- **Login Manager**: greetd + Dux Greeter
- **Terminal**: Kitty
- **File Manager**: Nautilus
- **Browser**: Firefox
- **Audio**: PipeWire + WirePlumber
- **Utilities**: btop, fastfetch, brightnessctl, ddcutil, cava, matugen
- **Editor**: gnome-text-editor
- **Media**: mpv, imv
- **Fonts**: Noto (including CJK + Emoji) + Terminus + JetBrains Mono Nerd
- **Boot Splash**: Plymouth with the branded Aelum theme (silent boot; LUKS passphrase splash on encrypted installs)
