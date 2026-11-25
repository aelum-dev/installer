import { LOCALES, RTL_LOCALES } from './locales';

export { isDmsLocale } from './locales';

// The locale the chrome falls back to: both the initial state and the reset when
// a language with no UI translation is selected. English is the source language
// every translation file is measured against (see index.test.ts).
export const DEFAULT_LOCALE = 'en-US';

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
let _locale = $state(DEFAULT_LOCALE);

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

// The installer ships one translation per language, but the language list offers
// every territory glibc can build: "Русский (Russia)" and "Русский (Ukraine)" are
// two rows of one language, "Deutsch (Germany)" and "Deutsch (Austria)" two more.
// A row is translated when its LANGUAGE is, not only when its exact territory is —
// so every territory variant of a translated language marks alike and switches the
// chrome to that language's one translation. The installed system still gets the
// row's own locale; only the wizard's language is shared across the variants.
//
// One language code carries two written forms the installer translates apart —
// Chinese, Simplified (zh-CN) and Traditional (zh-TW) — so a bare language match
// is not enough for it: the territory decides which form a row wants.
const TRADITIONAL_CHINESE = new Set(['TW', 'HK', 'MO']);

// Base language code → the frontend locales that write it. Only "zh" has two.
const BASE_TO_FRONTEND: Record<string, string[]> = (() => {
	const map: Record<string, string[]> = {};
	for (const code of Object.keys(translations)) {
		(map[code.split('-')[0]] ??= []).push(code);
	}
	return map;
})();

// Split a POSIX locale ("de_AT.UTF-8@euro") into its language and territory,
// dropping the codeset and any @modifier the table carries.
function parsePosixLocale(rawLocale: string): { language: string; region: string } {
	const [language = '', region = ''] = rawLocale.split('@')[0].split('.')[0].split('_');
	return { language, region };
}

/**
 * The installer translation a backend locale should show, or undefined when its
 * language has none. The exact territory first, then the language's own
 * translation so every territory variant of a translated language is treated
 * alike. This is what the Welcome page marks and switches to; the exact
 * {@link backendLocaleToFrontend} is the strict form, used where a locale must
 * match a shipped file one-for-one.
 */
export function translationForLocale(rawLocale: string): string | undefined {
	const exact = BACKEND_TO_FRONTEND[rawLocale];
	if (exact) return exact;

	const { language, region } = parsePosixLocale(rawLocale);
	const candidates = BASE_TO_FRONTEND[language];
	if (!candidates || candidates.length === 0) return undefined;
	if (candidates.length === 1) return candidates[0];

	// More than one written form is translated (Chinese): the territory picks
	// which. Hong Kong, Macau and Taiwan read Traditional; all else Simplified.
	const wantTraditional = TRADITIONAL_CHINESE.has(region.toUpperCase());
	return (
		candidates.find((c) => TRADITIONAL_CHINESE.has(c.split('-')[1] ?? '') === wantTraditional) ?? candidates[0]
	);
}

export type Locale = keyof typeof LOCALES;
