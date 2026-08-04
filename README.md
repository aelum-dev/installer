# AestheticOS Installer

A graphical Arch Linux installer with a polished UI, built with [Wails v2](https://wails.io/) (Go + Svelte 5).

Paired with the [AestheticOS](https://github.com/KornienkoDev/AestheticOS) distro — a fully offline Arch Linux ISO that requires zero internet during installation.

## Features

- **6-step wizard** — Welcome, User Account, Disk Selection, Confirmation, Installation, Finish
- **ext4 and btrfs** — Choose your root filesystem; btrfs includes automatic subvolume layout (`@`, `@home`, `@srv`, `@cache`, `@tmp`, `@log`, `@swap`)
- **Privileged helper** — All root operations go through a sandboxed helper binary with command whitelists (no full root access for the GUI)
- **CLI mode** — Headless automated installs via `aestheticos-installer-cli`
- **Progress tracking** — Line-count-based pacstrap progress for accurate percentages
- **Dry-run mode** — Simulate the full install for UI testing (`AESTHETIC_DRY_RUN=1`)

## Desktop Environment

The installed system ships with:

| Component | Package |
|-----------|---------|
| Window Manager | Hyprland |
| Shell | DankMaterialShell (Quickshell-based) |
| Login Manager | greetd + DMS greeter |
| Terminal | Alacritty |
| Browser | Firefox |
| Audio | PipeWire + WirePlumber |
| Boot Splash | Plymouth BGRT (silent boot with firmware logo) |

## Project Structure

```
AestheticOS-Installer/
├── backend/
│   ├── cmd/
│   │   ├── helper/          # Privileged helper (sudo-privileged system ops)
│   │   └── installer-cli/   # CLI installer
│   ├── gui/                 # Wails GUI layer (bindings, navigation, validation)
│   └── internal/
│       ├── core/            # Installer orchestrator (shared by CLI + GUI)
│       ├── install/         # Installation steps (format, mount, pacstrap, etc.)
│       └── shared/          # Types, validators, disk detection, regions
└── frontend/
    ├── src/
    │   ├── pages/           # 6 wizard pages
    │   ├── components/      # Reusable UI components
    │   └── lib/             # State, navigation, validation
    └── wailsjs/             # Auto-generated Wails bindings
```

## Building

### Prerequisites

- Go 1.25+
- Node.js 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- `webkit2gtk-4.1` (Linux) or `WebView2` (Windows)

### Build all binaries

```bash
./build.sh
```

This produces three binaries in `build/bin/`:

| Binary | Purpose |
|--------|---------|
| `aestheticos-installer` | GUI wizard (Wails app) |
| `aestheticos-installer-cli` | CLI automated installer |
| `aestheticos-helper` | Privileged helper (run via sudo) |

### Development mode

```bash
./run.sh
```

Starts the Wails dev server with hot-reload for both frontend and backend.

### Frontend only

```bash
cd frontend
npm install
npm run dev      # Dev server
npm run build    # Production build
npm run test     # Run Vitest tests
```

## Usage

### GUI

```bash
# Run as root (helper requires sudo)
sudo ./build/bin/aestheticos-installer
```

### CLI

```bash
# Full installation (password entered interactively)
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda

# With btrfs
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda --filesystem btrfs

# Dry run (simulate, no changes)
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda --dry-run

# Non-interactive: pipe password via stdin
sudo ./build/bin/aestheticos-installer-cli --install /dev/vda <<< 'mypassword'

# Rollback a partial installation (cleans up mounts + wipes partition signatures)
sudo ./build/bin/aestheticos-installer-cli --rollback /dev/vda

# Single step
sudo ./build/bin/aestheticos-installer-cli --format-disk /dev/vda
sudo ./build/bin/aestheticos-installer-cli --install-bootloader /dev/vda
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `AESTHETIC_DRY_RUN=1` | Simulate installation (steps sleep 200ms instead of executing) |
| `SHARED_DIR` | Copy binaries to this path after build (e.g. VM shared folder) |
| `BIN_DIR` | Copy binaries to distro airootfs bin dir after build |
| `FAKE_DISKS=1` | Use synthetic disk data (for frontend development) |

### Security Note

The CLI **never accepts the password as a command-line flag**. Passwords are read from the terminal (masked) or from stdin (piped). This prevents the password from appearing in `/proc`, `ps` output, or shell history.

## Installation Steps

The installer runs 12 steps sequentially:

| # | Step | Description |
|---|------|-------------|
| 1 | Format Disk | GPT partitioning (EFI 128MB + root), format (vfat/ext4/btrfs) |
| 2 | Mount Filesystem | Mount root + subvolumes to `/mnt`, ESP to `/mnt/boot` |
| 3 | Create Swap | 4GB swap file (btrfs: inside `@swap` subvolume) |
| 4 | Install Base | `pacstrap` base packages + linux-lts |
| 5 | Install Desktop | `pacstrap` Hyprland, DMS greeter, Firefox, PipeWire, etc. |
| 6 | Generate fstab | Custom fstab (btrfs subvolume-aware) |
| 7 | Configure Bootloader | systemd-boot install + boot entry |
| 8 | Setup Users | Create user (wheel group), set passwords, sudoers |
| 9 | Configure System | Hostname, timezone, locale, keymap, mirrorlist |
| 10 | Configure greetd | Write config, create greeter user, sync DMS greeter |
| 11 | Configure Plymouth | BGRT theme, mkinitcpio hook, `.hushlogin` |
| 12 | Enable Services | NetworkManager, greetd, bluetooth |

## Idempotency & Rollback

The installer is **idempotent** — if it fails mid-install, running it again will clean up and start fresh:

- Mounts are automatically torn down on any failure
- The `--rollback` flag manually cleans up a partial installation:
  ```bash
  sudo ./aestheticos-installer-cli --rollback /dev/vda
  ```
  This unmounts everything under `/mnt`, wipes filesystem signatures from both partitions, and re-probes the partition table. The disk is then ready for a fresh install.

## Security Design

The installer uses a **privileged helper pattern** to minimize the attack surface:

- The GUI runs as root but cannot directly execute system commands
- All privileged operations go through `aestheticos-helper`, invoked via sudo
- The helper uses **command whitelists** — only explicitly allowed commands can run
- Two whitelists: `runWhitelist` (host-level) and `chrootRunWhitelist` (inside chroot)
- Passwords are never logged; they're passed via stdin where possible

### Sudoers Configuration

On the live ISO, `/etc/sudoers.d/aestheticos` allows the helper to run without a password:

```
%sudo ALL=(ALL) NOPASSWD: /usr/local/bin/aestheticos-helper
```

On the installed system, `/etc/sudoers.d/wheel` is written for the new user:

```
%wheel ALL=(ALL:ALL) ALL
```

## Testing

```bash
# Run all Go tests (unit + integration)
go test ./...

# Run frontend tests
cd frontend && npm run test

# Type checking
cd frontend && npm run check
```

Integration tests verify the full install flow in dry-run mode (no real commands executed) — step ordering, progress tracking, error handling, and rollback.

## Development

See [RULES.md](./RULES.md) for development conventions and [OVERVIEW.md](./OVERVIEW.md) for architecture details.

### Key Concepts

- **Live ISO vs Installed System** — Never confuse the two. The installer runs on the live ISO; it writes to `/mnt/` (the target system).
- **Helper binary** — All privileged ops go through `aestheticos-helper`. See `backend/cmd/helper/main.go` for the full command list.
- **InstallContext** — Shared state passed through all installation steps. See `backend/internal/install/context.go`.

## License

GPL-3.0 — see [LICENSE](./LICENSE) for details.

## Related Projects

- [AestheticOS](https://github.com/KornienkoDev/AestheticOS) — The live ISO and distro configuration
- [DankMaterialShell](https://github.com/maxshutyning/dms) — The desktop shell
- [DMS Greeter](https://github.com/maxshutyning/dms-greeter) — The greetd greeter
