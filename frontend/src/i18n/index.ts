import { createI18n } from 'vue-i18n';

// Import translations
import he from './he.json';
import hu from './hu.json';
import ar from './ar.json';
import de from './de.json';
import el from './el.json';
import en from './en.json';
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

// Function to detect locale
export function detectLocale() {
  let locale = navigator.language.toLowerCase();
  switch (true) {
    case /^he\b/.test(locale):
      locale = 'he';
      break;
    case /^hu\b/.test(locale):
      locale = 'hu';
      break;
    case /^ar\b/.test(locale):
      locale = 'ar';
      break;
    case /^el.*/i.test(locale):
      locale = 'el';
      break;
    case /^es\b/.test(locale):
      locale = 'es';
      break;
    case /^en\b/.test(locale):
      locale = 'en';
      break;
    case /^is\b/.test(locale):
      locale = 'is';
      break;
    case /^it\b/.test(locale):
      locale = 'it';
      break;
    case /^fr\b/.test(locale):
      locale = 'fr';
      break;
    case /^pt-br\b/.test(locale):
      locale = 'pt-br';
      break;
    case /^pt\b/.test(locale):
      locale = 'pt';
      break;
    case /^ja\b/.test(locale):
      locale = 'ja';
      break;
    case /^zh-tw\b/.test(locale):
      locale = 'zh-tw';
      break;
    case /^zh-cn\b/.test(locale):
    case /^zh\b/.test(locale):
      locale = 'zh-cn';
      break;
    case /^de\b/.test(locale):
      locale = 'de';
      break;
    case /^ro\b/.test(locale):
      locale = 'ro';
      break;
    case /^ru\b/.test(locale):
      locale = 'ru';
      break;
    case /^pl\b/.test(locale):
      locale = 'pl';
      break;
    case /^ko\b/.test(locale):
      locale = 'ko';
      break;
    case /^sk\b/.test(locale):
      locale = 'sk';
      break;
    case /^tr\b/.test(locale):
      locale = 'tr';
      break;
    case /^uk\b/.test(locale):
      locale = 'uk';
      break;
    case /^sv-se\b/.test(locale):
    case /^sv\b/.test(locale):
      locale = 'sv';
      break;
    case /^nl-be\b/.test(locale):
      locale = 'nl-be';
      break;
    default:
      locale = 'en';
  }
  return locale;
}

// List of RTL languages
export const rtlLanguages: string[] = ['he', 'ar'];

// Function to check if locale is RTL
export const isRtl = (locale?: string): boolean => {
  // Determine the current locale, defaulting to i18n's locale if not provided
  const currentLocale = locale || i18n.global.locale;

  // Ensure the locale is a valid string before checking
  if (typeof currentLocale !== 'string') {
    console.error('Current locale is not a valid string');
    return false;
  }

  // Check if the locale is in the rtlLanguages array
  return rtlLanguages.includes(currentLocale);
};

// Create i18n instance
const i18n = createI18n({
  locale: detectLocale(),
  fallbackLocale: 'en',
  messages: {
    he,
    hu,
    ar,
    de,
    el,
    en,
    es,
    fr,
    is,
    it,
    ja,
    ko,
    'nl-be': nlBE,
    pl,
    'pt-br': ptBR,
    pt,
    ru,
    ro,
    sk,
    'sv-se': svSE,
    ua,
    'zh-cn': zhCN,
    'zh-tw': zhTW,
  },
});

export default i18n;
