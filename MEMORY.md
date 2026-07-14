# Memory — Known Issues & Fixes

## Project Stack
- **Backend**: Go 1.25 + Wails v2.13
- **Frontend**: Svelte 5 (runes mode) + TypeScript + Vite 6
- **Target**: Arch-based Linux distro installer, light theme, barebones functional design

---

## 1. Wails Go Bindings Generation

**Issue**: After updating `app.go` with new exported methods, the frontend still only had the old `Greet` binding. TypeScript compilation failed because the generated `.d.ts` didn't include new methods.

**Fix**: Run `wails generate module` from the project root after every change to exported Go methods. This regenerates:
- `frontend/wailsjs/go/main/App.d.ts` — TypeScript type declarations
- `frontend/wailsjs/go/main/App.js` — JS IPC bridge functions
- `frontend/wailsjs/go/models.ts` — Struct model types

**Command**: `wails generate module`

---

## 2. Vite Cannot Resolve `wailsjs` Directory

**Issue**: `npm run build` failed with:
```
Could not resolve "../../wailsjs/go/main/App" from "src/App.svelte"
```

**Root cause**: The `wailsjs` directory lives outside `src/`. Vite/Rollup only resolves modules from `node_modules` and the `src/` directory by default.

**Fix**: Add a resolve alias in `frontend/vite.config.ts`:
```ts
import path from 'path'

export default defineConfig({
  resolve: {
    alias: {
      '@wails': path.resolve('./wailsjs'),
      // ... other aliases
    },
  },
})
```

Then import using the alias:
```ts
import { CanGoNext } from '@wails/go/main/App.js';
```

**Note**: The `.js` extension is required — ESM modules need explicit file extensions.

---

## 3. Svelte 5 Runes Mode — Legacy Syntax Errors

### `export let` → `$props()`
**Issue**: `Cannot use export let in runes mode`
**Fix**: Replace all `export let prop: Type = default;` with destructured `$props()`:
```ts
let { children, direction = 'forward' }: { children: any; direction?: string } = $props();
```

### `<svelte:component>` deprecated
**Issue**: `` `<svelte:component>` is deprecated in runes mode ``
**Fix**: Use conditional `{#if}` blocks instead of dynamic component rendering:
```svelte
{#if currentPage === 0}
  <WelcomePage />
{:else if currentPage === 1}
  <UserAccountPage />
{/if}
```

### `$:` rune not allowed in runes mode
**Issue**: `` `$:` is not allowed in runes mode, use `$derived` or `$effect` instead ``
**Fix**: Replace all `$:` reactive statements with `$derived` (for computed values) or `$effect` (for side effects):
```ts
// Before
$: className = 'base' + (active ? ' active' : '');

// After
let className = $derived('base' + (active ? ' active' : ''));
```

---

## 4. PageTransition Component — Dynamic Class Management

### `class:list` with object literals causes TS parse errors
**Issue**: `class:list={{ leaving, entering }}` inside `{#key}` blocks caused:
```
Error: Left side of comma operator is unused and has no side effects.
Error: ';' expected.
```

**Fix**: Use a computed string via `$derived` instead:
```ts
let className = $derived('transition-content' + (leaving ? ' leaving' : '') + (entering ? ' entering' : ''));
```
```svelte
<div class={className}>
```

### `bind:this` doesn't expose exported component methods
**Issue**: Tried to use `bind:this={pageRef}` and call `pageRef.animate()` on a Svelte 5 component. This doesn't work — Svelte 5 components don't expose their exports as instance properties.

**Fix**: Lift animation state (key, direction, leaving/entering booleans) to the parent component and pass them as props. The child becomes a pure presentational wrapper.

### CSS unused selector warnings for dynamic classes
**Issue**: `Unused CSS selector ".transition-content.entering"` — Svelte's scoped CSS compiler can't detect classes applied via `class:list` or computed `class=` strings at runtime.

**Fix**: Add suppress comments before each affected selector:
```css
/* svelte-ignore css_unused_selector */
.transition-content.entering {
  opacity: 0;
}
```

### Attribute selectors on regular HTML elements trigger warnings
**Issue**: `.transition-content[transition-direction="forward"].leaving` still triggers `css_unused_selector` because Svelte doesn't recognize attribute selectors on plain `<div>` elements.

**Fix**: Prefix with the element type:
```css
/* svelte-ignore css_unused_selector */
div[data-transition-direction="forward"].leaving {
  transform: translateX(0);
}
```

---

## 5. Animation State Management Pattern

The working pattern for page transitions:

1. **Parent (App.svelte)** manages animation state: `animKey`, `animDirection`, `animLeaving`, `animEntering`
2. **Child (PageTransition.svelte)** receives all four as props — it's purely presentational
3. Animation flow in parent:
   ```ts
   async function animateTransition(direction) {
     animDirection = direction;
     animLeaving = true;            // phase 1: leave animation (200ms)
     await delay(200);
     animKey++;                     // triggers {#key} re-render → content swap
     animLeaving = false;           // start enter phase
     animEntering = true;
     await delay(200);              // wait for enter animation (200ms)
     animEntering = false;          // done
   }
   ```
4. CSS handles the actual transitions via `transition` property with class-based keyframes

---

## 6. TypeScript Path Aliases Must Match in Both Files

**Issue**: Added `$components`, `$pages`, `$lib` aliases in `vite.config.ts` but forgot to add matching entries in `tsconfig.json`. TypeScript would report "cannot find module" errors even though Vite resolved them at build time.

**Fix**: Always mirror path aliases in both files:
```json
// tsconfig.json
{
  "compilerOptions": {
    "paths": {
      "$components/*": ["src/components/*"],
      "$pages/*": ["src/pages/*"],
      "$lib/*": ["src/lib/*"]
    }
  }
}
```

---

## 7. Dropdown Popup Clipped by Parent Overflow

**Issue**: Custom dropdown component's popup list was visually covered/hidden behind the footer. The dropdown had `z-index: 1000` but still didn't appear.

**Root cause**: `.content-area` has `overflow: hidden` (needed for page transition animations). This clips *everything* inside it regardless of z-index — the dropdown popup extends past the bottom of `.content-area` and gets clipped by the overflow, not covered by the footer.

**Fix**: Change `.content-area` from `overflow: hidden` to `overflow: visible`. The page transition animations still work because they use `{#key}` re-rendering rather than overflow-based clipping.

```css
.content-area {
    /* ... */
    overflow: visible;  /* was: hidden */
}
```

---

## 8. Locking Window Size in Wails v2

**Issue**: Tried to set `Resizable: false` on the Wails app config, but the field doesn't exist.

**Fix**: Use `MinWidth`, `MaxWidth`, `MinHeight`, `MaxHeight` all set to the same values:
```go
wails.Run(&options.App{
    Title:     "my-app",
    Width:     900,
    Height:    600,
    MinWidth:  900,
    MaxWidth:  900,
    MinHeight: 600,
    MaxHeight: 600,
})
```

---

## 9. Consistent Heading Positioning Across Pages

**Issue**: Each page has its own heading (`<h1>` or `<h2>`) and they should all appear at the same vertical position.

**Fix**: Use absolute positioning on all page headings:
```css
h1, h2 {
    position: absolute;
    top: 24px;
    left: 50%;
    transform: translateX(-50%);
}
```
This keeps headings at a fixed distance from the content area top without affecting the layout flow of other page content.

---

## 10. Custom Dropdown Component Pattern

A reusable dropdown component (`Dropdown.svelte`) was created to replace native `<select>` for full light-theme control. Key design decisions:

- **Props**: `options`, `selectedValue`, `placeholder`, `width`, `onChange`
- **Keyboard nav**: Arrow keys, Enter, Space, Escape
- **ARIA**: `role="combobox"` on button, `role="listbox"`/`role="option"` on list
- **Click outside** closes via `$effect` with `mousedown` listener
- **Max height + scroll**: `max-height: 200px; overflow-y: auto` prevents overflow beyond window
- **Dynamic classes use clsx form**: `class={{ 'selected': condition }}` instead of `class:list` to avoid CSS unused selector warnings
- **Avoid attribute selectors in CSS**: Use `.dropdown--open` instead of `[aria-expanded='true']` to prevent `css_unused_selector` warnings

---

## 11. Page-to-Parent Communication via Custom Events

When a child page needs to trigger navigation in the parent App, use custom events on `document`:

```ts
// Child page dispatches:
document.dispatchEvent(new CustomEvent('go-next'));

// Parent listens:
$effect(() => {
    const handler = () => goToNext();
    document.addEventListener('go-next', handler);
    return () => document.removeEventListener('go-next', handler);
});
```

---

## 12. Custom Button Component (`Button.svelte`)

A reusable aesthetic button component was created to replace native `<button>` elements throughout the app.

### Location
`frontend/src/components/Button.svelte`

### Variants
| Variant | Look | Use Case |
|---------|------|----------|
| `primary` | Solid blue (`#4a90d9`) with shadow lift on hover | Main CTAs like "Get Started" |
| `secondary` | White with subtle border, lifts on hover | "Next →", "Finish" actions |
| `ghost` | Transparent bg, light gray border (`#d8d8d8`) | "← Back" navigation |

### Props
- `variant?: 'primary' | 'secondary' | 'ghost'` (default: `'primary'`)
- `size?: 'small' | 'medium' | 'large'` (default: `'medium'`)
- `disabled?: boolean` (default: `false`)
- `fullWidth?: boolean` (default: `false`)
- `onClick?: (e: MouseEvent) => void`
- `children: Snippet` — use `{@render children()}` in template, NOT `<slot />`

### Usage
```svelte
<Button variant="primary" size="large" onClick={handleClick}>
    Get Started
</Button>
```

### Key Implementation Details
- **NO `<slot />`** — deprecated in Svelte 5. Use `{@render children()}` with `children: Snippet` prop type.
- **NO `$derived.by()`** for class computation — causes HMR instability. Use a plain function `getBtnClass(): string` instead.
- **Event forwarding**: Use `onClick?: (e: MouseEvent) => void` prop, call via `onclick={(e) => onClick?.(e)}` on the native button element.

### Replaced Components
- `AppFooter.svelte` — Back (`ghost`) and Next/Finish (`secondary`) buttons
- `WelcomePage.svelte` — Get Started (`primary`, `large`) button

---

## 13. Wails Dev Mode — HMR Unreliable with New Component Files

**Issue**: Adding a new `.svelte` component file or modifying files that import new components causes `wails dev` to show a blank dark bluish page (the app crashes in the WebView). The app works after a full rebuild (`wails build`) but breaks intermittently during `wails dev`.

**Root cause**: Known Svelte 5 HMR bug. When Vite hot-reloads after adding new component files, the Wails WebView loses connection to the dev server. This is documented in Svelte issues (#12403) and Wails template README as a known limitation.

**Fix / Workaround**:
- **Always use `wails build` + run the binary** for reliable testing after component changes
- If `wails dev` goes blank: delete `frontend/node_modules/.vite` cache, then `fuser -k 34115/tcp 5173/tcp` to clear ports, restart
- Accept that HMR in `wails dev` is unreliable with new `.svelte` files — it may work after modifying existing files but not after adding new ones

**Development workflow**:
```bash
# After adding/modifying .svelte components:
fuser -k 34115/tcp 2>/dev/null; fuser -k 5173/tcp 2>/dev/null
wails build && ./build/bin/aesthetic-os-installer
```

---

## Checklist for Future Sessions

When modifying this project:

1. ✅ After changing exported Go methods → run `wails generate module`
2. ✅ Import Wails bindings using `@wails/...` alias, not relative paths
3. ✅ Always include `.js` extension on ESM imports
4. ✅ Use `$props()` — never `export let`
5. ✅ Use `$derived` — never `$:` statements
6. ✅ Use conditional blocks — never `<svelte:component>`
7. ✅ Mirror path aliases in both `vite.config.ts` AND `tsconfig.json`
8. ✅ Run `npx svelte-check` after every component change
9. ✅ Run `wails build` (NOT `wails dev`) for reliable testing with new `.svelte` files — HMR is broken in `wails dev` when adding new components
10. ✅ Use `MinWidth/MaxWidth/MinHeight/MaxHeight` (not `Resizable`) to lock window size in Wails v2
11. ✅ Use `overflow: visible` on `.content-area` for dropdowns/popups to escape overflow clipping
12. ✅ Position all page headings absolutely at `top: 24px` for consistent vertical alignment
13. ✅ Use `class={{ ... }}` (clsx form) instead of `class:list` to avoid CSS unused selector warnings
14. ✅ Avoid attribute selectors in scoped CSS — use regular classes like `.dropdown--open`
15. ✅ Use `{@render children()}` with `children: Snippet` prop — never `<slot />` (deprecated in Svelte 5)
16. ✅ Use plain function for dynamic class computation — never `$derived.by()` (causes HMR instability)
