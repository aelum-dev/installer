// Light / dark theme state, shared across the wizard.
//
// The active scheme lives on <html data-theme> (set by a no-flash inline script
// in index.html before first paint, and by setTheme on every toggle). The choice
// is persisted to localStorage under `aelum-theme`; when unset the OS
// prefers-color-scheme is followed. Mirrors the website's theme.js / theme-toggle.js.

export const STORAGE_KEY = 'aelum-theme';
export type Theme = 'light' | 'dark';

let _theme = $state<Theme>(initialTheme());

// Read the scheme the pre-paint inline script already applied to <html>.
function initialTheme(): Theme {
	return document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light';
}

export const getTheme = (): Theme => _theme;

export const setTheme = (theme: Theme): void => {
	_theme = theme;
	document.documentElement.setAttribute('data-theme', theme);
	try {
		window.localStorage.setItem(STORAGE_KEY, theme);
	} catch {
		// Storage may be unavailable (private mode / disabled) — the in-memory
		// theme still applies for this session.
	}
};

export const toggleTheme = (): void => {
	setTheme(_theme === 'dark' ? 'light' : 'dark');
};
