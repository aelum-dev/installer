# AestheticOS Installer — Internationalization (i18n) Implementation Plan

## 1. Overview

### Problem
All UI strings are hardcoded in English. The installer supports 28 regions but displays exclusively in English. Users from non-English-speaking regions must navigate the entire installation in a language they may not fully understand.

### Goal
Provide a fully localized installer UI with:
- **Frontend UI strings** translated into multiple languages
- **Runtime locale detection** from the host OS
- **Persistent locale selection** across installer sessions
- **Dynamic language switching** (change language mid-wizard)
- **ICU Message Format** support for interpolation and plurals
- **TypeScript type safety** for translation keys

### Scope
This plan covers the **frontend installer UI only**. The installed system's locale (set via region selection) is separate and already functional.

---

## 2. Library Selection: `@svelte-intl/svelte-intl`

### Why this library?

| Criterion | `@svelte-intl/svelte-intl` | `svelte-i18n` (v4) | Custom Solution |
|-----------|---------------------------|-------------------|-----------------|
| Svelte 5 native | ✅ Runes ($state) | ❌ Svelte 4 stores | ✅ |
| ICU Message Format | ✅ plurals, selects | ✅ | ❌ |
| TypeScript types | ✅ auto-generated | ✅ | ❌ |
| Bundle size | ~1.5KB | ~5KB | ~0.5KB |
| Rich text (Snippets) | ✅ | ❌ | ❌ |
| Maturity | New (0.1.x) | Mature (4.x) | N/A |
| No external deps | ✅ | ✅ | ✅ |

### Decision
**Use `@svelte-intl/svelte-intl`** for initial implementation because:
- It's built specifically for Svelte 5 with native Rune support
- ICU Message Format handles our interpolation needs (e.g., "Install on {disk}")
- TypeScript types auto-generated from translation files
- Simple key-value dictionary approach matches the installer's scope
- Rich text support via Svelte Snippets for future use (e.g., clickable paths)

**Fallback option**: `svelte-i18n` v4.0+ if `@svelte-intl/svelte-intl` proves too young for production.

---

## 3. Architecture

```
frontend/src/
├── lib/
│   ├── i18n/
│   │   ├── index.ts              # i18n setup, exports: t, setLocale, locale
│   │   └── locales.ts            # Available locales list
│   ├── translations/
│   │   ├── en-US.json            # English (source of truth)
│   │   ├── de-DE.json            # German
│   │   ├── es-ES.json            # Spanish
│   │   ├── fr-FR.json            # French
│   │   ├── it-IT.json            # Italian
│   │   ├── pt-BR.json            # Portuguese (Brazil)
│   │   ├── ru-RU.json            # Russian
│   │   ├── zh-CN.json            # Simplified Chinese
│   │   ├── zh-TW.json            # Traditional Chinese
│   │   ├── ja-JP.json            # Japanese
│   │   ├── ko-KR.json            # Korean
│   │   ├── uk-UA.json            # Ukrainian
│   │   ├── pl-PL.json            # Polish
│   │   ├── tr-TR.json            # Turkish
│   │   ├── ar-SA.json            # Arabic
│   │   └── he-IL.json            # Hebrew
│   ├── validate.ts               # Validation logic (keep pure)
│   ├── validate-messages.ts      # Validation messages (i18n-aware)
│   └── ...
├── pages/
│   ├── WelcomePage.svelte        # → {t('page.welcome.title')}
│   ├── UserAccountPage.svelte    # → {t('page.userAccount.username')}
│   └── ...
└── components/
    ├── ConfirmDialog.svelte      # → {t('dialog.confirm.title')}
    └── ...
```

### Key Design Decisions

1. **Frontend-only i18n** — The backend already handles locale for the installed system. The installer UI is separate.
2. **JSON files, not TypeScript** — Translators can work with JSON without needing TypeScript knowledge.
3. **Flat key structure** — No deep nesting. Keys are dot-separated: `page.welcome.title`.
4. **Runtime locale from OS** — Auto-detect on first launch, user can override.
5. **Persistence via Wails** — Store selected locale in backend state (like `selectedLanguage`).

---

## 4. Translation Key Map

### Phase 1: Core Strings (All Pages)

| Key | English Source | Context |
|-----|---------------|---------|
| `app.brand` | "AestheticOS" | Brand name (may stay same) |
| `app.dryRun` | "DRY RUN" | Status badge |
| `page.welcome.title` | "Welcome to AestheticOS" | Welcome page |
| `page.welcome.subtitle` | "This installer will guide you through setting up your system." | Welcome page |
| `page.welcome.loading` | "Loading..." | Language list loading |
| `page.userAccount.title` | "Create user account" | User account page |
| `page.userAccount.username` | "Username" | Label |
| `page.userAccount.usernamePlaceholder` | "Enter username" | Input placeholder |
| `page.userAccount.password` | "Password" | Label |
| `page.userAccount.passwordPlaceholder` | "Enter password" | Input placeholder |
| `page.userAccount.confirmPassword` | "Confirm Password" | Label |
| `page.userAccount.confirmPasswordPlaceholder` | "Confirm password" | Input placeholder |
| `page.diskSelection.title` | "Select Installation Disk" | Disk selection page |
| `page.diskSelection.hint` | "Choose a disk and filesystem to install AestheticOS." | Disk selection hint |
| `page.diskSelection.fileSystem` | "Filesystem" | Filesystem label |
| `page.diskSelection.loading` | "Scanning for disks..." | Disk scan |
| `page.diskSelection.noDevices` | "No storage devices found." | Empty state |
| `page.diskSelection.scanPartitions` | "Scanning partitions..." | Partition scan |
| `page.diskSelection.noPartitions` | "No partitions on this disk." | Empty partitions |
| `page.confirmation.title` | "Installation Summary" | Confirmation page |
| `page.confirmation.sectionLanguage` | "Language" | Summary section |
| `page.confirmation.sectionUser` | "User Account" | Summary section |
| `page.confirmation.sectionDisk` | "Target Disk" | Summary section |
| `page.confirmation.username` | "Username" | Summary |
| `page.confirmation.password` | "Password" | Summary |
| `page.installation.title` | "Installing AestheticOS" | Installation page |
| `page.installation.target` | "Target" | Disk target label |
| `page.installation.completed` | "Installation completed!" | Success |
| `page.installation.initializing` | "Initializing..." | Before start |
| `page.installation.checkLog` | "Check {path} for details." | Error hint |
| `page.finish.title` | "Installation Complete" | Finish page |
| `page.finish.message` | "AestheticOS has been installed successfully." | Success message |
| `page.finish.reboot` | "Reboot" | Reboot button |
| `dialog.confirm.title` | "Confirm Installation" | Confirm dialog |
| `dialog.confirm.message` | "Are you sure you want to install AestheticOS on {model}? All data on this disk will be permanently erased." | With interpolation |
| `dialog.confirm.cancel` | "Cancel" | Cancel button |
| `dialog.confirm.yes` | "Yes, Erase & Install" | Confirm button |
| `validation.usernameRequired` | "Username is required" | Validation |
| `validation.usernameMinLength` | "Username must be at least 3 characters" | Validation |
| `validation.passwordRequired` | "Password is required" | Validation |
| `validation.passwordMinLength` | "Password must be at least 6 characters" | Validation |
| `validation.passwordsMatch` | "Please confirm your password" | Validation |
| `validation.passwordsNoMatch` | "Passwords do not match" | Validation |
| `validation.selectLanguage` | "Please select a language to continue." | Navigation |
| `validation.selectDisk` | "Please select a disk to continue." | Navigation |
| `validation.diskTooSmall` | "Disk is too small. Minimum {min} required, but selected disk is {size}." | With interpolation |
| `nav.back` | "Back" | Navigation (aria-label) |
| `nav.next` | "Next" | Navigation (aria-label) |
| `fs.ext4` | "ext4" | Filesystem option |
| `fs.btrfs` | "btrfs" | Filesystem option |
| `disk.hdd` | "HDD" | Disk type |
| `disk.ssd` | "SSD" | Disk type |
| `step.formatting` | "Formatting disk..." | Installation step |
| `step.mounting` | "Mounting filesystem..." | Installation step |
| `step.installingBase` | "Installing base system..." | Installation step |
| `step.installingDesktop` | "Installing desktop environment..." | Installation step |
| `step.setupUsers` | "Setting up users..." | Installation step |
| `step.configuringSystem` | "Configuring system..." | Installation step |
| `step.clearingCache` | "Clearing package cache..." | Installation step |

### ICU Message Examples

```json
{
  "dialog.confirm.message": "{model}, Are you sure you want to install AestheticOS on this disk? All data will be permanently erased."
}
```

Usage in Svelte:
```svelte
<p>{t('dialog.confirm.message', { model: disk.model })}</p>
```

---

## 5. Implementation Phases

### Phase 1: Foundation (Backend → Frontend Infrastructure)

**Duration**: ~2 days

#### 5.1. Install Library
```bash
cd frontend && npm install @svelte-intl/svelte-intl
```

#### 5.2. Create i18n Setup (`frontend/src/lib/i18n/index.ts`)
```typescript
import enUS from '../translations/en-US.json';
import { makeI18n } from '@svelte-intl/svelte-intl';

// Initially only English; others added in Phase 2
export const { t, setLocale, locale, format } = makeI18n(
  { 'en-US': enUS },
  'en-US'
);

export type Locale = 'en-US';
```

#### 5.3. Create English Translation File (`frontend/src/translations/en-US.json`)
- Extract ALL hardcoded strings from all pages/components
- Use flat dot-notation keys
- Apply ICU Message Format for interpolated strings

#### 5.4. Add Vite Plugin
```typescript
// vite.config.ts
import { svelteIntl } from '@svelte-intl/svelte-intl/vite';

export default defineConfig({
  plugins: [
    svelteIntl({
      localesDir: './src/translations',
      defaultLocale: 'en-US',
    }),
  ],
});
```

#### 5.5. Bootstrap i18n in `main.ts`
```typescript
import { setLocale } from '$lib/i18n';

// Detect OS locale on first launch
async function detectLocale(): Promise<string> {
  const osLocale = navigator.language; // e.g., "de-DE", "ja-JP"
  // Map to our supported locales
  const mapping: Record<string, string> = {
    'de': 'de-DE', 'es': 'es-ES', 'fr': 'fr-FR',
    'it': 'it-IT', 'pt-BR': 'pt-BR', 'ru': 'ru-RU',
    'zh-CN': 'zh-CN', 'zh-TW': 'zh-TW', 'ja': 'ja-JP',
    'ko': 'ko-KR', 'uk': 'uk-UA', 'pl': 'pl-PL',
    'tr': 'tr-TR', 'ar': 'ar-SA', 'he': 'he-IL',
  };
  return mapping[osLocale] || 'en-US';
}
```

#### 5.6. Add Backend Locale Persistence
- Extend `backend/gui/bindings.go` with `GetUILocale()` and `SetUILocale()`
- Store in existing state file (same location as `selectedLanguage`)
- On app start, call `detectLocale()` and set if no saved locale exists

---

### Phase 2: Welcome Page & Language Switcher

**Duration**: ~1 day

#### 5.7. Add Language Switcher to Welcome Page
- Replace static "Loading..." with i18n: `{t('page.welcome.loading')}`
- Keep existing language/region selector (it sets the installed system locale)
- Add a small language indicator that shows current UI locale

#### 5.8. Migrate WelcomePage Strings
```svelte
<PageLayout title={t('page.welcome.title')}>
  <p class="subtitle">{t('page.welcome.subtitle')}</p>
  {#if loading}
    <span class="loading-text">{t('page.welcome.loading')}</span>
  {/if}
</PageLayout>
```

---

### Phase 3: All Pages & Components

**Duration**: ~3 days

#### 5.9. Migrate Page Titles
Every `PageLayout` title becomes:
```svelte
<PageLayout title={t('page.welcome.title')} />
```

#### 5.10. Migrate Form Labels & Placeholders
```svelte
<label for="username">{t('page.userAccount.username')}</label>
<input placeholder={t('page.userAccount.usernamePlaceholder')} />
```

#### 5.11. Migrate Dialog Strings
```svelte
<h2>{t('dialog.confirm.title')}</h2>
<p>{t('dialog.confirm.message', { model: disk.model })}</p>
<button>{t('dialog.confirm.cancel')}</button>
<button>{t('dialog.confirm.yes')}</button>
```

#### 5.12. Migrate Navigation Strings
```svelte
<button aria-label={t('nav.back')} />
<button aria-label={t('nav.next')} />
```

#### 5.13. Migrate Installation Step Labels
- Backend sends step labels like "Formatting disk..." 
- These should be translated on the backend side (Go) OR mapped on frontend
- **Recommended**: Backend sends step IDs (e.g., "format_disk") and frontend maps via `t('step.formatting')`

#### 5.14. Migrate Error Messages
- Validation messages in `validate.ts` → move to `validate-messages.ts` with i18n
- Navigation error messages in `navigation.ts` → use `t('validation.selectDisk')`

---

### Phase 4: Validation Messages

**Duration**: ~1 day

#### 5.15. Create i18n Validation Messages
```typescript
// frontend/src/lib/validate-messages.ts
import { t } from '$lib/i18n';

export function getValidationMessages() {
  return {
    usernameRequired: t('validation.usernameRequired'),
    usernameMinLength: t('validation.usernameMinLength'),
    passwordRequired: t('validation.passwordRequired'),
    passwordMinLength: t('validation.passwordMinLength'),
    passwordsMatch: t('validation.passwordsMatch'),
    passwordsNoMatch: t('validation.passwordsNoMatch'),
  };
}
```

#### 5.16. Update `validateUserAccount` to Accept Messages
```typescript
export function validateUserAccount(
  username: string,
  password: string,
  confirmPassword: string,
  state: ValidationState,
  messages?: ReturnType<typeof getValidationMessages>
): ValidationResult
```

---

### Phase 5: RTL Support (Arabic, Hebrew)

**Duration**: ~1 day

#### 5.17. Add RTL Detection
```typescript
// frontend/src/lib/i18n/index.ts
const RTL_LOCALES = new Set(['ar-SA', 'he-IL']);

export const isRTL = $derived(RTL_LOCALES.has(locale.current));
```

#### 5.18. Apply `dir` Attribute
```svelte
<!-- main.ts or App.svelte root -->
<svelte:head>
  <html lang={locale.current} dir={isRTL ? 'rtl' : 'ltr'} />
</svelte:head>
```

#### 5.19. CSS RTL Support
- Use logical properties (`margin-inline-start` instead of `margin-left`)
- Add `.rtl` class-based overrides where needed
- Test disk card layout, form alignment, progress bar direction

---

### Phase 6: Additional Language Translations

**Duration**: ~2 weeks (ongoing, community-driven)

#### 5.20. Create Translation Files
Start with the most requested languages:
1. **German (de-DE)** — Large Arch user base
2. **Spanish (es-ES)** — Global reach
3. **French (fr-FR)** — Standard
4. **Chinese Simplified (zh-CN)** — Large market
5. **Japanese (ja-JP)** — Large market
6. **Korean (ko-KR)** — Growing Arch community
7. **Russian (ru-RU)** — Large Arch community
8. **Portuguese BR (pt-BR)** — Large Arch community
9. **Italian (it-IT)** — Standard
10. **Ukrainian (uk-UA)** — Standard
11. **Polish (pl-PL)** — Standard
12. **Turkish (tr-TR)** — Standard
13. **Arabic (ar-SA)** — RTL
14. **Hebrew (he-IL)** — RTL
15. **Chinese Traditional (zh-TW)** — Standard

#### 5.21. Translation Workflow
```bash
# Extract keys from all .svelte files
npx svelte-intl-extract --dir frontend/src/translations

# Generate template from English
cp frontend/src/translations/en-US.json frontend/src/translations/template.json
```

Consider using a translation platform (Lokalise, Weblate) for community contributions.

---

## 6. Backend Changes

### 6.1. UI Locale State
```go
// backend/gui/app.go
type App struct {
    // ... existing fields
    uiLocale string
}

func (a *App) GetUILocale() string {
    return a.uiLocale
}

func (a *App) SetUILocale(locale string) {
    a.uiLocale = locale
    // Persist to state file
}
```

### 6.2. Step Label Localization (Optional — Phase 2)
Instead of sending raw step labels from backend, send step IDs:
```go
// install/logger.go
type StepID string

const (
    StepFormatDisk     StepID = "format_disk"
    StepMountFilesystem StepID = "mount_filesystem"
    StepInstallBase     StepID = "install_base"
    StepInstallDesktop  StepID = "install_desktop"
    StepSetupUsers      StepID = "setup_users"
    StepConfigureSystem StepID = "configure_system"
    StepClearCache      StepID = "clear_cache"
)
```

Frontend maps: `t('step.format_disk')`, `t('step.install_base')`, etc.

### 6.3. Error Message Localization (Optional — Later)
```go
// install/errors.go
type InstallError struct {
    Code    string // e.g., "disk_not_found"
    Message string // English fallback
}
```

### 6.4. Backend Constants for Locale
```go
// backend/shared/config.go
var SupportedUILocales = []string{
    "en-US", "de-DE", "es-ES", "fr-FR", "it-IT",
    "pt-BR", "ru-RU", "zh-CN", "zh-TW", "ja-JP",
    "ko-KR", "uk-UA", "pl-PL", "tr-TR", "ar-SA", "he-IL",
}

var RTLLocales = map[string]bool{
    "ar-SA": true,
    "he-IL": true,
}
```

---

## 7. Filesystem Structure (Detailed)

```
frontend/src/
├── translations/
│   ├── en-US.json          # Source of truth (~150 keys)
│   ├── de-DE.json
│   ├── es-ES.json
│   ├── fr-FR.json
│   ├── it-IT.json
│   ├── pt-BR.json
│   ├── ru-RU.json
│   ├── zh-CN.json
│   ├── zh-TW.json
│   ├── ja-JP.json
│   ├── ko-KR.json
│   ├── uk-UA.json
│   ├── pl-PL.json
│   ├── tr-TR.json
│   ├── ar-SA.json
│   └── he-IL.json
├── lib/
│   ├── i18n/
│   │   ├── index.ts        # makeI18n, exports t, setLocale, locale, isRTL
│   │   └── locales.ts      # LOCALE_LIST, LOCALE_NAMES, RTL_LOCALES
│   ├── validate-messages.ts # getValidationMessages()
│   ├── validate.ts         # validateUserAccount() — accepts messages
│   ├── navigation.ts       # Uses t() for page error messages
│   └── ...
├── components/
│   ├── AppHeader.svelte    # t('app.dryRun')
│   ├── AppFooter.svelte    # t('nav.back'), t('nav.next')
│   ├── Button.svelte       # No strings (children prop)
│   ├── ConfirmDialog.svelte # t('dialog.confirm.*')
│   ├── ErrorBanner.svelte  # Message passed as prop
│   └── PageLayout.svelte   # title passed as prop
├── pages/
│   ├── WelcomePage.svelte  # t('page.welcome.*')
│   ├── UserAccountPage.svelte # t('page.userAccount.*')
│   ├── DiskSelectionPage.svelte # t('page.diskSelection.*')
│   ├── ConfirmationPage.svelte # t('page.confirmation.*')
│   ├── InstallationPage.svelte # t('page.installation.*')
│   └── FinishPage.svelte   # t('page.finish.*')
└── App.svelte              # i18n bootstrap
```

---

## 8. Testing Strategy

### 8.1. Unit Tests
```typescript
// frontend/src/lib/i18n/i18n.test.ts
import { describe, it, expect } from 'vitest';
import { t, setLocale, locale } from './index';

describe('i18n', () => {
  it('translates welcome title', () => {
    expect(t('page.welcome.title')).toBe('Welcome to AestheticOS');
  });

  it('interpolates messages', () => {
    const msg = t('dialog.confirm.message', { model: 'Samsung 990 PRO' });
    expect(msg).toContain('Samsung 990 PRO');
  });

  it('switches locale', () => {
    setLocale('de-DE');
    expect(locale.current).toBe('de-DE');
  });
});
```

### 8.2. Integration Tests
- Verify all translation keys exist in all locale files
- Check for missing translations (key exists in en-US but not in other locales)
- Test RTL rendering for Arabic/Hebrew

### 8.3. Manual Testing
- Switch languages mid-wizard and verify all strings update
- Test on VM with different OS locales
- Verify disk selection page with long translations (German compounds)
- Test progress bar and step labels during installation

---

## 9. Migration Checklist

### Phase 1
- [ ] Install `@svelte-intl/svelte-intl`
- [ ] Create `frontend/src/translations/en-US.json`
- [ ] Create `frontend/src/lib/i18n/index.ts`
- [ ] Add Vite plugin config
- [ ] Bootstrap i18n in `main.ts`
- [ ] Add backend `GetUILocale()` / `SetUILocale()`
- [ ] Run `npm run check` — no type errors

### Phase 2
- [ ] Migrate `WelcomePage.svelte` strings
- [ ] Migrate `PageLayout` title prop
- [ ] Test language switcher
- [ ] Run `npm run check`

### Phase 3
- [ ] Migrate `UserAccountPage.svelte` strings
- [ ] Migrate `DiskSelectionPage.svelte` strings
- [ ] Migrate `ConfirmationPage.svelte` strings
- [ ] Migrate `InstallationPage.svelte` strings
- [ ] Migrate `FinishPage.svelte` strings
- [ ] Migrate `ConfirmDialog.svelte` strings
- [ ] Migrate `AppHeader.svelte` strings
- [ ] Migrate `AppFooter.svelte` strings
- [ ] Migrate navigation error messages
- [ ] Run `npm run check`

### Phase 4
- [ ] Create `validate-messages.ts`
- [ ] Update `validateUserAccount` signature
- [ ] Update `validate.test.ts`
- [ ] Run `npm run test`

### Phase 5
- [ ] Add `isRTL` derived state
- [ ] Apply `dir` attribute to root element
- [ ] Fix CSS for RTL (logical properties)
- [ ] Test Arabic/Hebrew rendering
- [ ] Run `npm run check`

### Phase 6
- [ ] Create translation files for each target locale
- [ ] Verify all keys present in all files
- [ ] Add CI check for missing translations
- [ ] Update `AGENTS.md` with i18n patterns
- [ ] Update `CHANGELOG.md`

---

## 10. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `@svelte-intl/svelte-intl` is too new | Medium | Medium | Have `svelte-i18n` v4 as fallback; test thoroughly |
| Large translation files slow startup | Low | Low | Lazy-load non-English locales |
| RTL breaks layout | Medium | Medium | Test early with German (long text) first |
| Missing translations crash app | Low | High | Fallback to English for missing keys |
| Backend step labels not localized | Medium | Low | Frontend maps step IDs to translations |
| Community translation delays | High | Low | Ship with English + German first; add others via PRs |

---

## 11. Recommended Rollout Order

1. **English + German** (Phase 1-3) — Largest Arch Linux user base
2. **Spanish, French, Italian** (Phase 6) — Standard European languages
3. **Chinese Simplified, Japanese, Korean** (Phase 6) — Asian market
4. **Russian, Ukrainian, Polish, Turkish** (Phase 6) — Active Arch communities
5. **Portuguese BR** (Phase 6) — Growing Linux adoption
6. **Arabic, Hebrew** (Phase 5-6) — RTL support validation
7. **Chinese Traditional** (Phase 6) — Taiwan/HK market

---

## 12. Estimated Timeline

| Phase | Duration | Cumulative |
|-------|----------|------------|
| 1. Foundation | 2 days | 2 days |
| 2. Welcome Page | 1 day | 3 days |
| 3. All Pages | 3 days | 6 days |
| 4. Validation | 1 day | 7 days |
| 5. RTL Support | 1 day | 8 days |
| 6. Translations | 2 weeks (ongoing) | 8 days + ongoing |

**Minimum viable i18n** (Phases 1-4): **1 week** of focused development.
