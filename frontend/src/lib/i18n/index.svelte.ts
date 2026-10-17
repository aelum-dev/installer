import { LOCALES, RTL_LOCALES } from './locales';

// Every translation file under src/translations is bundled eagerly and keyed by
// its frontend locale code (the filename). Adding a locale is then a one-file
// change: drop in the JSON and add its display name to locales.ts.
const translationModules = import.meta.glob<Record<string, string>>(
	'../../translations/*.json',
	{ eager: true, import: 'default' },
);

const translations: Record<string, Record<string, string>> = Object.fromEntries(
	Object.entries(translationModules).map(([path, messages]) => [
		path.slice(path.lastIndexOf('/') + 1, -'.json'.length),
		messages,
	]),
);

// Svelte $state for guaranteed reactivity
let _locale = $state('en-US');

export const t = (key: string, args?: Record<string, unknown>): string => {
	// Flat key lookup only (e.g. 'app.brand', 'validation.selectDisk', 'validation.modal.title')
	const msg = translations[_locale]?.[key] ?? translations['en-US']?.[key] ?? key;
	return interpolate(msg, args);
};

function interpolate(msg: string, args: Record<string, unknown> | undefined): string {
	if (!args) return msg;
	return msg.replace(/\{(\w+)\}/g, (_, k) => String(args[k] ?? ''));
}

export const setLocale = (l: string) => {
	_locale = l;
	// Keep the document language in sync for screen readers and CSS :lang().
	if (typeof document !== 'undefined') document.documentElement.lang = l;
};

export const locale = {
	get current() {
		return _locale;
	},
};

// RTL detection
export const getIsRTL = () => RTL_LOCALES.has(locale.current);

// Backend locale codes use the POSIX form (e.g. "de_DE.UTF-8"); the frontend
// keys translations by BCP 47 (e.g. "de-DE"). The mapping is mechanical, so it
// is derived from the locale list instead of being maintained by hand.
export function frontendLocaleToBackend(code: string): string {
	const [language, region] = code.split('-');
	return region ? `${language}_${region}.UTF-8` : `${language}.UTF-8`;
}

export const BACKEND_TO_FRONTEND: Record<string, string> = Object.fromEntries(
	Object.keys(translations).map((code) => [frontendLocaleToBackend(code), code]),
);

/** Map backend locale string to frontend locale code */
export function backendLocaleToFrontend(rawLocale: string): string | undefined {
	return BACKEND_TO_FRONTEND[rawLocale];
}

export type Locale = keyof typeof LOCALES;
