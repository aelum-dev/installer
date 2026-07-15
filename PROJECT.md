# Project Analysis — aesthetic-os-installer

## Project Overview

A **desktop application** for installing an Arch-based Linux distribution, built with **Wails v2** (Go + web frontend). The installer follows a wizard-style, 6-page flow with a light theme and functional design.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Runtime** | Wails v2.13.0 (Go + WebView2/WebKit) |
| **Backend** | Go 1.25 — handles system operations (disk detection, installation, reboot) |
| **Frontend** | Svelte 5 (runes mode) + TypeScript + Vite 6 |
| **Testing** | Vitest (unit tests) |
| **Window** | Fixed 900×600 (locked via Min/Max width/height) |

---

## Architecture

```
┌─────────────────────────────────────────────┐
│                   Wails App                  │
├──────────────────┬──────────────────────────┤
│   Go Backend     │     Svelte Frontend       │
│  (IPC via Wails) │     (Vite dev server)     │
├──────────────────┴──────────────────────────┤
│  Embeds frontend/dist as static assets       │
└─────────────────────────────────────────────┘
```

### Backend (`backend/`) — 8 files

| File | Purpose |
|------|---------|
| `app.go` | App struct, startup, start-page logic, reboot/close |
| `types.go` | Shared types: `PageID`, `UserAccountData`, `DiskInfo`, `Language` |
| `navigation.go` | Page navigation: `CanGoNext`, `CanGoBack`, `SetCurrentPage` |
| `validators.go` | Server-side validation for user account & disk selection |
| `disks.go` | Disk detection via `lsblk` JSON parsing |
| `installation.go` | 7-step Arch installation pipeline with progress events |
| `root_linux.go` / `root_other.go` | Platform-specific root privilege checks |
| `languages.go` | Language selection support |

### Frontend (`frontend/src/`) — 12 Svelte components

**Pages (6):**
1. `WelcomePage` — Landing/intro
2. `UserAccountPage` — Username + password form
3. `DiskSelectionPage` — Disk picker with dropdown
4. `ConfirmationPage` — Review before install
5. `InstallationPage` — Progress bar with step labels
6. `FinishPage` — Reboot button

**Components (6):**
- `PageLayout` — Shared layout wrapper
- `PageTransition` — Animated page transitions (200ms leave/enter)
- `AppHeader` — Header with page title
- `AppFooter` — Back/Next navigation buttons
- `Button` — Reusable button (primary/secondary/ghost variants)
- `Dropdown` — Custom select component with keyboard nav

**Lib:**
- `store.ts` — Writable stores for form state
- `types.ts` — Shared TypeScript types
- `validate.ts` — Frontend validation utilities

---

## Installer Wizard Flow

```
0. Welcome → 1. User Account → 2. Disk Selection → 3. Confirmation → 4. Installation → 5. Finish
```

Each page validates server-side via Go IPC before proceeding. The installation step (page 4) runs 7 async steps with progress events emitted to the frontend.

---

## Key Design Decisions (from MEMORY.md)

- **Svelte 5 runes** — `$props()`, `$state()`, `$derived()`, `$effect()` (no legacy `export let`)
- **Event-based navigation** — Pages dispatch `go-next` custom events to parent
- **Animation via `{#key}`** — Page transitions use key increment + CSS transitions
- **Design tokens** — CSS custom properties in `style.css` for consistency
- **Path aliases** — `$components/`, `$pages/`, `$lib/`, `@wails/`
- **Dry-run mode** — Simulates installation without system changes

---

## Pending Refactoring (from REFACTORING_PLAN.md)

| Phase | Task | Status |
|-------|------|--------|
| Phase 1 | Design tokens + PageLayout | ✅ Complete |
| 2.1 | Replace `writable` store → `$state` | ⏳ TODO |
| 2.2 | Extract navigation → `useWizard.ts` | ⏳ TODO |
| 2.3 | Replace events → callback props | ⏳ TODO |
| 3.1 | Extract `FormField` component | ⏳ TODO |
| 3.2 | Flesh out stub pages (FinishPage) | ⏳ TODO |
| 3.3 | Button `onclick` → `onClick` | ⏳ TODO |

---

## Project Stats

- **Total Svelte code:** ~1,718 lines
- **Backend:** ~7 Go files with disk detection, validation, installation pipeline
- **Git commits:** 10+ tracked changes
- **Build artifact:** `aesthetic-os-installer` (5.6 MB binary)
