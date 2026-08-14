export const LOCALES = {
  'en-US': 'English (US)',
  'de-DE': 'Deutsch (DE)',
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
} as const;

export const RTL_LOCALES = new Set(['ar-SA', 'he-IL']);

export type Locale = keyof typeof LOCALES;
