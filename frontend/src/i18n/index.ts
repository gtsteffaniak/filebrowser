// i18n.js
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

type LocaleMap = { [key: string]: string };

export function detectLocale(): string {
  let locale = navigator.language.toLowerCase();
  console.log("locale", locale);

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
    'pt-br': 'pt-br',
    'pt': 'pt',
    'ja': 'ja',
    'zh-tw': 'zh-tw',
    'zh-cn': 'zh-cn',
    'zh': 'zh-cn',
    'de': 'de',
    'ro': 'ro',
    'ru': 'ru',
    'pl': 'pl',
    'ko': 'ko',
    'sk': 'sk',
    'tr': 'tr',
    'uk': 'uk',
    'sv-se': 'sv',
    'sv': 'sv',
    'nl-be': 'nl-be',
  };

  for (const key in localeMap) {
    if (locale.startsWith(key)) {
      return localeMap[key];
    }
  }

  console.log("is default");
  return 'en'; // Default fallback
}


// List of RTL languages
export const rtlLanguages = ['he', 'ar'];

// Function to check if locale is RTL
export const isRtl = (locale: string) => {
  const currentLocale = locale || i18n.global.locale;
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
