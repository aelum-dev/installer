# AestheticOS Installer — Development Roadmap

> A Wails-based (Go + Svelte 5) Arch Linux installer with GUI, CLI, and privileged helper binaries.

---

## Project Overview

| Layer | Technology |
|-------|-----------|
| GUI Framework | Wails v2 (Go ↔ JS bridge) |
| Frontend | Svelte 5 (runes), TypeScript, Vite, Vitest, Playwright |
| Backend | Go 1.25, Wails bindings |
| Packaging | Wails build (Linux/macOS/Windows NSIS) |

### Architecture

```
┌─────────────────────────────────────────────┐
│  Frontend (Svelte 5 + TS)                   │
│  ┌───────────────────────────────────────┐  │
│  │ 6-Page Wizard: Welcome → Finish       │  │
│  │ Components: Button, Dropdown, etc.    │  │
│  └───────────────────────────────────────┘  │
│              ↕ Wails JS bindings            │
├─────────────────────────────────────────────┤
│  Backend (Go)                               │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌───────────┐   │
│  │ GUI  │ │ Core │ │ CLI  │ │  Helper   │   │
│  │(Wails│ │(Inst │ │(Flags│ │ (sudo bin)│   │
│  │ Bind)│ │ Orch)│ │  App)│ │           │   │
│  └──────┘ └──────┘ └──────┘ └───────────┘   │
│  ┌──────────┐ ┌────────┐ ┌──────────────┐   │
│  │ Shared   │ │ Steps  │ │  Logger      │   │
│  │(Types,   │ │(Format,│ │(File/Console)│   │
│  │ Valid.)  │ │ Mount, │ │              │   │
│  │          │ │Install)│ │              │   │
│  └──────────┘ └────────┘ └──────────────┘   │
└─────────────────────────────────────────────┘
         ↕ sudo calls
┌─────────────────────────────────────────────┐
│  aestheticos-helper (privileged operations) │
│  parted, format, mount, pacstrap, bootctl,  │
│  useradd, passwd, efibootmgr, reboot, etc.  │
└─────────────────────────────────────────────┘
```

---

## Current State Assessment

### ✅ What's Working
- **6-page wizard flow** with navigation, validation, and transitions
- **Welcome page** — language/region selection dropdown
- **User Account page** — username/password with real-time validation
- **Disk Selection page** — auto-discovers disks via `lsblk`, shows HDD/SSD icons, size, serial
- **Confirmation page** — summary of all choices before install
- **Installation page** — live progress bar, step labels, elapsed timer, dry-run badge
- **Finish page** — success message with reboot button
- **CLI** — full install and individual step execution
- **Helper binary** — 15 privileged commands (parted, format, mount, pacstrap, bootctl, etc.)
- **Core installer** — orchestrates 7 installation steps with progress/error callbacks
- **Auto-cleanup** on failure (CleanupMounts)
- **Dry-run mode** — simulates installation without changes
- **Logging** — file + console loggers, installation logs saved to disk
- **Wails build system** — cross-platform (Linux/macOS/Windows)
- **Go tests** — all pass (`go test ./...` is clean)

### ⚠️ Known Issues
1. **`svelte-check` reports 23 errors** — wizard.e2e.ts imports from `$lib/store` (file was renamed to `$lib/state.svelte`); App.test.ts has outdated page count assumptions
2. **`simulateStep()` is dead code** — referenced but never called
3. **`CanGoNext()` has a TODO** — validation is disabled in `pages.go`
4. **`state.svelte.ts` missing type for `disk` field** — `diskSelectionData.disk` is assigned but not in the initial `$state()` shape
5. **No `.gitignore` for `frontend/dist`** — build artifacts may be tracked
6. **E2E tests use `@vitest/browser`** — requires Playwright browser installation to run

### ❌ What's Missing
- README.md / project documentation
- CI/CD pipeline
- Password strength indicator
- Installation log viewer UI
- i18n for the installer UI itself (language selector only sets system locale)
- Package customization (all installs use the same `packages.x86_64`)
- Keyboard navigation / accessibility
- Dark/light theme toggle
- Unit tests for frontend components (only App.test.ts exists, and it's outdated)
- Help page or tooltip documentation

---

## Roadmap

### Phase 1: Fix & Stabilize (Immediate)

#### 1.1 Fix Frontend Type Errors
- [ ] Update `App.test.ts` — fix page count references (5 → 6), update page name strings
- [ ] Run `npx svelte-check` until clean (0 errors, 0 warnings)
- [ ] Run `npx vitest run` to confirm all tests pass

#### 1.2 Fix Backend Dead Code
- [ ] Remove or repurpose `simulateStep()` in `core/installer.go`
- [ ] Add `disk: null` to `diskSelectionData` initial state in `state.svelte.ts`

#### 1.3 Re-enable `CanGoNext()` Validation
- [ ] Implement proper validation logic in `gui/pages.go` `CanGoNext()` method
- [ ] Ensure each page validates its required data before allowing navigation

#### 1.4 Add `.gitignore` for Frontend Build
- [ ] Add `frontend/dist/` to `.gitignore`
- [ ] Add `frontend/node_modules/` to `.gitignore` if not present

---

### Phase 2: Core Installer Improvements

#### 2.1 Robust Partition Discovery
- [ ] Use `findmnt` or `/proc/mounts` to auto-discover existing ESP and root mountpoints
- [ ] Detect and warn about existing installations on the target disk
- [ ] Support NVMe devices (`/dev/nvme0n1p1`) and virtual disks (`/dev/vda`, `/dev/xvda`) consistently
- [ ] Add partition scheme selection: GPT only vs. support BIOS/MBR (grub)

#### 2.2 Package Customization
- [ ] Add a "Packages" page or expand Confirmation page with package selection
- [ ] Support package groups:
  - `base` (always included)
  - `base-devel`
  - Desktop environments: KDE, GNOME, XFCE, i3, sway, etc.
  - Drivers: nvidia, broadcom, bluetooth, wifi
  - Utilities: vim, git, neofetch, etc.
- [ ] Save custom package list and load on next run
- [ ] Allow removing any package from the default list

#### 2.3 Installation Resilience
- [ ] Add checkpoint/rollback: if a step fails, unmount and clean up before reporting
- [ ] Add step retry capability (especially for pacstrap network failures)
- [ ] Add network availability check before installation starts
- [ ] Support resume from checkpoint (skip already-completed steps)

#### 2.4 Bootloader Improvements
- [ ] Add dual-boot detection: scan existing EFI entries before installing
- [ ] Auto-add existing OS entries to systemd-boot config
- [ ] Support GRUB as alternative bootloader option
- [ ] Allow custom kernel parameters

---

### Phase 3: Frontend Enhancements

#### 3.1 Installation Log Viewer
- [ ] Add a "View Log" button on Installation and Finish pages
- [ ] Display full installation log in a scrollable modal or expandable panel
- [ ] Color-code log lines: info (blue), success (green), error (red)
- [ ] Allow copying log to clipboard or downloading as `.txt`

#### 3.2 Password Strength Indicator
- [ ] Show real-time password strength meter on User Account page
- [ ] Categories: weak / fair / strong / very strong
- [ ] Suggest improvements (length, special chars, etc.)

#### 3.3 Disk Selection Improvements
- [ ] Block disks that are currently mounted as `/` or `/boot` with a lock icon
- [ ] Show a warning when selecting the currently running system's disk
- [ ] Add a "Delete all partitions" checkbox for advanced users
- [ ] Show partition layout preview (ASCII art or visual bar) after format step

#### 3.4 Theme & Accessibility
- [ ] Add dark/light theme toggle (system preference default)
- [ ] Add keyboard navigation: Tab order, Enter/Space for buttons, Escape for dialogs
- [ ] Add ARIA labels to all interactive elements
- [ ] Ensure minimum contrast ratio 4.5:1 (WCAG AA)
- [ ] Add focus indicators for all interactive elements

#### 3.5 Frontend Testing
- [ ] Add unit tests for each page component (Welcome, UserAccount, DiskSelection, Confirmation, Installation, Finish)
- [ ] Add tests for navigation logic (`$lib/navigation.ts`)
- [ ] Add tests for validation logic (`$lib/validate.ts`)
- [ ] Add snapshot tests for key components

---

### Phase 4: CLI & Automation

#### 4.1 Interactive CLI Mode
- [ ] Add `--interactive` flag for guided terminal-based installation (ncurses/TUI)
- [ ] Support all options via flags: `--username`, `--password`, `--hostname`, `--language`, `--packages`
- [ ] Add `--config <file>` to load installation profile from JSON/YAML

#### 4.2 Unattended Installation
- [ ] Add `--unattended` mode for scripted/kickstart installations
- [ ] Support answering all prompts via environment variables or config file
- [ ] Add `--verbose` for detailed output in logs

#### 4.3 Helper Binary Improvements
- [ ] Add `helper check` command for pre-flight checks (root, network, disk space)
- [ ] Add `helper list-distros` to enumerate existing OS installations
- [ ] Add `helper verify` to validate a mounted system before reboot

---

### Phase 5: Cross-Platform & Distribution

#### 5.1 macOS Support
- [ ] Build and test on macOS (Wails supports it)
- [ ] Note: Actual installation won't work on macOS, but the UI can be tested
- [ ] Add platform detection and graceful degradation

#### 5.2 Windows Support
- [ ] Build with Wails Windows target
- [ ] Use NSIS installer (already configured in `build/windows/`)
- [ ] Test Windows-specific disk enumeration

#### 5.3 Live ISO Integration
- [ ] Package installer into a Live ISO for distribution
- [ ] Auto-detect and use helper from `/shared/` on live system
- [ ] Add auto-start on boot from live ISO

#### 5.4 CI/CD Pipeline
- [ ] Set up GitHub Actions (or similar) for:
  - [ ] Lint: `gofmt`, `golangci-lint`, `svelte-check`, `eslint`
  - [ ] Test: `go test ./...`, `vitest run`
  - [ ] Build: `wails build` for Linux, macOS, Windows
  - [ ] Release: auto-create GitHub release with binaries attached
- [ ] Add code coverage reporting

---

### Phase 6: Advanced Features

#### 6.1 Custom Partitioning
- [ ] Allow manual partition layout (drag-and-drop or form-based)
- [ ] Support LVM, Btrfs subvolumes, ZFS
- [ ] Preserve existing partitions option
- [ ] RAID configuration

#### 6.2 System Configuration
- [ ] Network configuration: WiFi, Ethernet, static IP, hostname
- [ ] Timezone and locale selection (expand beyond language)
- [ ] Bootloader customization (timeout, default entry)
- [ ] Swap partition configuration (size, file vs. partition)

#### 6.3 Post-Install Scripts
- [ ] Allow user to define post-install commands
- [ ] Template scripts for common setups (dotfiles, AUR helper, etc.)
- [ ] Enable/disable services at first boot

#### 6.4 Security
- [ ] Encrypt home directory (LUKS) option
- [ ] Full disk encryption (LUKS + LVM) option
- [ ] Secure password generation with options
- [ ] SSH key import for user account

#### 6.5 Telemetry & Feedback (Opt-in)
- [ ] Anonymous hardware report (disk count, RAM, CPU type)
- [ ] Installation success/failure tracking
- [ ] Help improve installer with crash reports

---

## Priority Matrix

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| 🔴 P0 | Fix svelte-check errors | Low | Blocks clean build |
| 🔴 P0 | Re-enable CanGoNext validation | Low | Prevents invalid state |
| 🟠 P1 | Package customization | Medium | Core feature gap |
| 🟠 P1 | Installation log viewer | Low | User experience |
| 🟠 P1 | Robust partition discovery | Medium | Reliability |
| 🟡 P2 | Theme toggle + accessibility | Medium | Inclusivity |
| 🟡 P2 | CI/CD pipeline | Medium | Dev workflow |
| 🟡 P2 | E2E test fixes | Low | Test coverage |
| 🟢 P3 | Custom partitioning | High | Advanced users |
| 🟢 P3 | Full disk encryption | High | Security |
| 🟢 P3 | Live ISO packaging | High | Distribution |

---

## File Reference

```
.
├── main.go                      # Wails app entry point
├── wails.json                   # Wails configuration
├── build.sh                     # Build script
├── run.sh                       # Dev run script
├── go.mod / go.sum              # Go modules
│
├── backend/
│   ├── cli/main.go              # CLI binary entry point
│   ├── core/
│   │   ├── installer.go         # Installation orchestrator
│   │   └── installer_test.go    # Core tests
│   ├── gui/
│   │   ├── api.go               # Wails API bindings
│   │   ├── app.go               # App state management
│   │   ├── installation.go      # StartInstallation + events
│   │   ├── pages.go             # Page navigation
│   │   ├── validation.go        # Input validation
│   │   └── root_linux.go        # Linux-specific root checks
│   ├── helper/main.go           # Privileged helper binary
│   ├── shared/
│   │   ├── types.go             # Shared data types
│   │   ├── validators.go        # Backend validation
│   │   ├── languages.go         # Language list
│   │   ├── regions.go           # Region data
│   │   ├── disks.go             # Disk enumeration
│   │   ├── mirrorlist.go        # Mirror list management
│   │   └── mirrorlist_gen.go    # Mirror list generator
│   └── steps/
│       ├── format_disk.go       # Partition creation
│       ├── mount_filesystem.go  # Mount partitions
│       ├── install.go           # pacstrap
│       ├── generate_fstab.go    # fstab generation
│       ├── configure_bootloader.go # systemd-boot
│       ├── setup_users.go       # User creation
│       ├── configure_system.go  # System config
│       ├── cleanup.go           # Unmount & cleanup
│       ├── context.go           # InstallContext
│       ├── validation.go        # Step validation
│       └── logger.go            # Logging
│
├── frontend/
│   ├── src/
│   │   ├── App.svelte           # Main app component
│   │   ├── main.ts              # Entry point
│   │   ├── style.css            # Global styles
│   │   ├── lib/
│   │   │   ├── types.ts         # Type definitions
│   │   │   ├── state.svelte.ts  # Reactive state
│   │   │   ├── navigation.ts    # Page navigation logic
│   │   │   └── validate.ts      # Frontend validation
│   │   ├── pages/
│   │   │   ├── WelcomePage.svelte
│   │   │   ├── UserAccountPage.svelte
│   │   │   ├── DiskSelectionPage.svelte
│   │   │   ├── ConfirmationPage.svelte
│   │   │   ├── InstallationPage.svelte
│   │   │   └── FinishPage.svelte
│   │   ├── components/
│   │   │   ├── AppHeader.svelte
│   │   │   ├── AppFooter.svelte
│   │   │   ├── Button.svelte
│   │   │   ├── ConfirmDialog.svelte
│   │   │   ├── Dropdown.svelte
│   │   │   ├── ErrorBanner.svelte
│   │   │   └── PageLayout.svelte
│   │   └── test/
│   │       ├── setup.ts         # Test mocks
│   │       ├── wizard.e2e.ts    # E2E tests
│   │       └── App.test.ts      # Unit tests
│   └── wailsjs/                 # Auto-generated Wails bindings
│
├── build/
│   ├── bin/                     # Compiled binaries
│   ├── darwin/                  # macOS build config
│   ├── windows/                 # Windows installer config
│   └── appicon.png
│
├── mirrorlist/                  # Mirror lists (root level)
├── backend/steps/packages.x86_64  # Default package list
├── RULES.md                     # Project rules
└── ROADMAP.md                   # This file
```

---

## Notes

- **Building**: Run `./build.sh` — this builds frontend, runs `wails build`, and builds helper + CLI binaries
- **Testing**: Run `go test ./...` for Go tests; `cd frontend && npx vitest run` for frontend tests
- **Dry run**: Launch with `--dry-run` flag to simulate installation without changes
- **Helper**: `aestheticos-helper` requires root/sudo; main installer calls it via `sudo`
- **Target**: This installer is designed to run from a Live USB/ISO environment targeting Arch Linux installation
