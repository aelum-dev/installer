# Data Flow Plan

## Overview

This document describes how data flows through the aesthetic-os-installer, where it's stored, and the relationships between frontend and backend data layers.

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Wails Application                        │
├──────────────────────────────────┬──────────────────────────────┤
│         Go Backend (App)         │       Svelte Frontend        │
│                                  │                              │
│  ┌────────────────────────────┐  │  ┌────────────────────────┐  │
│  │ App struct (in-memory)     │  │  │ Writable Stores        │  │
│  │ - currentPage              │  │  │ - userAccountData      │  │
│  │ - startPage                │  │  │ - diskSelectionData    │  │
│  │ - dryRun                   │  │  └────────────────────────┘  │
│  │ - userAccount              │  │                              │
│  │ - diskSelection            │  │  ┌────────────────────────┐  │
│  │ - selectedLanguage         │  │  │ Local Component State  │  │
│  └────────────────────────────┘  │  │ - disks (DiskInfo[])   │  │
│                                  │  │ - loading, error       │  │
│  ┌────────────────────────────┐  │  │ - installation state   │  │
│  │ System Calls               │  │  │ - progress, steps      │  │
│  │ - lsblk (disk detection)   │  │  └────────────────────────┘  │
│  │ - exec.Command (install)   │  │                              │
│  │ - reboot                   │  │  ┌────────────────────────┐  │
│  └────────────────────────────┘  │  │ Wails IPC Bridge       │  │
│                                  │  │ - @wails/go/app/App    │  │
│  ┌────────────────────────────┐  │  │ - @wails/runtime       │  │
│  │ Events Bus                 │  │  └────────────────────────┘  │
│  │ - installation:progress    │  │                              │
│  │ - installation:complete    │  │  ┌────────────────────────┐  │
│  │ - installation:error       │  │  │ Custom Events          │  │
│  └────────────────────────────┘  │  │ - go-next              │  │
│                                  │  │ - page-leave           │  │
│                                  │  └────────────────────────┘  │
└──────────────────────────────────┴──────────────────────────────┘
```

---

## Data Entities

### 1. User Account Data

**Type:** `{ username: string; password: string; confirmPassword: string }`

| Attribute | Location | Lifecycle |
|-----------|----------|-----------|
| Source of truth | Frontend: `userAccountData` (writable store) | Page 1 (`UserAccountPage.svelte`) |
| Local form state | `UserAccountPage` component | `let username = $state('')` etc. |
| Validation (frontend) | `validate.ts` — reactive on input | Runs on blur (touched fields) |
| Validation (backend) | `backend/validators.go` — `ValidateUserAccount()` | Runs on "next" navigation |
| Persisted by backend | `App.userAccount` struct field | After backend validation passes |

**Flow:**
```
User types → $state variables → store.set() on every change
                            → frontend validate() on blur
                            → backend ValidateUserAccount() on "next"
                            → App.userAccount (Go struct) if valid
```

**Security Note:** Passwords stored in plaintext in memory on both sides. No hashing.

---

### 2. Disk Selection Data

**Type:** `{ diskId: string; disk: DiskInfo | null }`

| Attribute | Location | Lifecycle |
|-----------|----------|-----------|
| Source of truth | Frontend: `diskSelectionData` (writable store) | Page 2 (`DiskSelectionPage.svelte`) |
| Raw disk list | `DiskSelectionPage` local state | Fetched from backend on mount |
| Selected disk ID | `DiskSelectionPage.selectedDiskId` | Set on card click |
| Validation (backend) | `backend/validators.go` — `ValidateDiskSelection()` | Runs on "next" navigation |
| Persisted by backend | `App.diskSelection` struct field | After backend validation passes |

**Flow:**
```
Backend: GetDisks() → lsblk JSON → []DiskInfo → Frontend
                                                    → user selects card
                                                    → store.set({ diskId, disk })
                                                    → backend ValidateDiskSelection() on "next"
                                                    → App.diskSelection (Go struct) if valid
```

**Discovery Command:**
```bash
lsblk -b -d -o NAME,TYPE,SIZE,MODEL,VENDOR,ROTA,RO,FSTYPE,MOUNTPOINT,SERIAL,TRAN -J
```

---

### 3. Language Data

**Type:** `{ code: string; name: string; locale: string }[]`

| Attribute | Location | Lifecycle |
|-----------|----------|-----------|
| Source of truth | Backend: hardcoded list | `backend/languages.go` |
| Selected language | `App.selectedLanguage` (Go struct) | Set via `SetSelectedLanguage()` |
| Displayed on | `ConfirmationPage` | Read via `GetSelectedLanguage()` |

**Note:** Language selection UI is not yet implemented. The list is available but no dropdown exists in the frontend.

---

### 4. Installation Progress Data

**Type:** `InstallationProgress { label: string; percent: number; phase: string }`

| Attribute | Location | Lifecycle |
|-----------|----------|-----------|
| Emitted by | Backend: `runInstallation()` goroutine | Wails Events API |
| Received by | `InstallationPage` via `EventsOn()` | Real-time during install |
| Displayed state | `InstallationPage` local `$state` | `currentStep`, `currentStepLabel`, `progress`, `isRunning`, `isComplete` |

**Events:**
| Event Name | Payload | Trigger |
|------------|---------|---------|
| `installation:progress` | `{ label, percent, phase: "running"\|"done" }` | Each step starts/completes |
| `installation:complete` | `{ label, percent: 100, phase: "done" }` | All 7 steps done |
| `installation:error` | `{ label, percent: 0, phase: "error" }`, message string | Step fails |

**Flow:**
```
User reaches page 4 → init() calls StartInstallation()
                    → Backend: goroutine runs 7 steps
                    → Each step: EventsEmit("installation:progress", ...)
                    → Frontend: EventsOn handler updates $state
                    → Final step: EventsEmit("installation:complete")
                    → Frontend: auto-dispatches "go-next" to FinishPage
```

**7 Installation Steps:**
1. Format disk (`mkfs.ext4`)
2. Mount filesystem (`mount /dev/X /mnt`)
3. Install base system (`pacstrap /mnt base linux linux-firmware base-devel`)
4. Configure bootloader (`grub-install`, `grub-mkconfig`)
5. Set up users (`stepSetupUsers` — stub)
6. Install packages (`stepInstallPackages` — stub)
7. Configure system (`stepConfigureSystem` — stub)

---

### 5. Navigation / Page State

| Attribute | Location | Type |
|-----------|----------|------|
| `currentPage` (0-5) | `App.svelte` ($state) + `App.currentPage` (Go) | Local + synced |
| `animKey` | `App.svelte` ($state) | Frontend-only (animation trigger) |
| `animDirection` | `App.svelte` ($state) | Frontend-only |
| `animLeaving` / `animEntering` | `App.svelte` ($state) | Frontend-only |
| `startPage` | `App.startPage` (Go) | Go-only (debug/resume feature) |

**Flow:**
```
Frontend currentPage changes → SetCurrentPage(currentPage) → Go App.currentPage
Go App.currentPage ← GetCurrentPage() ← App.svelte (on init, to resume)
```

---

## Data Flow Diagram

```
                    ┌─────────────────────────────────────────────────────┐
                    │                    WIRING & INIT                     │
                    │                                                      │
                    │  main.go: NewApp(dryRun) ─→ App struct (Go)         │
                    │       wails.Run()  ─→ embeds frontend/dist          │
                    └─────────────────────────────────────────────────────┘
                                      │
                                      ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │                          PAGE 0: Welcome                             │
    │                                                                      │
    │  User clicks "Get Started" ──→ dispatches CustomEvent('go-next')    │
    │                                      │                               │
    │                              App.svelte listener                     │
    │                                      │                               │
    │                              validateNext() → CanGoNext() → Go       │
    │                                      │                               │
    │                              animateTransition() → currentPage++     │
    └──────────────────────────────────────┼───────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │                       PAGE 1: User Account                           │
    │                                                                      │
    │  User types ──→ $state (username, password, confirmPassword)        │
    │       │                                                                      │
    │       ├─→ $effect syncs ──→ userAccountData.store.set()                    │
    │       │                                                                      │
    │       └─→ on blur ──→ validateUserAccount() ──→ errors (local)             │
    │                                                                              │
    │  User clicks "Next" ──→ dispatches 'go-next'                               │
    │       │                                                                      │
    │       ▼                                                                      │
    │  App.svelte: validateNext() ──→ Go ValidateUserAccount() ──→               │
    │       App.userAccount = {username, password, confirmPassword}                │
    └──────────────────────────────────────┼───────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │                      PAGE 2: Disk Selection                          │
    │                                                                      │
    │  On mount: GetDisks() ──→ Go: lsblk -b -d -o ... -J                 │
    │       │                                                              │
    │       ▼                                                              │
    │  Disks[] ──→ disks local state ($state)                             │
    │                                                                      │
    │  User clicks disk card ──→ selectDisk(disk)                         │
    │       │                                                              │
    │       ├─→ selectedDiskId = disk.path                                │
    │       └─→ diskSelectionData.store.set({ diskId, disk })             │
    │                                                                      │
    │  User clicks "Next" ──→ dispatches 'go-next'                        │
    │       │                                                              │
    │       ▼                                                              │
    │  App.svelte: validateNext() ──→ Go ValidateDiskSelection() ──→      │
    │       App.diskSelection = {diskId}                                  │
    └──────────────────────────────────────┼───────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │                      PAGE 3: Confirmation                            │
    │                                                                      │
    │  Subscribes to stores:                                              │
    │  - userAccountData.subscribe ──→ userData (display)                  │
    │  - diskSelectionData.subscribe ──→ diskData (display)                │
    │  - GetLanguages() ──→ languages list ──→ selectedLanguageName       │
    │                                                                      │
    │  User clicks "Confirm" ──→ dispatches 'go-next'                     │
    │       (no backend validation — pure review page)                     │
    └──────────────────────────────────────┼───────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │                     PAGE 4: Installation                             │
    │                                                                      │
    │  On mount: init() ──→ GetDryRun() ──→ isDryRun                     │
    │       │                                                              │
    │       └─→ StartInstallation() ──→ Go: goroutine                     │
    │               │                                                       │
    │               ▼                                                       │
    │  Go: runInstallation() ──→ 7 steps in sequence:                      │
    │       │                                                               │
    │       ├─ Step 1: mkfs.ext4 ──→ EventsEmit("installation:progress")   │
    │       ├─ Step 2: mount ──→ EventsEmit("installation:progress")       │
    │       ├─ Step 3: pacstrap ──→ EventsEmit("installation:progress")    │
    │       ├─ Step 4: grub-install ──→ EventsEmit("installation:progress")│
    │       ├─ Step 5: setupUsers ──→ EventsEmit("installation:progress")  │
    │       ├─ Step 6: installPackages ──→ EventsEmit("installation:progress")│
    │       └─ Step 7: configureSystem ──→ EventsEmit("installation:progress")│
    │               │                                                       │
    │               ├─ on error ──→ EventsEmit("installation:error")       │
    │               └─ on complete ──→ EventsEmit("installation:complete") │
    │               │                                                       │
    │       Frontend: EventsOn handlers ──→ updates $state                 │
    │               │                                                       │
    │               └─→ isComplete = true ──→ dispatches 'go-next'         │
    └──────────────────────────────────────┼───────────────────────────────┘
                                           │
                                           ▼
    ┌─────────────────────────────────────────────────────────────────────┐
    │                        PAGE 5: Finish                                │
    │                                                                      │
    │  User clicks "Reboot & Close" ──→ RebootAndClose() ──→ Go:         │
    │       │                                                              │
    │       ├─ dryRun: Quit() only                                       │
    │       └─ real: exec.Command("reboot") + Quit()                     │
    └─────────────────────────────────────────────────────────────────────┘
```

---

## Store Architecture

### Current (Pre-Refactoring)

```typescript
// frontend/src/lib/store.ts
import { writable } from 'svelte/store';

export const userAccountData = writable<UserAccountData>({...});
export const diskSelectionData = writable<DiskSelectionData>({...});
```

**Access pattern:**
```typescript
import { get } from 'svelte/store';
const data = get(userAccountData); // one-time read

// Or subscribe:
$effect(() => {
  const unsub = userAccountData.subscribe(value => { ... });
  return unsub;
});
```

### Target (After Phase 2.1)

```typescript
// frontend/src/lib/store.ts
export let userAccountData = $state({...});
export let diskSelectionData = $state({...});
```

**Access pattern:**
```typescript
import { userAccountData } from '$lib/store';
const data = userAccountData; // direct access, no get()
```

---

## IPC Communication Map

### Frontend → Backend (Wails Bindings)

| Method | Called From | Purpose |
|--------|-----------|---------|
| `GetDisks()` | `DiskSelectionPage` | Fetch available block devices |
| `ValidateUserAccount(data)` | `App.svelte` (on next) | Server-side user validation |
| `ValidateDiskSelection(data)` | `App.svelte` (on next) | Server-side disk validation |
| `CanGoNext()` | `App.svelte` | Check if forward navigation allowed |
| `CanGoBack()` | `App.svelte` | Check if backward navigation allowed |
| `SetCurrentPage(n)` | `App.svelte` | Sync page index with backend |
| `GetCurrentPage()` | (unused) | Get current page from backend |
| `StartInstallation()` | `InstallationPage` | Begin async installation |
| `GetDryRun()` | `InstallationPage` | Check if running in dry-run mode |
| `RebootAndClose()` | `App.svelte` (finish) | Reboot system and quit |
| `SetStartPage(n)` | `main.go` | Configure debug start page |
| `GetStartPage()` | `App.svelte` | Read configured start page |
| `ClearStartPage()` | `App.svelte` | Clear after use |
| `GetLanguages()` | `ConfirmationPage` | Fetch available languages |
| `GetSelectedLanguage()` | `ConfirmationPage` | Get currently selected language |
| `SetSelectedLanguage(code)` | (unused) | Set language + apply locale |

### Backend → Frontend (Events)

| Event | Payload | Emitted By |
|-------|---------|------------|
| `installation:progress` | `{ label, percent, phase }` | `runInstallation()` per step |
| `installation:complete` | `{ label, percent: 100, phase }` | `runInstallation()` final |
| `installation:error` | `{ label, percent: 0, phase }`, message | `runInstallation()` on failure |

### Frontend → Frontend (Custom Events)

| Event | Dispatched From | Handled By | Purpose |
|-------|----------------|------------|---------|
| `go-next` | Any page | `App.svelte` | Trigger forward navigation |
| `page-leave` | `App.svelte` | Any page | Signal before leaving page (for logging) |

---

## Data Lifecycle Summary

```
User Account Data:
  UserInput → $state → store.set() → frontend validate() → backend validate() → App.userAccount

Disk Selection Data:
  Go:lsblk → JSON parse → Frontend disks[] → user click → store.set() → backend validate() → App.diskSelection

Installation Data:
  User clicks next → Go:StartInstallation() → goroutine → EventsEmit → Frontend EventsOn → display state

Navigation Data:
  App.svelte $state → SetCurrentPage() → Go App.currentPage ← GetStartPage() on init

Language Data:
  Go: hardcoded list → GetLanguages() → Frontend display → (SetSelectedLanguage not wired yet)
```

---

## Known Gaps & TODOs

1. **Language selection UI** — Languages are available via backend but no frontend component to select one
2. **Password hashing** — Passwords stored in plaintext in both frontend stores and Go struct
3. **Installation state persistence** — No way to resume interrupted installation
4. **`stepSetupUsers`, `stepInstallPackages`, `stepConfigureSystem`** — All stubs, need real implementation
5. **Store migration** — Still using `writable` stores; planned to migrate to `$state` (Phase 2.1)
6. **Event-based navigation** — Pages use `document.dispatchEvent('go-next')`; planned to replace with callback props (Phase 2.3)
