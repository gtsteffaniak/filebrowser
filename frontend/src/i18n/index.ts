import { createI18n, useI18n as useVueI18n, type Composer } from 'vue-i18n';
import { nextTick } from 'vue';
import en from './locales/en.json' with { type: "json" };
import { locales } from './locales.ts';
import type { MessageParams, OptionalParam } from './messages.generated.ts';

type MessageSchema = typeof en;

// Type vue-i18n' against en.json
declare module 'vue-i18n' {
  export interface DefineLocaleMessage extends MessageSchema {}
}

/** Every valid translation key generated from en.json */
export type MessageKey = keyof MessageParams;

type RequiredParams<K extends MessageKey> = Exclude<MessageParams[K], OptionalParam>;

/** Keys that can be translated without params. Use it for data that is later passed to `t(key)` on its own */
export type PlainMessageKey = {
  [K in MessageKey]: [RequiredParams<K>] extends [never] ? K : never;
}[MessageKey];

type MessageArgs<K extends MessageKey> = MessageKey extends K
  ? [named?: never]
  : [MessageParams[K]] extends [never]
  ? []
  : [RequiredParams<K>] extends [never]
    ? [named?: { [P in MessageParams[K]]?: unknown }]
    : [
        named: { [P in RequiredParams<K>]: unknown } & {
          [P in Extract<MessageParams[K], OptionalParam>]?: unknown;
        },
      ];

type ExactKey<K extends MessageKey> = MessageKey extends K ? never : K;

/** `t()` with the key and the named params type-checked against en.json */
export type StrictTranslate = <K extends MessageKey>(
  key: K & ExactKey<K>,
  ...args: MessageArgs<K>
) => string;

type StrictComposer = Omit<Composer, 't'> & { t: StrictTranslate };

/** vue-i18n's `useI18n()`, with a `t()` that type-checks keys and params. */
export function useI18n(): StrictComposer {
  return useVueI18n() as unknown as StrictComposer;
}

const tagOf = (id: string) => locales[id]?.tag ?? id;
const idByTag = (tag: string) => Object.keys(locales).find((id) => tagOf(id) === tag);

// Converts a locale id to its BCP 47 tag (for the html lang, moment, OnlyOffice)
export const toStandardLocale = tagOf;

export function detectLocale(): string {
  const browserLocale = navigator.language.toLowerCase();
  const prefix = browserLocale.split('-')[0] ?? '';
  return (
    idByTag(browserLocale) ??
    idByTag(prefix) ??
    // 'zh' -> zhCN, 'sv' -> svSE: the first regional variant
    Object.keys(locales).find((id) => tagOf(id).startsWith(`${prefix}-`)) ??
    'en'
  );
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
const localeModules = import.meta.glob<{ default: MessageSchema }>(
  ['./locales/*.json', '!./locales/en.json'],
);

export async function setLocale(locale: string) {
  const file = locales[locale]?.file;
  // If the locale doesn't exist in our list, fallback to English
  if (!file) {
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
    const loadMessages = localeModules[`./locales/${file}.json`];
    if (!loadMessages) {
      setLanguage('en');
      return;
    }
    const messages = (await loadMessages()).default;
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

// The default export exposes the strict `t()`: `i18n.global.t(key, params)`
export default i18n as unknown as Omit<typeof i18n, 'global'> & {
  global: Omit<typeof i18n.global, 't'> & { t: StrictTranslate };
};
