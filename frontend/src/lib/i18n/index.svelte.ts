import { LOCALES } from './locales';

// Static imports for available locales
import enUS from '../../translations/en-US.json';
import deDE from '../../translations/de-DE.json';
import esES from '../../translations/es-ES.json';
import frFR from '../../translations/fr-FR.json';
import zhCN from '../../translations/zh-CN.json';

// Build translations record
const translations: Record<string, Record<string, string>> = {
  'en-US': enUS,
  'de-DE': deDE,
  'es-ES': esES,
  'fr-FR': frFR,
  'zh-CN': zhCN,
};

// Svelte $state for guaranteed reactivity
let _locale = $state('en-US');

export const t = (key: string, args?: Record<string, unknown>): string => {
  // Get the message using current locale
  const msg = translations[_locale]?.[key] ?? translations['en-US']?.[key] ?? key;
  
  // Simple interpolation for ICU-style messages
  if (!args) return msg;
  
  return msg.replace(/\{(\w+)\}/g, (_, k) => String(args[k] ?? ''));
};

export const setLocale = (l: string) => {
  _locale = l;
};

export const locale = { current: _locale };

export const format = {
  number: (n: number) => n.toString(),
  dateTime: (d: Date) => d.toString(),
  relativeDateTime: (v: number, u: string) => `${v} ${u}`,
};

// RTL detection
import { RTL_LOCALES } from './locales';
export const getIsRTL = () => RTL_LOCALES.has(_locale);

export type Locale = keyof typeof LOCALES;

// Map browser locale codes to our supported locales
const LOCALE_MAP: Record<string, string> = {
  'de': 'de-DE',
  'es': 'es-ES',
  'fr': 'fr-FR',
  'it': 'it-IT',
  'pt-BR': 'pt-BR',
  'pt': 'pt-BR',
  'ru': 'ru-RU',
  'zh-CN': 'zh-CN',
  'zh-TW': 'zh-TW',
  'zh': 'zh-CN',
  'ja': 'ja-JP',
  'ko': 'ko-KR',
  'uk': 'uk-UA',
  'pl': 'pl-PL',
  'tr': 'tr-TR',
  'ar': 'ar-SA',
  'he': 'he-IL',
};

export async function detectLocale(): Promise<string> {
  const osLocale = navigator.language;
  return LOCALE_MAP[osLocale] ?? LOCALE_MAP[osLocale.split('-')[0]] ?? 'en-US';
}
