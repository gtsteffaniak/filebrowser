import fs from "node:fs";
import path from "node:path";
import { DeepLClient, type TargetLanguageCode, type TranslateTextOptions } from "deepl-node";
import { localesDir, readJson, readLocale, translationFiles } from "./shared.ts";

type Tree = { [key: string]: unknown };

const MASTER_LANGUAGE = "en";

const DEEPL_LANGUAGES: Record<string, string> = {
  "zh-cn": "ZH-HANS",
  "zh-tw": "ZH-HANT",
  pt: "PT-PT",
  "pt-br": "PT-BR",
  en: "EN",
  "en-us": "EN-US",
  "en-gb": "EN-GB",
  "sv-se": "SV",
  ua: "UK",
  "nl-be": "NL",
  cz: "CS",
  bg: "BG",
};

let translator: DeepLClient | null = null;

const isTree = (value: unknown): value is Tree => typeof value === "object" && value !== null && !Array.isArray(value);

/** Reports what syncTranslations would change */
export function checkTranslations(): string[] {
  const master = Object.keys(readLocale("en.json"));
  const problems: string[] = [];
  const translatable = (key: string) => !key.startsWith("languages.");
  for (const file of translationFiles()) {
    let target: Record<string, string>;
    try {
      target = readLocale(file);
    } catch (error) {
      problems.push(`${file}: cannot be read (${error instanceof Error ? error.message : error})`);
      continue;
    }
    for (const key of master.filter(translatable)) {
      if (!target[key]) problems.push(`${file}: missing "${key}"`);
    }
    for (const key of Object.keys(target).filter(translatable)) {
      if (!master.includes(key)) problems.push(`${file}: obsolete key "${key}"`);
    }
  }
  return problems;
}

async function translateText(text: string, targetLanguage: string, keyPath: string): Promise<string | null> {
  if (!text.trim()) {
    console.warn(`Skipping translation for empty text: "${text}"`);
    return text;
  }
  if (keyPath === "languages" || keyPath.startsWith("languages.")) return text;

  const hasPlaceholders = /\{[^}]+\}/.test(text);
  let textToTranslate = text;
  const options: TranslateTextOptions = {};
  if (hasPlaceholders) {
    console.log(`Found placeholder in: "${text}". Wrapping for translation.`);
    textToTranslate = text.replace(/(\{[^}]+\})/g, "<ph>$1</ph>");
    options.tagHandling = "xml";
    options.ignoreTags = ["ph"];
  }

  try {
    console.log(`Translating "${text}" from '${MASTER_LANGUAGE}' to '${targetLanguage}'...`);
    const deeplTarget = (DEEPL_LANGUAGES[targetLanguage.toLowerCase()] ?? targetLanguage.toUpperCase()) as TargetLanguageCode;
    const result = await translator?.translateText(textToTranslate, MASTER_LANGUAGE, deeplTarget, options);
    await new Promise((resolve) => {setTimeout(resolve, 100)}); // avoid rate-limiting
    let translated = result?.text ?? "";
    if (hasPlaceholders) {
      // the translator might add spaces around the tags
      translated = translated.replace(/<ph>\s*(\{[^}]+\})\s*<\/ph>/g, "$1");
    }
    if (!translated.trim()) {
      console.error(`⚠️ Translation failed for "${text}" (${keyPath}): empty result`);
      return null;
    }
    return translated;
  } catch (error) {
    console.error(`⚠️ Translation failed for "${text}" (${keyPath}):`, error instanceof Error ? error.message : error);
    return null;
  }
}

/** Reorders `value` to match the key order of `master`, dropping keys `master` lacks */
function reorderObject(value: unknown, master: unknown): unknown {
  if (!isTree(value) || !isTree(master)) return value;
  const ordered: Tree = {};
  for (const key of Object.keys(master)) {
    if (Object.hasOwn(value, key)) ordered[key] = reorderObject(value[key], master[key]);
  }
  return ordered;
}

/** Add missing keys, remove obsolete ones */
async function processKeys(
  master: Tree,
  target: Tree,
  lang: string,
  parents: string[],
  enforceOrder: boolean,
  cleanup: boolean,
): Promise<boolean | "UNSUPPORTED"> {
  let changed = false;

  for (const key of Object.keys(master)) {
    const masterValue = master[key];
    const keyPath = [...parents, key].join(".");

    // the "languages" block is always copied from master
    if (key === "languages" && parents.length === 0 && !cleanup) {
      if (!enforceOrder) console.log(`Copying entire "languages" object from master to ${lang}.json`);
      target[key] = structuredClone(masterValue);
      changed = true;
      continue;
    }
    if (Object.hasOwn(target, key) && isTree(masterValue) !== isTree(target[key])) {
      console.log(`🗑️  Removing "${keyPath}" from ${lang}.json: its structure differs from en.json`);
      Reflect.deleteProperty(target, key);
      changed = true;
    }
    if (isTree(masterValue)) {
      if (!cleanup && !isTree(target[key])) {
        console.log(`Creating missing object structure for "${keyPath}" in ${lang}.json`);
        target[key] = {};
        changed = true;
      }
      const child = target[key];
      if (isTree(child)) {
        const result = await processKeys(masterValue, child, lang, [...parents, key], enforceOrder, cleanup);
        if (result === "UNSUPPORTED") {
          console.log(`Skipping translation for "${lang}" due to unsupported structure.`);
          return result;
        }
        if (result) changed = true;
      }
    } else if (!cleanup) {
      if (typeof masterValue === "string") {
        const existing = target[key];
        if (!Object.hasOwn(target, key) || existing === "" || existing === null) {
          if (enforceOrder) {
            console.log(`Skipping new key "${keyPath}" for ${lang}.json`);
          } else {
            const translated = await translateText(masterValue, lang, keyPath);
            if (translated === null) return "UNSUPPORTED";
            if (target[key] !== translated) {
              target[key] = translated;
              changed = true;
            }
          }
        }
      } else if (!Object.hasOwn(target, key)) {
        console.log(`Key "${keyPath}" (non-string) missing in ${lang}.json. Copying from English.`);
        target[key] = masterValue;
        changed = true;
      }
    }
  }
  for (const key of Object.keys(target)) {
    if (!Object.hasOwn(master, key)) {
      console.log(`🗑️  Removing obsolete key "${[...parents, key].join(".")}" from ${lang}.json`);
      Reflect.deleteProperty(target, key);
      changed = true;
    }
  }
  return changed;
}

export async function syncTranslations({ enforceOrder, cleanup }: { enforceOrder: boolean; cleanup: boolean }): Promise<number> {
  const reorder = enforceOrder && !cleanup;

  if (!enforceOrder && !cleanup) {
    const apiKey = process.env.DEEPL_API_KEY;
    if (!apiKey) {
      console.error("❌ Missing DEEPL_API_KEY in environment.");
      return 1;
    }
    translator = new DeepLClient(apiKey);
    console.warn("--- Using DeepL API for translation ---");
  } else if (cleanup) {
    console.warn("--- Only obsolete keys will be removed - No translations, or reordering will happen ---");
  } else {
    console.warn("--- Enforcing order of all files to match master language (no translations will be performed) ---");
  }

  const master = readJson(path.join(localesDir, "en.json")) as Tree;
  const failed: string[] = [];

  for (const file of translationFiles()) {
    const targetFile = path.join(localesDir, file);
    const lang = path.basename(file, ".json");
    let original: Tree;
    try {
      const parsed = readJson(targetFile);
      if (!isTree(parsed)) throw new Error("the root must be an object");
      original = parsed;
    } catch (error) {
      console.error(`❌ Could not read ${targetFile}, leaving it unchanged: ${error instanceof Error ? error.message : error}`);
      failed.push(file);
      continue;
    }
    let target = structuredClone(original);

    const result = await processKeys(master, target, lang, [], reorder, cleanup);
    if (result === "UNSUPPORTED") {
      console.error(`❌ ${targetFile} was left unchanged: a translation failed.`);
      failed.push(file);
      continue;
    }

    let orderChanged = false;
    if (reorder) {
      target = reorderObject(target, master) as Tree;
      orderChanged = JSON.stringify(original) !== JSON.stringify(target);
      if (orderChanged) console.log(`- Reordered keys in ${lang}.json`);
    }

    if (result || orderChanged) {
      fs.writeFileSync(targetFile, `${JSON.stringify(target, null, 2)}\n`);
      if (!cleanup && !reorder) console.log(`Successfully updated ${targetFile}`);
    } else {
      console.log(`No changes needed for ${targetFile}`);
    }
  }
  if (failed.length) {
    console.error(`\n❌ Synchronization incomplete, ${failed.length} locale(s) were not changed: ${failed.join(", ")}`);
    return 1;
  }

  if (cleanup) console.log("\n✅ Cleanup complete (obsolete keys removed).");
  else if (reorder) console.log("\n✅ Enforce order complete.");
  else console.log("\n✅ Translation synchronization complete (via DeepL).");
  return 0;
}
