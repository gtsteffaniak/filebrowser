// i18n.js
import { createI18n } from 'vue-i18n';

// Import English (always included)
import en from './en.json';

// Import all other languages statically
// These will be tree-shaken by Vite based on the conditional logic below
import he from './he.json';
import hu from './hu.json';
import ar from './ar.json';
import de from './de.json';
import el from './el.json';
import es from './es.json';
import fr from './fr.json';
import is from './is.json';
import it from './it.json';
import ja from './ja.json';
import ko from './ko.json';
import nlBE from './nl-be.json';
import pl from './pl.json';
import pt from './pt.json';
import ptBR from './pt-br.json';
import ro from './ro.json';
import ru from './ru.json';
import sk from './sk.json';
import ua from './ua.json';
import svSE from './sv-se.json';
import zhCN from './zh-cn.json';
import zhTW from './zh-tw.json';
import cz from './cz.json';

// Create messages object based on build environment
// Vite's tree-shaking should remove unused languages in development builds
const messages: Record<string, any> = import.meta.env.DEV_BUILD === 'true' 
  ? { en } // Development: English only
  : { // Production: All languages
      en, he, hu, ar, de, el, es, fr, is, it, ja, ko, nlBE, pl, pt, ptBR, ro, ru, sk, svSE, ua, zhCN, zhTW, cz
    };

type LocaleMap = { [key: string]: string };

export const internalToStandardLocaleMap: { [key: string]: string } = {
  nlBE: 'nl-be',
  ptBR: 'pt-br',
  svSE: 'sv-se',
  zhCN: 'zh-cn',
  zhTW: 'zh-tw',
  cz: 'cs',
  ua: 'uk',
};

export function toStandardLocale(locale: string): string {
  return internalToStandardLocaleMap[locale] || locale;
}

export function detectLocale(): string {
  const locale = navigator.language.toLowerCase();
  const localeMap: LocaleMap = {
    'he': 'he',
    'hu': 'hu',
    'ar': 'ar',
    'el': 'el',
    'es': 'es',
    'en': 'en',
    'is': 'is',
    'it': 'it',
    'fr': 'fr',
    'pt-br': 'ptBR',
    'pt': 'pt',
    'ja': 'ja',
    'zh-tw': 'zhTW',
    'zh-cn': 'zhCN',
    'zh': 'zhCN',
    'de': 'de',
    'ro': 'ro',
    'ru': 'ru',
    'pl': 'pl',
    'ko': 'ko',
    'cz': 'cz',
    'sk': 'sk',
    'tr': 'tr',
    'uk': 'uk',
    'sv-se': 'svSE',
    'sv': 'svSE',
    'nl-be': 'nlBE',
  };

  for (const key in localeMap) {
    if (locale.startsWith(key)) {
      return localeMap[key];
    }
  }
  return 'en-us'; // Default fallback
}

// List of RTL languages
export const rtlLanguages = ['he', 'ar'];

// Function to check if locale is RTL
export const isRtl = (locale: string) => {
  const currentLocale = locale || i18n.global.locale;
  return rtlLanguages.includes(currentLocale);
};

export function setLocale(locale: string) {
  // With legacy: true, locale is a string, not a ref
  // With legacy: false, locale is a ref and needs .value
  if (typeof i18n.global.locale === 'string') {
    // Legacy mode - direct assignment
    (i18n.global as any).locale = locale;
  } else {
    // Composition mode - use .value
    (i18n.global.locale as any).value = locale;
  }
}


// Create i18n instance
const i18n = createI18n({
  locale: detectLocale(),
  fallbackLocale: 'en',
  // expose i18n.global for outside components
  legacy: true,
  messages,
});

export default i18n;
