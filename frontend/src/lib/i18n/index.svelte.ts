import { LOCALES } from './locales';

// Static imports for available locales
import enUS from '../../translations/en-US.json';
import deDE from '../../translations/de-DE.json';
import elGR from '../../translations/el-GR.json';
import esES from '../../translations/es-ES.json';
import frFR from '../../translations/fr-FR.json';
import zhCN from '../../translations/zh-CN.json';
import ruRU from '../../translations/ru-RU.json';
import ukUA from '../../translations/uk-UA.json';
import itIT from '../../translations/it-IT.json';
import ptBR from '../../translations/pt-BR.json';
import zhTW from '../../translations/zh-TW.json';
import jaJP from '../../translations/ja-JP.json';
import koKR from '../../translations/ko-KR.json';
import plPL from '../../translations/pl-PL.json';
import trTR from '../../translations/tr-TR.json';
import arSA from '../../translations/ar-SA.json';
import heIL from '../../translations/he-IL.json';
import fiFI from '../../translations/fi-FI.json';
import huHU from '../../translations/hu-HU.json';
import nbNO from '../../translations/nb-NO.json';
import nlNL from '../../translations/nl-NL.json';
import svSE from '../../translations/sv-SE.json';
import bgBG from '../../translations/bg-BG.json';
import caES from '../../translations/ca-ES.json';
import faIR from '../../translations/fa-IR.json';
import idID from '../../translations/id-ID.json';
import viVN from '../../translations/vi-VN.json';
import eoLang from '../../translations/eo.json';

// Build translations record
const translations: Record<string, Record<string, string>> = {
  'en-US': enUS,
  'de-DE': deDE,
  'el-GR': elGR,
  'es-ES': esES,
  'fr-FR': frFR,
  'zh-CN': zhCN,
  'ru-RU': ruRU,
  'uk-UA': ukUA,
  'it-IT': itIT,
  'pt-BR': ptBR,
  'zh-TW': zhTW,
  'ja-JP': jaJP,
  'ko-KR': koKR,
  'pl-PL': plPL,
  'tr-TR': trTR,
  'ar-SA': arSA,
  'he-IL': heIL,
  'fi-FI': fiFI,
  'hu-HU': huHU,
  'nb-NO': nbNO,
  'nl-NL': nlNL,
  'sv-SE': svSE,
  'bg-BG': bgBG,
  'ca-ES': caES,
  'fa-IR': faIR,
  'id-ID': idID,
  'vi-VN': viVN,
  'eo': eoLang,
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

export const locale = {
  get current() { return _locale; },
};

// RTL detection
import { RTL_LOCALES } from './locales';
export const getIsRTL = () => RTL_LOCALES.has(locale.current);

// Backend locale codes (e.g. "de_DE.UTF-8") → frontend locale codes (e.g. "de-DE")
export const BACKEND_TO_FRONTEND: Record<string, string> = {
  'en_US.UTF-8': 'en-US',
  'de_DE.UTF-8': 'de-DE',
  'el_GR.UTF-8': 'el-GR',
  'es_ES.UTF-8': 'es-ES',
  'fr_FR.UTF-8': 'fr-FR',
  'it_IT.UTF-8': 'it-IT',
  'pt_BR.UTF-8': 'pt-BR',
  'ru_RU.UTF-8': 'ru-RU',
  'zh_CN.UTF-8': 'zh-CN',
  'zh_TW.UTF-8': 'zh-TW',
  'ja_JP.UTF-8': 'ja-JP',
  'ko_KR.UTF-8': 'ko-KR',
  'uk_UA.UTF-8': 'uk-UA',
  'pl_PL.UTF-8': 'pl-PL',
  'tr_TR.UTF-8': 'tr-TR',
  'ar_SA.UTF-8': 'ar-SA',
  'he_IL.UTF-8': 'he-IL',
  'fa_IR.UTF-8': 'fa-IR',
  'fi_FI.UTF-8': 'fi-FI',
  'hu_HU.UTF-8': 'hu-HU',
  'nb_NO.UTF-8': 'nb-NO',
  'nl_NL.UTF-8': 'nl-NL',
  'sv_SE.UTF-8': 'sv-SE',
  'bg_BG.UTF-8': 'bg-BG',
  'ca_ES.UTF-8': 'ca-ES',
  'id_ID.UTF-8': 'id-ID',
  'vi_VN.UTF-8': 'vi-VN',
  'eo.UTF-8': 'eo',
};

/** Map backend locale string to frontend locale code */
export function backendLocaleToFrontend(rawLocale: string): string | undefined {
  return BACKEND_TO_FRONTEND[rawLocale];
}

export type Locale = keyof typeof LOCALES;
