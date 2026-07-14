# Frontend Refactoring Plan

## Status: Phase 1 ✅ Complete

**Commit:** `e8ef447`

### What Phase 1 did:
- Created `PageLayout.svelte` component (shared page layout with absolute heading)
- Added design tokens in `style.css` (colors, typography, spacing, layout, shadows)
- Migrated all 5 pages and all components to use design tokens
- Removed ~188 lines of duplicated CSS

---

## Phase 2 — Architecture (next)

### 2.1 Replace `writable` store with Svelte 5 `$state`
**File:** `frontend/src/lib/store.ts`

**Current:**
```ts
import { writable } from 'svelte/store';
export const userAccountData = writable<{ username: string; password: string; confirmPassword: string }>({...});
```

**To:**
```ts
export let userAccountData = $state({
    username: '',
    password: '',
    confirmPassword: '',
});
```

**Affected files:**
- `frontend/src/App.svelte` — remove `get()` calls, use direct property access
- `frontend/src/pages/UserAccountPage.svelte` — remove `get()` call
- Any other file importing from `$lib/store`

---

### 2.2 Extract navigation logic from `App.svelte`
**File:** `frontend/src/lib/useWizard.ts` (new)

Move out of `App.svelte`:
- `currentPage` state
- `goToNext()`, `goToBack()` logic
- `validateNext()` function
- `animKey`, `animDirection`, `animLeaving`, `animEntering` state
- `animateTransition()` function

`App.svelte` becomes a thin layout shell.

---

### 2.3 Replace event-based navigation with store/callbacks
**Current:** Pages dispatch `document.dispatchEvent(new CustomEvent('go-next'))`

**To:** Pass `onNext` callback prop to pages that need it (e.g., WelcomePage).

**Affected files:**
- `frontend/src/App.svelte` — pass `onNext` prop
- `frontend/src/pages/WelcomePage.svelte` — call `onNext()` instead of dispatching event

---

## Phase 3 — Component improvements

### 3.1 Reusable `FormField` component
**New file:** `frontend/src/components/FormField.svelte`

Extract from `UserAccountPage.svelte` — each field block (label + input + error) becomes:
```svelte
<FormField label="Username" error={errors.username} bind:value={username} />
```

---

### 3.2 Flesh out stub pages
- `InstallationPage.svelte` — placeholder with "This is page 4 of 5"
- `FinishPage.svelte` — placeholder with "This is page 5 of 5"

Add real content or clear TODO markers.

---

### 3.3 Button `onclick` → `onClick`
**File:** `frontend/src/components/Button.svelte`

Change `onclick={(e) => onClick?.(e)}` to `onClick={(e) => onClick?.(e)}` (Svelte convention).

---

## How to continue

In a new session, start with:
> "Let's continue with Phase 2 of the frontend refactoring. The plan is in REFACTORING_PLAN.md. Phase 1 is committed at e8ef447."
