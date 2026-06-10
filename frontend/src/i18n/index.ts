// i18n.js
import { createI18n } from 'vue-i18n';
import { nextTick } from 'vue';
import en from './en.json';

type MessageSchema = typeof en;

// All the locales in alphabetical order
export const availableLocales: Record<string, string> = {
  // internal keys: 'display value'
  ar: 'ar',
  cz: 'cz',
  de: 'de',
  el: 'el',
  en: 'en',
  es: 'es',
  fr: 'fr',
  he: 'he',
  hu: 'hu',
  is: 'is',
  it: 'it',
  ja: 'ja',
  ko: 'ko',
  nl: 'nl',
  nlBE: 'nl-be',
  pl: 'pl',
  pt: 'pt',
  ptBR: 'pt-br',
  ro: 'ro',
  ru: 'ru',
  sk: 'sk',
  svSE: 'sv-se',
  ua: 'ua',
  zhCN: 'zh-cn',
  zhTW: 'zh-tw',
};

const availableLocalesMap = new Map(Object.entries(availableLocales));

// Maps internal locale names to standard BCP 47 codes (for navigator.language)
const internalToStandard = new Map<string, string>([
  ['nlBE', 'nl-be'],
  ['ptBR', 'pt-br'],
  ['svSE', 'sv-se'],
  ['zhCN', 'zh-cn'],
  ['zhTW', 'zh-tw'],
  ['cz', 'cs'],
  ['ua', 'uk'],
]);

export function toStandardLocale(locale: string): string {
  return internalToStandard.get(locale) ?? locale;
}

export function detectLocale(): string {
  const browserLocale = navigator.language.toLowerCase();

  // Map of browser locale codes to internal locale keys
  const browserToInternalMap = new Map<string, string>([
    ['pt-br', 'ptBR'],
    ['zh-tw', 'zhTW'],
    ['zh-cn', 'zhCN'],
    ['zh',    'zhCN'],
    ['sv-se', 'svSE'],
    ['sv',    'svSE'],
    ['nl-be', 'nlBE'],
  ]);
  const mappedLocale = browserToInternalMap.get(browserLocale);
  if (mappedLocale !== undefined) {
    return mappedLocale;
  }
  const prefix = browserLocale.split('-')[0];
  return availableLocalesMap.get(prefix) ?? 'en';
}

// List of RTL languages
export const rtlLanguages = ['he', 'ar'];

// Function to check if locale is RTL
export const isRtl = (locale?: string) => rtlLanguages.includes(locale || i18n.global.locale.value);

function setLanguage(locale: string) {
  i18n.global.locale.value = locale;
  document.querySelector('html')?.setAttribute('lang', toStandardLocale(locale));
}

// Create i18n instance with auto-generated messages from imported modules
const i18n = createI18n<MessageSchema, string, false>({
  locale: 'en',
  fallbackLocale: 'en',
  legacy: false,
  messages: { en },
});

// Set English as initial language and update the html lang
setLanguage('en');

// import.meta.glob is vite-specific
// here we are preloading all json files as lazy chunks, except 'en.json' since this one will be always loaded.
const localeModules = import.meta.glob<{ default: Record<string, unknown> }>('./!(en).json');

export async function setLocale(locale: string) {
  // If the locale doesn't exist in our list, fallback to English
  if (!availableLocalesMap.has(locale)) {
    setLanguage('en');
    return;
  }
  // If the locale is already loaded just switch to it
  if (i18n.global.availableLocales.includes(locale)) {
    setLanguage(locale);
    return;
  }
  // But if isn't loaded, we will load it dynamically.
  try {
    const fileName = availableLocalesMap.get(locale);
    if (!fileName) {
      setLanguage('en');
      return;
    }
    const messages = (await localeModules[`./${fileName}.json`]()).default;
    i18n.global.setLocaleMessage(locale, messages);
    setLanguage(locale);
    await nextTick();
  } catch (e) {
    console.error("Failed to set locale:", e);
    setLanguage('en'); // Just in case that anything fails, fallback to english
  }
}

const detected = detectLocale();
if (detected !== 'en') void setLocale(detected);

export default i18n;
