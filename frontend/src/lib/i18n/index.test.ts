import { describe, it, expect, afterEach } from 'vitest';
import { BACKEND_TO_FRONTEND, backendLocaleToFrontend, setLocale, t } from './index.svelte';
import { LOCALES } from './locales';

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
