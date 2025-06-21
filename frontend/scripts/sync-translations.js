// scripts/sync-translations.js
import fs from 'fs-extra';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as glob from 'glob';
import * as deepl from 'deepl-node';

// --- Configuration ---
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const localesDir = path.resolve(__dirname, '../src/i18n');
const masterLocaleFile = path.join(localesDir, 'en.json');
const masterLanguageCode = 'en';
const targetLocaleFiles = glob.sync(path.join(localesDir, '*.json'))
  .filter(file => path.basename(file) !== `${masterLanguageCode}.json`);

const DEEPL_API_KEY = process.env.DEEPL_API_KEY;
if (!DEEPL_API_KEY) {
  console.error("❌ Missing DEEPL_API_KEY in environment.");
  process.exit(1);
}

const translator = new deepl.Translator(DEEPL_API_KEY);

// --- Translation Function using DeepL ---
async function translateText(text, targetLanguage, keyPath = '') {
  if (!text || typeof text !== 'string' || text.trim() === '') {
    console.warn(`Skipping translation for empty or non-string text: "${text}"`);
    return text;
  }

  if (keyPath === 'languages' || keyPath.startsWith('languages.')) {
    return text;
  }

  if (/\{[^}]+\}/.test(text)) {
    console.log(`Skipping translation for placeholder string: "${text}"`);
    return text;
  }

  try {
    console.log(`Translating "${text}" from '${masterLanguageCode}' to '${targetLanguage}'...`);

    const result = await translator.translateText(text, masterLanguageCode, targetLanguage.toUpperCase());

    // Delay to avoid rate-limiting
    await new Promise(resolve => setTimeout(resolve, 100));

    return result.text;

  } catch (err) {
    console.error(`⚠️ Translation failed for "${text}" (${keyPath}):`, err?.message || err);
    return ``;
  }
}

// --- Recursive key processor ---
async function processKeys(masterObj, targetObj, targetLangCode, currentPathParts = []) {
  let changesMade = false;
  for (const key in masterObj) {
    if (Object.prototype.hasOwnProperty.call(masterObj, key)) {
      const currentPathPartsNext = [...currentPathParts, key];
      const currentKeyPath = currentPathPartsNext.join('.');
      if (currentPathParts[0] === 'languages') continue;

      const masterValue = masterObj[key];

      if (typeof masterValue === 'object' && masterValue !== null && !Array.isArray(masterValue)) {
        if (!targetObj[key] || typeof targetObj[key] !== 'object') {
          console.log(`Creating missing object structure for "${currentKeyPath}" in ${targetLangCode}.json`);
          targetObj[key] = {};
          changesMade = true;
        }
        const result = await processKeys(masterValue, targetObj[key], targetLangCode, currentPathPartsNext);
        if (result == "UNSUPPORTED") {
            console.log(`Skipping translation for "${targetLangCode}" due to unsupported structure.`);
            return "UNSUPPORTED";
        }
        if (result) {
          changesMade = true;
        }
      } else if (typeof masterValue === 'string') {
        if (!targetObj.hasOwnProperty(key) || targetObj[key] === '' || targetObj[key] === null) {
          const result = await translateText(masterValue, targetLangCode, currentKeyPath);
          if (result == "") {
            return "UNSUPPORTED";
          }
          targetObj[key] = result;
          changesMade = true;
        }
      } else {
        if (!targetObj.hasOwnProperty(key)) {
          console.log(`Key "${currentKeyPath}" (non-string) missing in ${targetLangCode}.json. Copying from English.`);
          targetObj[key] = masterValue;
          changesMade = true;
        }
      }
    }
  }
  return changesMade;
}

// --- Main synchronization ---
async function syncAllTranslations() {
  console.warn("--- Using DeepL API for translation ---");

  if (!await fs.pathExists(masterLocaleFile)) {
    console.error(`Master locale file not found: ${masterLocaleFile}`);
    process.exit(1);
  }

  const masterContent = await fs.readJson(masterLocaleFile);
  console.log(`Loaded master translations from ${masterLocaleFile}`);

  for (const targetFile of targetLocaleFiles) {
    const targetLangCode = path.basename(targetFile, '.json');
    let targetContent = {};
    let fileExisted = await fs.pathExists(targetFile);

    if (fileExisted) {
      try {
        targetContent = await fs.readJson(targetFile);
        console.log(`\nProcessing target language: ${targetLangCode} (from ${targetFile})`);
      } catch (e) {
        console.warn(`Warning: Could not parse ${targetFile}. Starting fresh. Error: ${e.message}`);
        targetContent = {};
      }
    } else {
      console.log(`\nTarget file ${targetFile} not found. Will create for language: ${targetLangCode}.`);
    }

    const wasUpdated = await processKeys(masterContent, targetContent, targetLangCode);

    if (wasUpdated || !fileExisted) {
      try {
        await fs.writeJson(targetFile, targetContent, { spaces: 2 });
        console.log(`Successfully ${wasUpdated ? 'updated' : 'created'} ${targetFile}`);
      } catch (error) {
        console.error(`Error writing to ${targetFile}:`, error);
      }
    } else {
      console.log(`No changes needed for ${targetFile}`);
    }
  }

  console.log('\n✅ Translation synchronization complete (via DeepL).');
}

syncAllTranslations().catch(error => {
  console.error("\n❌ An error occurred during translation synchronization:", error);
  process.exit(1);
});