# Aesthetic OS Installer - Development Roadmap

## Current Architecture

```
┌─────────────┐     ┌─────────────┐
│    GUI      │     │    CLI      │
│  (Wails)    │     │ (Installer) │
└──────┬──────┘     └──────┬──────┘
       │                   │
       └────────┬──────────┘
                │
     ┌──────────▼──────────┐
     │   Installer (Core)  │  ← Single source of truth
     └──────────┬──────────┘
                │
     ┌──────────▼──────────┐
     │    Steps (Logic)    │
     └─────────────────────┘
```

**Key Principles:**
- Single source of truth for installation logic
- If CLI works, GUI works
- Clean separation of concerns
- Testable core logic

---

## Phase 1: Core Stability (Current)

### ✅ Completed
- [x] Refactored to single `Installer` struct
- [x] CLI installer (`cmd/installer/`)
- [x] GUI uses same backend as CLI
- [x] Embedded packages file (no external dependencies)
- [x] Automatic mount cleanup before formatting
- [x] Fixed partition layout (128MB EFI + rest for root)
- [x] Clear pacman cache before pacstrap
- [x] Removed console installer (redundant)

### 🔧 In Progress
- [ ] Fix remaining pacstrap integrity errors
- [ ] Verify installation on real hardware

---

## Phase 2: Reliability & Safety

### High Priority
- [ ] **Implement rollback** — if installation fails, undo what was done
  - Unmount partitions
  - Clear partition table
  - Restore original state
- [ ] **Add structured logging** — essential for debugging
  - Log all commands executed
  - Log outputs and errors
  - Write to `/var/log/aesthetic-os-install.log`
- [ ] **Add disk validation** — before any destructive operations
  - Verify disk exists
  - Verify it's a disk (not a partition)
  - Check available space
  - Warn about data loss

### Medium Priority
- [ ] **Add retry logic** — for network operations (if using repos)
- [ ] **Improve error messages** — actionable, user-friendly
- [ ] **Add progress estimation** — time remaining for long operations

---

## Phase 3: Testing & Quality

### High Priority
- [ ] **Increase test coverage** — aim for 80%+ on `shared/` and `steps/`
  - Unit tests for validators
  - Unit tests for package parsing
  - Integration tests for step ordering
- [ ] **Add integration tests** — verify full installation flow
  - Test on virtual machine (QEMU/KVM)
  - Automated smoke tests
- [ ] **Add linting configuration**
  - `golangci-lint` for Go code
  - `eslint` for TypeScript/Svelte
  - Pre-commit hooks

### Medium Priority
- [ ] **Add benchmark tests** — measure installation time
- [ ] **Add fuzz testing** — for package file parsing

---

## Phase 4: Features & Flexibility

### High Priority
- [ ] **Add installation profiles** — save/load configurations
  - Default profile (current layout)
  - Custom layouts (LVM, encryption, etc.)
  - User-defined package lists
- [ ] **Add multi-disk support** — currently assumes single disk
  - Detect all available disks
  - Let user choose target disk
  - Support for RAID configurations
- [ ] **Add customization options**
  - Filesystem choice (ext4, btrfs, xfs, etc.)
  - Partition layout options
  - Bootloader options (systemd-boot, GRUB)

### Medium Priority
- [ ] **Add LUKS encryption support**
- [ ] **Add LVM support**
- [ ] **Add network configuration step**
- [ ] **Add hostname configuration**
- [ ] **Add timezone/locale configuration**

---

## Phase 5: User Experience

### High Priority
- [ ] **Improve GUI progress indication**
  - Show current step
  - Show time remaining
  - Show what's happening in detail
- [ ] **Add accessibility features**
  - Keyboard navigation
  - Screen reader support
  - High contrast mode
- [ ] **Add i18n/l10n** — multiple languages

### Medium Priority
- [ ] **Add dark/light theme support**
- [ ] **Add installation preview** — show what will happen before starting
- [ ] **Add installation history** — log of past installations

---

## Phase 6: Distribution & DevOps

### High Priority
- [ ] **Create proper build system**
  - Refine `build.sh`
  - Add version information
  - Add build timestamps
- [ ] **Add CI/CD pipeline**
  - Automated testing on PR
  - Automated building
  - Automated releasing
- [ ] **Create release process**
  - Semantic versioning
  - Changelog generation
  - Signed releases

### Medium Priority
- [ ] **Create AUR package** (for Arch Linux)
- [ ] **Create Flatpak/Snap package** (for GUI)
- [ ] **Add update mechanism** — check for new versions

---

## Phase 7: Documentation & Community

### High Priority
- [ ] **Create user documentation**
  - Installation guide
  - Troubleshooting guide
  - FAQ
- [ ] **Create developer documentation**
  - Architecture overview
  - API documentation
  - Contributing guidelines
- [ ] **Add code comments** — especially for complex logic

### Medium Priority
- [ ] **Create video tutorials**
- [ ] **Add examples directory**
- [ ] **Create community forums/Discord**

---

## Architecture Notes

### Current File Structure
```
aesthetic-os-installer/
├── backend/
│   ├── installer.go          # Core installation logic
│   ├── installation.go       # GUI integration
│   ├── shared/               # Shared utilities
│   │   ├── disks.go
│   │   ├── languages.go
│   │   └── validators.go
│   └── steps/                # Installation steps
│       ├── format_disk.go
│       ├── mount_filesystem.go
│       ├── install.go
│       ├── generate_fstab.go
│       ├── configure_bootloader.go
│       ├── setup_users.go
│       └── configure_system.go
├── cmd/
│   ├── installer/            # CLI entry point
│   └── root-helper/          # Privileged helper
├── frontend/                 # SvelteKit GUI
├── build.sh                  # Build script
└── ROADMAP.md                # This file
```

### Key Files
- `backend/installer.go` — **Single source of truth** for installation
- `backend/steps/*.go` — Individual installation steps
- `cmd/installer/main.go` — CLI entry point
- `cmd/root-helper/main.go` — Privileged operations

### Commands
```bash
# Build everything
./build.sh

# Run CLI installer
./build/bin/installer --install /dev/vda [--dry-run]

# Run GUI installer
./build/bin/aesthetic-os-installer

# Run tests
go test ./...

# Run linter
golangci-lint run
```

---

## Success Criteria

### Phase 2 (Reliability)
- [ ] Installation succeeds on 10+ different hardware configurations
- [ ] Rollback works correctly on failure
- [ ] All errors are logged and actionable

### Phase 3 (Testing)
- [ ] 80%+ test coverage on core logic
- [ ] Automated tests pass on every commit
- [ ] No regressions in 30 days

### Phase 4 (Features)
- [ ] Supports LUKS, LVM, RAID
- [ ] Multi-disk installations work
- [ ] Installation profiles save/load correctly

### Phase 5 (UX)
- [ ] Installation takes < 15 minutes on modern hardware
- [ ] GUI is accessible (WCAG 2.1 AA)
- [ ] Supports 5+ languages

### Phase 6 (Distribution)
- [ ] CI/CD pipeline runs on every PR
- [ ] Releases are signed and verified
- [ ] AUR package is maintained

---

## References

- [Install Process Documentation](install_process.md)
- [Wails Documentation](https://wails.io/)
- [SvelteKit Documentation](https://kit.svelte.dev/)
- [Arch Linux Installation Guide](https://wiki.archlinux.org/title/Installation_guide)

---

**Last Updated:** 2026-07-16
**Version:** 0.1.0 (Development)
