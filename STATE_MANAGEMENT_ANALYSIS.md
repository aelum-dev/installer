# State Management Analysis — Modern Practices vs. Current Implementation

## Executive Summary

Your project uses a **reasonable but dated** approach to state management. The core architecture (frontend stores + backend struct as single source of truth) is sound for a wizard-style installer, but there are several areas where modern practices diverge from your current implementation. Most issues are **low-severity** for this specific use case (a linear installer that runs once and reboots), but they're worth addressing.

---

## 1. Frontend State: `writable` Stores vs. `$state`

### Current
```typescript
// store.ts
import { writable } from 'svelte/store';
export const userAccountData = writable<UserAccountData>({ ... });
```

Accessed via `get(store)` or `.subscribe()`.

### Modern Practice (Svelte 5+)

The Svelte 5 docs and community consensus are clear:

> **`$state` is the default and preferred way to manage reactive state.** Stores should be reserved for specific use cases like complex async data streams or legacy code.

**Recommended pattern for shared reactive state:**

```typescript
// store.ts — Svelte 5 approach
export const userAccountData = $state<UserAccountData>({
  username: '',
  password: '',
  confirmPassword: '',
});
```

**Why it's better:**
- No `.subscribe()` boilerplate — direct property access
- No `get()` calls — import and use directly
- Cleaner developer experience
- `$state` uses Proxies under the hood for objects, same as stores but without the store abstraction layer

### Verdict for Your Project

**✅ Phase 2.1 in your REFACTORING_PLAN already addresses this.** This is the highest-priority fix. Your planned migration is correct.

---

## 2. Data Storage Model: In-Memory Only

### Current
```
┌─────────────────────────────────────────┐
│ Frontend: Writable stores (RAM)         │
│ Backend: Go struct fields (RAM)         │
│                                          │
│ NO PERSISTENCE between runs             │
└─────────────────────────────────────────┘
```

All data lives only in memory. If the app crashes or is closed, everything is lost.

### Modern Practice

For **wizard/installer apps**, there are two schools of thought:

| Approach | Use Case | Examples |
|----------|----------|----------|
| **In-memory only** (yours) | Linear, one-shot workflows | Your installer, setup wizards |
| **Persistent intermediate state** | Long, complex, interruptible workflows | OS installers with resume, multi-step config tools |

**What modern installers do:**
- **Anaconda (RHEL/Fedora)** — Saves progress to `/tmp` after each step, allows resume
- **Calamares (Kubuntu, Manjaro)** — Persists state to a temp file, supports going back and forth
- **Ubuntu installer** — Saves partial state, can restart from any step

**When persistence matters:**
- Installation takes >30 minutes (yours does: 7 steps)
- User might need to close/reopen app
- Network or power interruption is possible
- Validation can fail and user needs to retry without re-entering data

### Verdict for Your Project

**⚠️ Acceptable for now, but recommended to add.** Your installer has 7 steps that each take 0.6–2 seconds (dry run) or much longer (real install). If the app crashes mid-install, the user loses all form data. A simple persistence layer would help:

```go
// backend/persistence.go
func (a *App) SaveProgress() error {
  data := map[string]interface{}{
    "currentPage":  a.currentPage,
    "userAccount":  a.userAccount,
    "diskSelection": a.diskSelection,
  }
  return json.MarshalIndent(data, "", "  ")
}
```

Write to a temp file on each page transition. Restore on startup.

---

## 3. Password Storage

### Current
```
Frontend store:    { username: "admin", password: "secret123" }   ← PLAINTEXT
Backend struct:    UserAccountData{Password: "secret123"}         ← PLAINTEXT
```

Passwords travel in plaintext through:
1. Frontend `$state` variables
2. Frontend writable store
3. Wails IPC bridge (JSON)
4. Go struct field

### Modern Practice (OWASP)

> **Passwords should never be stored in plaintext.** Use Argon2id, bcrypt, or PBKDF2 with unique salt per password.

**However — your context is different from a typical web app:**

| Factor | Web App | Your Installer |
|--------|---------|----------------|
| Attack surface | Exposed to internet | Runs locally on user's machine |
| Data at rest | Database on server | In-memory during one session |
| Threat model | Attacker compromises server | Attacker has physical access to machine |
| Password reuse | Critical concern | User creates password for their own system |

**The reality:** Your installer is creating a **local system account**, not authenticating against a service. The password is eventually passed to `useradd` / `chpasswd` on the target system, which hashes it with `SHA-512` + salt using `crypt()`.

### Verdict for Your Project

**✅ Acceptable for this specific use case, with caveats.**

The password only exists in memory for the duration of the installer session. It's never written to disk in plaintext by your app. The target system will hash it properly.

**Recommendations:**
1. **Clear password fields from memory after use** — Overwrite the struct fields after passing to `useradd`
2. **Use `SecureString` or equivalent** — Not available in Go, but you can zero out memory
3. **Consider keyring integration** — On Linux, you could use `libsecret` (via Go binding) to store credentials securely if you ever add a "save profile" feature

---

## 4. Event-Based Communication Between Pages

### Current
```typescript
// Child page
document.dispatchEvent(new CustomEvent('go-next'));

// Parent (App.svelte)
$effect(() => {
  const handler = () => goToNext();
  document.addEventListener('go-next', handler);
  return () => document.removeEventListener('go-next', handler);
});
```

### Modern Practice

**Component props with callbacks** are preferred over global event dispatching:

```typescript
// App.svelte
<WelcomePage onNext={goToNext} />

// Child component
let { onNext }: { onNext: () => void } = $props();
```

**Why:**
- Explicit dependencies — you can see what a component needs
- Type-safe — TypeScript can verify the callback signature
- No global namespace pollution
- Easier to test — pass mock callbacks
- No memory leak risk from missed event removal

Your **REFACTORING_PLAN Phase 2.3** already identifies this.

### Verdict for Your Project

**✅ Phase 2.3 in your plan is correct.** This is a well-known anti-pattern that should be addressed.

---

## 5. Backend as Single Source of Truth

### Current
```
Dual state problem:
  Frontend store: userAccountData  ← "source of truth" (claimed)
  Backend struct: App.userAccount  ← actual persisted state
  
  Validation: Frontend validates on blur, Backend validates on "next"
  → Two validation layers, both must pass
```

### Modern Practice

For **Wails/IPC desktop apps**, there are two patterns:

| Pattern | Frontend Role | Backend Role | Best For |
|---------|--------------|--------------|----------|
| **Frontend-authoritative** (yours) | Stores data, sends to backend for validation | Validates, rejects if invalid | Form wizards, data entry |
| **Backend-authoritative** | Stateless display layer | Owns all state, frontend reads via getters | Complex workflows, multi-user |

Your hybrid approach (frontend stores + backend validation) is **the recommended pattern for wizard-style installers**. The frontend owns the UI state for responsiveness, the backend validates for security.

### Verdict for Your Project

**✅ Your approach is correct.** This is the standard pattern for Wails/Svelte desktop apps.

---

## 6. Disk Data Handling

### Current
```
Backend: lsblk JSON → parse → []DiskInfo → JSON over IPC
Frontend: Store entire DiskInfo objects in memory
```

### Modern Practice

For **system data that doesn't change during the session**, your approach is fine. However, consider:

- **Disk detection is expensive** — `lsblk` spawns a subprocess. You're only calling it once on page mount, which is correct.
- **Storing full disk objects** — You're keeping the entire `DiskInfo` struct in memory for display. This is reasonable for a list of 2-10 disks.
- **No caching** — If the user goes back to the disk page, you re-fetch. Consider caching.

### Verdict for Your Project

**✅ Correct.** No changes needed.

---

## 7. Installation Progress Data

### Current
```
Backend goroutine → EventsEmit("installation:progress") → Frontend EventsOn → $state
```

### Modern Practice

This is **the correct pattern** for Wails event-driven communication. The Wails docs recommend this exact approach for async operations that need to report progress to the UI.

### Verdict for Your Project

**✅ Best practice.** No changes needed.

---

## Summary Table

| Area | Current | Modern Practice | Verdict | Priority |
|------|---------|-----------------|---------|----------|
| Frontend stores | `writable` | `$state` | ⚠️ Needs update | **HIGH** — Phase 2.1 |
| Data persistence | In-memory only | Consider temp file for resume | ⚠️ Acceptable now | **MEDIUM** — Add after Phase 2 |
| Password storage | Plaintext in memory | Hash before storing | ✅ Acceptable for context | **LOW** — Only if adding "save profile" |
| Page communication | Custom events | Callback props | ⚠️ Needs update | **HIGH** — Phase 2.3 |
| Backend state model | Frontend stores + backend validation | Standard hybrid | ✅ Correct | **N/A** |
| Disk data | lsblk JSON → IPC | Correct pattern | ✅ Correct | **N/A** |
| Installation events | EventsEmit/EventsOn | Standard pattern | ✅ Correct | **N/A** |

---

## Recommended Action Plan

1. **Do Phase 2.1 immediately** — Migrate `writable` stores to `$state`. This is the single biggest win.
2. **Do Phase 2.3** — Replace custom events with callback props. Clean architecture.
3. **Add progress persistence** (medium-term) — Save partial state to `/tmp/aesthetic-os-installer.state` on each page transition. Restore on startup. This is especially important since real installation takes minutes.
4. **Password memory clearing** (low priority) — After passing password to `useradd`, zero out the struct field. Good hygiene.
5. **Consider keyring integration** (future) — If you add a "save installation profile" feature, use `libsecret` on Linux.
