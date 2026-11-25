export const LOCALES = {
  'en-US': 'English (US)',
  'de-DE': 'Deutsch (DE)',
  'el-GR': 'Ελληνικά (GR)',
  'es-ES': 'Español (ES)',
  'fr-FR': 'Français (FR)',
  'it-IT': 'Italiano (IT)',
  'pt-BR': 'Português (BR)',
  'ru-RU': 'Русский (RU)',
  'zh-CN': '简体中文 (CN)',
  'zh-TW': '繁體中文 (TW)',
  'ja-JP': '日本語 (JP)',
  'ko-KR': '한국어 (KR)',
  'uk-UA': 'Українська (UA)',
  'pl-PL': 'Polski (PL)',
  'tr-TR': 'Türkçe (TR)',
  'ar-SA': 'العربية (SA)',
  'he-IL': 'עברית (IL)',
  'fi-FI': 'Suomi (FI)',
  'hu-HU': 'Magyar (HU)',
  'nb-NO': 'Norsk bokmål (NO)',
  'nl-NL': 'Nederlands (NL)',
  'sv-SE': 'Svenska (SE)',
  'bg-BG': 'Български (BG)',
  'ca-ES': 'Català (ES)',
  'fa-IR': 'فارسی (IR)',
  'id-ID': 'Bahasa Indonesia (ID)',
  'vi-VN': 'Tiếng Việt (VN)',
  'eo': 'Esperanto',
} as const;

export const RTL_LOCALES = new Set(['ar-SA', 'he-IL', 'fa-IR']);

export type Locale = keyof typeof LOCALES;

/**
 * The installer languages the desktop (Dux Shell, "DMS") is translated into as
 * well. This mirrors dux-shell's shipped set — the 22 files under
 * `quickshell/translations/poexports` plus its English source, 23 in all —
 * written in the installer's own BCP 47 codes so it lines up with LOCALES.
 *
 * It is a deliberate copy, not a live read: the installer cannot ask the desktop
 * what it speaks (its translations are compiled into the shell binary), so the
 * one fact the Welcome page needs — whether a translated language also carries a
 * translated desktop — is curated here. Keep it a subset of LOCALES: a language
 * with no installer translation has no row to mark, and a code outside LOCALES
 * would silently never match. When dux-shell adds or drops a language, move the
 * matching code here.
 */
export const DMS_LOCALES = new Set<Locale>([
	'en-US',
	'de-DE',
	'es-ES',
	'fr-FR',
	'it-IT',
	'pt-BR',
	'ru-RU',
	'zh-CN',
	'zh-TW',
	'ja-JP',
	'ko-KR',
	'uk-UA',
	'pl-PL',
	'tr-TR',
	'ar-SA',
	'he-IL',
	'hu-HU',
	'nl-NL',
	'sv-SE',
	'bg-BG',
	'fa-IR',
	'vi-VN',
	'eo',
]);

/** True when the desktop is translated into this frontend locale code. */
export function isDmsLocale(code: string): boolean {
	return DMS_LOCALES.has(code as Locale);
}
