// i18n.js
import { createI18n } from 'vue-i18n';

// Import translations (alphabetical order)
import ar from './ar.json';
import cz from './cz.json';
import de from './de.json';
import el from './el.json';
import en from './en.json';
import es from './es.json';
import fr from './fr.json';
import he from './he.json';
import hu from './hu.json';
import is from './is.json';
import it from './it.json';
import ja from './ja.json';
import ko from './ko.json';
import nl from './nl.json';
import nlBE from './nl-be.json';
import pl from './pl.json';
import pt from './pt.json';
import ptBR from './pt-br.json';
import ro from './ro.json';
import ru from './ru.json';
import sk from './sk.json';
import svSE from './sv-se.json';
import ua from './ua.json';
import zhCN from './zh-cn.json';
import zhTW from './zh-tw.json';

type LocaleMap = { [key: string]: string };

// Map of all imported translation modules (alphabetical order)
// This is the single source for all translations
const translationModules: { [key: string]: any } = {
  ar, cz, de, el, en, es, fr, he, hu, is, it, ja, ko, nl, nlBE, pl, pt, ptBR, ro, ru, sk, svSE, ua, zhCN, zhTW
};

// Shared list of available locales for the application
// Auto-generated from translation modules
export const availableLocales: LocaleMap = Object.keys(translationModules).reduce((acc, key) => {
  // Convert internal keys to their display format
  const displayMap: { [key: string]: string } = {
    nlBE: 'nl-be',
    ptBR: 'pt-br',
    svSE: 'sv-se',
    zhCN: 'zh-cn',
    zhTW: 'zh-tw',
  };
  acc[key] = displayMap[key] || key;
  return acc;
}, {} as LocaleMap);

// Maps internal locale names to standard BCP 47 codes (for navigator.language)
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
  const browserLocale = navigator.language.toLowerCase();

  // Map of browser locale codes to internal locale keys
  const browserToInternalMap: LocaleMap = {
    'pt-br': 'ptBR',
    'zh-tw': 'zhTW',
    'zh-cn': 'zhCN',
    'zh': 'zhCN',
    'sv-se': 'svSE',
    'sv': 'svSE',
    'nl-be': 'nlBE',
  };

  // Check for exact matches first (including variants like pt-br)
  if (browserToInternalMap[browserLocale]) {
    return browserToInternalMap[browserLocale];
  }

  // Check for language prefix matches (e.g., 'en-US' -> 'en')
  const languagePrefix = browserLocale.split('-')[0];

  // If the language prefix exists in our translations, use it
  if (translationModules[languagePrefix]) {
    return languagePrefix;
  }

  // Try browser-specific mappings for the prefix
  if (browserToInternalMap[languagePrefix]) {
    return browserToInternalMap[languagePrefix];
  }

  return 'en'; // Default fallback
}

// List of RTL languages
export const rtlLanguages = ['he', 'ar'];

// Function to check if locale is RTL
export const isRtl = (locale: string) => {
  const currentLocale = locale || i18n.global.locale;
  return rtlLanguages.includes(currentLocale);
};

export function setLocale(locale: string) {
  // according to doc u only need .value if legacy: false but they lied
  // https://vue-i18n.intlify.dev/guide/essentials/scope.html#local-scope-1
  //@ts-ignore
  i18n.global.locale.value = locale;
}


// Create i18n instance with auto-generated messages from imported modules
const i18n = createI18n({
  locale: detectLocale(),
  fallbackLocale: 'en',
  // expose i18n.global for outside components
  legacy: true,
  messages: translationModules,
});

export default i18n;
