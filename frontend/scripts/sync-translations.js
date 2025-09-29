// scripts/sync-translations.js
import fs from 'fs-extra';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as glob from 'glob';
import * as deepl from 'deepl-node';

// Parse command line arguments
const args = process.argv.slice(2);
const checkOnly = args.includes('--check') || args.includes('-c');

// --- Configuration ---
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const localesDir = path.resolve(__dirname, '../src/i18n');
const masterLocaleFile = path.join(localesDir, 'en.json');
const masterLanguageCode = 'en';
const targetLocaleFiles = glob.sync(path.join(localesDir, '*.json'))
  .filter(file => path.basename(file) !== `${masterLanguageCode}.json`)
  .filter(file => path.basename(file) !== 'is.json'); // Exclude Icelandic - DeepL doesn't support it

const DEEPL_API_KEY = process.env.DEEPL_API_KEY;
if (!checkOnly && !DEEPL_API_KEY) {
  console.error("❌ Missing DEEPL_API_KEY in environment.");
  process.exit(1);
}

const translator = checkOnly ? null : new deepl.Translator(DEEPL_API_KEY);

const deeplLangMap = {
  'zh-cn': 'ZH-HANS',
  'zh-tw': 'ZH-HANT',
  'pt': 'PT-PT',      // or 'PT-BR' if you want Brazilian Portuguese
  'pt-br': 'PT-BR',
  'en': 'EN',
  'en-us': 'EN-US',
  'en-gb': 'EN-GB',
  'sv-se': 'SV',
  'ua': 'UK',
  'nl-be': 'NL',
  'is': 'IS',
  'cz': 'CS',
  // Add more as needed
};

// --- Translation Function using DeepL ---
async function translateText(text, targetLanguage, keyPath = '') {
  if (checkOnly) {
    // In check mode, just return the original text to detect changes
    return text;
  }

  if (!text || typeof text !== 'string' || text.trim() === '') {
    console.warn(`Skipping translation for empty or non-string text: "${text}"`);
    return text;
  }

  if (keyPath === 'languages' || keyPath.startsWith('languages.')) {
    return text;
  }

  const hasPlaceholders = /\{[^}]+\}/.test(text);
  let textToTranslate = text;
  const options = {};

  if (hasPlaceholders) {
    console.log(`Found placeholder in: "${text}". Wrapping for translation.`);
    textToTranslate = text.replace(/(\{[^}]+\})/g, '<ph>$1</ph>');
    options.tagHandling = 'xml';
    options.ignoreTags = ['ph'];
  }

  try {
    console.log(`Translating "${text}" from '${masterLanguageCode}' to '${targetLanguage}'...`);

    let deeplTargetLang = deeplLangMap[targetLanguage.toLowerCase()] || targetLanguage.toUpperCase();

    const result = await translator.translateText(textToTranslate, masterLanguageCode, deeplTargetLang, options);

    // Delay to avoid rate-limiting
    await new Promise(resolve => setTimeout(resolve, 100));

    let translatedText = result.text;

    if (hasPlaceholders) {
      // Unwrap the placeholders. The translator might add spaces around tags.
      translatedText = translatedText.replace(/<ph>\s*(\{[^}]+\})\s*<\/ph>/g, '$1');
    }

    return translatedText;

  } catch (err) {
    console.error(`⚠️ Translation failed for "${text}" (${keyPath}):`, err?.message || err);
    return ``;
  }
}

// --- Recursive key processor ---
async function processKeys(masterObj, targetObj, targetLangCode, currentPathParts = []) {
  let changesMade = false;
  let meaningfulChanges = 0; // Only count meaningful changes

  // First pass: Add/update keys from master to target
  for (const key in masterObj) {
    if (Object.prototype.hasOwnProperty.call(masterObj, key)) {
      const currentPathPartsNext = [...currentPathParts, key];
      const currentKeyPath = currentPathPartsNext.join('.');

      const masterValue = masterObj[key];

      // Special handling for "languages" key - always copy the entire object from master
      if (key === 'languages' && currentPathParts.length === 0) {
        if (checkOnly) {
          // Don't print or count languages object copying - it's routine
        } else {
          console.log(`Copying entire "languages" object from master to ${targetLangCode}.json`);
          targetObj[key] = JSON.parse(JSON.stringify(masterValue)); // Deep copy
        }
        changesMade = true;
        continue;
      }

      if (typeof masterValue === 'object' && masterValue !== null && !Array.isArray(masterValue)) {
        if (!targetObj[key] || typeof targetObj[key] !== 'object') {
          if (checkOnly) {
            console.log(`Would create missing object structure for "${currentKeyPath}" in ${targetLangCode}.json`);
            meaningfulChanges++; // Creating new object structures is meaningful
            // In check mode, create a temporary object for processing
            targetObj[key] = {};
          } else {
            console.log(`Creating missing object structure for "${currentKeyPath}" in ${targetLangCode}.json`);
            targetObj[key] = {};
          }
          changesMade = true;
        }
        const result = await processKeys(masterValue, targetObj[key], targetLangCode, currentPathPartsNext);
        if (result == "UNSUPPORTED") {
            console.log(`Skipping translation for "${targetLangCode}" due to unsupported structure.`);
            return "UNSUPPORTED";
        }
        if (typeof result === 'number') {
          meaningfulChanges += result;
          changesMade = true;
        } else if (result) {
          changesMade = true;
        }
      } else if (typeof masterValue === 'string') {
        if (!targetObj.hasOwnProperty(key) || targetObj[key] === '' || targetObj[key] === null) {
          if (checkOnly) {
            console.log(`Would translate "${currentKeyPath}" for ${targetLangCode}.json`);
            meaningfulChanges++; // Translation is meaningful
          } else {
            const result = await translateText(masterValue, targetLangCode, currentKeyPath);
            if (result == "") {
              return "UNSUPPORTED";
            }
            targetObj[key] = result;
          }
          changesMade = true;
        }
      } else {
        if (!targetObj.hasOwnProperty(key)) {
          if (checkOnly) {
            console.log(`Would copy key "${currentKeyPath}" (non-string) from English to ${targetLangCode}.json`);
            meaningfulChanges++; // Copying non-string values is meaningful
          } else {
            console.log(`Key "${currentKeyPath}" (non-string) missing in ${targetLangCode}.json. Copying from English.`);
            targetObj[key] = masterValue;
          }
          changesMade = true;
        }
      }
    }
  }

  // Second pass: Remove obsolete keys that exist in target but not in master
  const keysToRemove = [];
  for (const key in targetObj) {
    if (Object.prototype.hasOwnProperty.call(targetObj, key)) {
      if (!masterObj.hasOwnProperty(key)) {
        keysToRemove.push(key);
      }
    }
  }

  for (const key of keysToRemove) {
    const currentKeyPath = [...currentPathParts, key].join('.');
    if (checkOnly) {
      console.log(`Would remove obsolete key "${currentKeyPath}" from ${targetLangCode}.json`);
      meaningfulChanges++; // Removing obsolete keys is meaningful
    } else {
      console.log(`🗑️  Removing obsolete key "${currentKeyPath}" from ${targetLangCode}.json`);
      delete targetObj[key];
    }
    changesMade = true;
  }

  return checkOnly ? meaningfulChanges : changesMade;
}

// --- Main synchronization ---
async function syncAllTranslations() {
  if (checkOnly) {
    console.log("--- Checking for translation changes (no translations will be performed) ---");
  } else {
    console.warn("--- Using DeepL API for translation ---");
  }

  if (!await fs.pathExists(masterLocaleFile)) {
    console.error(`Master locale file not found: ${masterLocaleFile}`);
    process.exit(1);
  }

  const masterContent = await fs.readJson(masterLocaleFile);
  console.log(`Loaded master translations from ${masterLocaleFile}`);

  let meaningfulChanges = 0;
  let hasMeaningfulChanges = false;

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

    const result = await processKeys(masterContent, targetContent, targetLangCode);

    if (checkOnly) {
      if (typeof result === 'number' && result > 0) {
        meaningfulChanges += result;
        hasMeaningfulChanges = true;
        console.log(`Found ${result} meaningful changes needed for ${targetLangCode}.json`);
      } else if (!fileExisted) {
        meaningfulChanges++;
        hasMeaningfulChanges = true;
        console.log(`Would create new file for ${targetLangCode}.json`);
      } else {
        // Only print if there are meaningful changes
      }
    } else {
      if (result || !fileExisted) {
        try {
          await fs.writeJson(targetFile, targetContent, { spaces: 2 });
          console.log(`Successfully ${result ? 'updated' : 'created'} ${targetFile}`);
        } catch (error) {
          console.error(`Error writing to ${targetFile}:`, error);
        }
      } else {
        console.log(`No changes needed for ${targetFile}`);
      }
    }
  }

  if (checkOnly) {
    if (hasMeaningfulChanges) {
      console.log(`\n⚠️  Found ${meaningfulChanges} meaningful translation changes needed.`);
      return 1; // Exit code 1 for meaningful changes needed
    } else {
      console.log('\n✅ No meaningful translation changes needed.');
      return 0; // Exit code 0 for no meaningful changes
    }
  } else {
    console.log('\n✅ Translation synchronization complete (via DeepL).');
    return 0;
  }
}

syncAllTranslations()
  .then(exitCode => {
    process.exit(exitCode);
  })
  .catch(error => {
    console.error("\n❌ An error occurred during translation synchronization:", error);
    process.exit(1);
  });