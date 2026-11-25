import { describe, it, expect, afterEach } from 'vitest';
import { BACKEND_TO_FRONTEND, backendLocaleToFrontend, translationForLocale, setLocale, t } from './index.svelte';
import { DMS_LOCALES, LOCALES, isDmsLocale } from './locales';

afterEach(() => setLocale('en-US'));

describe('i18n locale registry', () => {
	it('derives a backend code for every bundled frontend locale', () => {
		const frontendCodes = new Set(Object.values(BACKEND_TO_FRONTEND));
		for (const code of Object.keys(LOCALES)) {
			expect(frontendCodes.has(code)).toBe(true);
		}
	});

	it('maps regional locale codes to the POSIX form', () => {
		expect(backendLocaleToFrontend('de_DE.UTF-8')).toBe('de-DE');
		expect(backendLocaleToFrontend('pt_BR.UTF-8')).toBe('pt-BR');
	});

	it('maps a region-less locale code', () => {
		expect(backendLocaleToFrontend('eo.UTF-8')).toBe('eo');
	});

	it('returns undefined for an unknown backend locale', () => {
		expect(backendLocaleToFrontend('xx_XX.UTF-8')).toBeUndefined();
	});

	it('falls back to English for a key missing from the active locale', () => {
		setLocale('de-DE');
		expect(t('app.brand')).toBe('Aelum');
	});

	it('updates the document language for assistive tech', () => {
		setLocale('ar-SA');
		expect(document.documentElement.lang).toBe('ar-SA');
	});

	it('returns the key itself when no locale defines it', () => {
		expect(t('missing.translation.key')).toBe('missing.translation.key');
	});
});

describe('desktop (DMS) translation set', () => {
	// The set mirrors dux-shell's shipped languages (22 poexports + English).
	it('holds the 23 languages the desktop speaks', () => {
		expect(DMS_LOCALES.size).toBe(23);
	});

	// A code with no installer translation has no row to mark, so a DMS code that
	// is not a bundled locale would silently never light up a row.
	it('is a subset of the installer locales', () => {
		for (const code of DMS_LOCALES) {
			expect(LOCALES, code).toHaveProperty(code);
		}
	});

	// The installer-only languages: translated chrome, but the desktop stays in
	// English. These are the rows that get the "only installer" tooltip.
	it('excludes the installer-only languages', () => {
		for (const code of ['el-GR', 'fi-FI', 'nb-NO', 'ca-ES', 'id-ID']) {
			expect(isDmsLocale(code), code).toBe(false);
			expect(LOCALES, code).toHaveProperty(code);
		}
	});

	it('recognises a desktop locale and rejects an unknown one', () => {
		expect(isDmsLocale('de-DE')).toBe(true);
		expect(isDmsLocale('eo')).toBe(true);
		expect(isDmsLocale('xx-XX')).toBe(false);
	});
});

describe('translation lookup across territory variants', () => {
	// The exact territory still wins when it is a shipped translation.
	it('matches an exact territory first', () => {
		expect(translationForLocale('ru_RU.UTF-8')).toBe('ru-RU');
		expect(translationForLocale('zh_CN.UTF-8')).toBe('zh-CN');
		expect(translationForLocale('zh_TW.UTF-8')).toBe('zh-TW');
	});

	// The bug this guards: a translated language offered under another territory
	// ("Русский (Ukraine)") must read the same translation as its home territory.
	it('falls back to the language for an unshipped territory', () => {
		expect(translationForLocale('ru_UA.UTF-8')).toBe('ru-RU');
		expect(translationForLocale('de_AT.UTF-8')).toBe('de-DE');
		expect(translationForLocale('es_MX.UTF-8')).toBe('es-ES');
		expect(translationForLocale('pt_PT.UTF-8')).toBe('pt-BR');
		expect(translationForLocale('en_GB.UTF-8')).toBe('en-US');
	});

	// A codeset or @modifier on the locale must not defeat the match.
	it('ignores the codeset and modifier', () => {
		expect(translationForLocale('de_DE.UTF-8@euro')).toBe('de-DE');
		expect(translationForLocale('de_AT.UTF-8@euro')).toBe('de-DE');
	});

	// Chinese is the one language code with two shipped written forms, so the
	// territory — not the bare language — decides which one a row wants.
	it('splits Chinese by territory into Simplified and Traditional', () => {
		expect(translationForLocale('zh_HK.UTF-8')).toBe('zh-TW');
		expect(translationForLocale('zh_MO.UTF-8')).toBe('zh-TW');
		expect(translationForLocale('zh_SG.UTF-8')).toBe('zh-CN');
	});

	// A language with no translation stays untranslated, whatever its territory.
	it('returns undefined for an untranslated language', () => {
		expect(translationForLocale('nn_NO.UTF-8')).toBeUndefined();
		expect(translationForLocale('sr_RS.UTF-8@latin')).toBeUndefined();
		expect(translationForLocale('xx_XX.UTF-8')).toBeUndefined();
	});

	// A region-less locale (Esperanto) resolves through the language path too.
	it('resolves a region-less locale', () => {
		expect(translationForLocale('eo.UTF-8')).toBe('eo');
	});
});

// Bundled exactly like the runtime (see index.svelte.ts) so the audit covers
// every shipped locale, including ones added later.
const translationModules = import.meta.glob<Record<string, string>>('../../translations/*.json', {
	eager: true,
	import: 'default',
});

const translationsByLocale = Object.fromEntries(
	Object.entries(translationModules).map(([path, messages]) => [
		path.slice(path.lastIndexOf('/') + 1, -'.json'.length),
		messages,
	]),
);

function placeholders(message: string): string[] {
	return [...message.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
}

describe('translation completeness', () => {
	const english = translationsByLocale['en-US'];
	const englishKeys = Object.keys(english).sort();

	// The Welcome page builds some keys from the current tab (`page.welcome.${tab}Placeholder`),
	// and a dynamic key is exactly the kind a rename breaks silently: the runtime falls back to
	// printing the key itself, so the page still renders and nobody notices until someone reads
	// it. These are the keys it can build — if the page changes its prefixes, change these.
	it('defines every key the Welcome page builds from its tab', () => {
		for (const prefix of ['language', 'region', 'timezone']) {
			for (const suffix of ['Placeholder', 'Empty']) {
				expect(englishKeys, `page.welcome.${prefix}${suffix}`).toContain(`page.welcome.${prefix}${suffix}`);
			}
		}
		for (const key of [
			'page.welcome.language',
			'page.welcome.region',
			'page.welcome.timezone',
			'page.welcome.keyboardValue',
			'page.welcome.mirrorsLocalValue',
			'page.welcome.mirrorsWorldValue',
			'page.welcome.summary',
			'page.welcome.timezoneOnlyCountry',
			'page.welcome.timezoneAllZones',
		]) {
			expect(englishKeys, key).toContain(key);
		}
	});

	for (const [locale, messages] of Object.entries(translationsByLocale)) {
		it(`${locale} matches en-US keys and placeholders`, () => {
			expect(Object.keys(messages).sort()).toEqual(englishKeys);
			for (const key of englishKeys) {
				expect(placeholders(messages[key]), `${locale} ${key}`).toEqual(
					placeholders(english[key]),
				);
			}
		});
	}
});
