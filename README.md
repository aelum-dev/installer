# Aesthetic OS Installer

A modern, beautiful Arch Linux installer built with [Wails](https://wails.io/) (Go + Svelte 5) featuring a step-by-step wizard interface. Designed for simplicity and visual appeal using the [Catppuccin Latte](https://catppuccin.com/palette/latte) color palette.

<img width="1125" height="750" alt="screenshot_2026-07-16_11-25-01" src="https://github.com/user-attachments/assets/0b5cc06e-aadd-4510-aa49-a9ea5e1319d2" />

## Features

- **Multi-step wizard** — 6-page guided installation flow
- **Language selection** — Support for 8 languages (English, German, French, Spanish, Japanese, Chinese, Korean, Russian)
- **User account setup** — Create your username and password with validation
- **Disk selection** — Auto-detects available block devices via `lsblk`, displays model, size, and type (HDD/SSD)
- **Installation confirmation** — Review details before proceeding
- **Live progress tracking** — Real-time progress updates during installation
- **Dry-run mode** — Simulate the entire installation without making changes (great for testing)
- **Cross-platform builds** — macOS, Windows, and Linux support via Wails

## Installation Wizard Pages

| # | Page | Description |
|---|------|-------------|
| 1 | **Welcome** | Language selection to begin |
| 2 | **User Account** | Create username and password |
| 3 | **Disk Selection** | Choose target disk from auto-detected devices |
| 4 | **Confirmation** | Review all installation settings |
| 5 | **Installation** | Watch progress as the system is installed |
| 6 | **Finish** | Completion screen with reboot option |

## Installation Steps (Backend)

The installer performs the following steps on the target disk:

1. **Format disk** — Creates ext4 filesystem
2. **Mount filesystem** — Mounts target at `/mnt`
3. **Install base system** — Uses `pacstrap` to install `base`, `linux`, `linux-firmware`, `base-devel`
4. **Configure bootloader** — Installs and configures GRUB
5. **Set up users** — Creates user accounts
6. **Install packages** — Additional package installation
7. **Configure system** — Hostname, timezone, locale, etc.

## Tech Stack

| Layer | Technology |
|-------|------------|
| **Backend** | Go 1.25 + Wails v2 |
| **Frontend** | Svelte 5, TypeScript, Vite |
| **UI Theme** | Catppuccin Latte color palette |
| **Font** | Nunito (primary), JetBrains Mono (code) |
| **System Integration** | D-Bus (Linux notifications), native shell commands |

## Project Structure

```
aesthetic-os-installer/
├── backend/                 # Go backend (Wails bindings)
│   ├── app.go              # App lifecycle, reboot logic
│   ├── types.go            # Shared types & PageID enum
│   ├── navigation.go       # Page navigation & validation
│   ├── installation.go     # Installation steps & progress
│   ├── disks.go            # Disk detection via lsblk
│   ├── languages.go        # Language selection
│   ├── validators.go       # Form validation
│   ├── root_linux.go       # Root privilege check (Linux)
│   └── root_other.go       # Root privilege check (other OS)
├── frontend/               # Svelte 5 + TypeScript frontend
│   ├── src/
│   │   ├── components/     # Reusable UI components
│   │   │   ├── AppHeader.svelte
│   │   │   ├── AppFooter.svelte
│   │   │   ├── Button.svelte
│   │   │   ├── ConfirmDialog.svelte
│   │   │   ├── Dropdown.svelte
│   │   │   ├── ErrorBanner.svelte
│   │   │   └── PageLayout.svelte
│   │   ├── pages/          # Wizard page components
│   │   │   ├── WelcomePage.svelte
│   │   │   ├── UserAccountPage.svelte
│   │   │   ├── DiskSelectionPage.svelte
│   │   │   ├── ConfirmationPage.svelte
│   │   │   ├── InstallationPage.svelte
│   │   │   └── FinishPage.svelte
│   │   ├── lib/            # Utilities & state
│   │   │   ├── types.ts    # TypeScript types & helpers
│   │   │   ├── navigation.ts  # Navigation logic
│   │   │   └── store.ts    # Svelte state stores
│   │   ├── App.svelte      # Main app component
│   │   └── style.css       # Catppuccin theme variables
│   └── package.json
├── build/                  # Platform-specific build assets
├── main.go                 # Application entrypoint
├── wails.json              # Wails configuration
├── go.mod                  # Go dependencies
└── run.sh                  # Development launcher script
```

## Getting Started

### Prerequisites

- [Go](https://go.dev/) 1.25+
- [Node.js](https://nodejs.org/) 18+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- npm or pnpm

### Development

```bash
# Clone the repository
git clone <repository-url>
cd aesthetic-os-installer

# Run in development mode (with hot reload)
./run.sh

# Run with dry-run mode (simulates installation)
./run.sh --dry-run

# Run with CSS debug overlay
./run.sh --debug-css
```

### Building

```bash
wails build
```

## Architecture

### State Management

State is managed through a combination of:
- **Backend struct** (`backend/types.go`) — Holds form data, current page, language selection
- **Svelte 5 runes** (`$state`, `$effect`, `$derived`) — Reactive frontend state
- **Wails events** — Installation progress events (`installation:progress`, `installation:complete`, `installation:error`)

### Navigation

The wizard uses a page ID system (`PageID` enum) with built-in forward/backward navigation. Validation is performed before advancing to the next step. Pages requiring validation: User Account and Disk Selection.

### Dry-Run Mode

Set via `--dry-run` flag or `AESTHETIC_DRY_RUN=1` environment variable. In dry-run mode:
- All installation steps are simulated with randomized delays
- No system modifications are made
- No root privileges are required
- Progress events include `[DRY RUN]` prefix

## License

This project is licensed under the terms of the license included in the repository.

## Author

**KornienkoDev** — [ik.workmail39@gmail.com](mailto:tsunadegodaime3@gmail.com)
