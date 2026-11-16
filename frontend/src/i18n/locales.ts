// To add a new language create ./locales/<file>.json and add a line here in alphabetical order

export interface Locale {
  /** Name of the locale to display in UI */
  name: string;
  /** filename in ./locales without the extension */
  file: string;
  /** BCP 47 tag matching navigator.language */
  tag: string;
}

// The key is the id saved in the user settings
export const locales: Record<string, Locale> = {
  ar: { name: 'العربية', file: 'ar', tag: 'ar' },
  bg: { name: 'Български', file: 'bg', tag: 'bg' },
  cz: { name: 'Čeština', file: 'cz', tag: 'cs' },
  de: { name: 'Deutsch', file: 'de', tag: 'de' },
  el: { name: 'Ελληνικά', file: 'el', tag: 'el' },
  en: { name: 'English', file: 'en', tag: 'en' },
  es: { name: 'Español', file: 'es', tag: 'es' },
  eu: { name: 'Euskara', file: 'eu', tag: 'eu' },
  fr: { name: 'Français', file: 'fr', tag: 'fr' },
  he: { name: 'עברית', file: 'he', tag: 'he' },
  hu: { name: 'Magyar', file: 'hu', tag: 'hu' },
  it: { name: 'Italiano', file: 'it', tag: 'it' },
  ja: { name: '日本語', file: 'ja', tag: 'ja' },
  ko: { name: '한국어', file: 'ko', tag: 'ko' },
  nl: { name: 'Nederlands', file: 'nl', tag: 'nl' },
  nlBE: { name: 'Nederlands (België)', file: 'nl-be', tag: 'nl-be' },
  pl: { name: 'Polski', file: 'pl', tag: 'pl' },
  pt: { name: 'Português', file: 'pt', tag: 'pt' },
  ptBR: { name: 'Português (Brasil)', file: 'pt-br', tag: 'pt-br' },
  ro: { name: 'Romanian', file: 'ro', tag: 'ro' },
  ru: { name: 'Русский', file: 'ru', tag: 'ru' },
  sk: { name: 'Slovenčina', file: 'sk', tag: 'sk' },
  svSE: { name: 'Swedish (Sweden)', file: 'sv-se', tag: 'sv-se' },
  tr: { name: 'Türkçe', file: 'tr', tag: 'tr' },
  ua: { name: 'Українська', file: 'ua', tag: 'uk' },
  zhCN: { name: '中文 (简体)', file: 'zh-cn', tag: 'zh-cn' },
  zhTW: { name: '中文 (繁體)', file: 'zh-tw', tag: 'zh-tw' },
};
