// scripts/sync-translations.js
import fs from 'fs-extra';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as glob from 'glob';
import * as deepl from 'deepl-node';

// Parse command line arguments
const args = process.argv.slice(2);
const checkOnly = args.includes('--check') || args.includes('-c');
const enforceOrder = args.includes('--enforce-order'); // When using this will force the order of eng, will not perform new translations. If we move keys to a different block will count as new translations and will delete it from all the languages
const cleanupOnly = args.includes('--cleanup'); // Will only delete keys that not longer exist in eng, no translation or reorder, just deletion.

// --- Configuration ---
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const localesDir = path.resolve(__dirname, '../src/i18n');
const masterLocaleFile = path.join(localesDir, 'en.json');
const masterLanguageCode = 'en';
const targetLocaleFiles = glob.sync(path.join(localesDir, '*.json'))
  .filter(file => path.basename(file) !== `${masterLanguageCode}.json`)
  .filter(file => path.basename(file) !== 'is.json'); // Exclude Icelandic - DeepL doesn't support it

const requireApiKey = !checkOnly && !enforceOrder && !cleanupOnly;
const DEEPL_API_KEY = process.env.DEEPL_API_KEY;
if (requireApiKey && !DEEPL_API_KEY) {
  console.error("❌ Missing DEEPL_API_KEY in environment.");
  process.exit(1);
}

const translator = requireApiKey ? new deepl.Translator(DEEPL_API_KEY) : null;

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

// Reorder objects to match eng (master lang) key order
function reorderObject(obj, masterObj) {
  // If obj is not an object, return as is
  if (typeof obj !== 'object' || obj === null || Array.isArray(obj)) return obj;
  const newObj = {};
  for (const key in masterObj) {
    if (Object.prototype.hasOwnProperty.call(masterObj, key)) {
      if (Object.prototype.hasOwnProperty.call(obj, key)) {
        newObj[key] = reorderObject(obj[key], masterObj[key]);
      }
    }
  }
  return newObj;
}

// --- Recursive key processor ---
async function processKeys(masterObj, targetObj, targetLangCode, currentPathParts = [], doEnforceOrder, doCleanupOnly = false) {
  let changesMade = false;
  let meaningfulChanges = 0; // Only count meaningful changes

  // First pass: Add/update keys from master to target
  for (const key in masterObj) {
    if (Object.prototype.hasOwnProperty.call(masterObj, key)) {
      const currentPathPartsNext = [...currentPathParts, key];
      const currentKeyPath = currentPathPartsNext.join('.');

      const masterValue = masterObj[key];

      // Special handling for "languages" key - always copy the entire object from master
      if (key === 'languages' && currentPathParts.length === 0 && !doCleanupOnly) {
        if (!checkOnly && !doEnforceOrder) {
          console.log(`Copying entire "languages" object from master to ${targetLangCode}.json`);
        }
        targetObj[key] = JSON.parse(JSON.stringify(masterValue)); // Deep copy
        changesMade = true;
        continue;
      }

      if (typeof masterValue === 'object' && masterValue !== null && !Array.isArray(masterValue)) {
        if (!doCleanupOnly && (!targetObj[key] || typeof targetObj[key] !== 'object')) {
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

        if (targetObj[key] && typeof targetObj[key] === 'object') {
          const result = await processKeys( masterValue, targetObj[key], targetLangCode, currentPathPartsNext,
            doEnforceOrder, doCleanupOnly);
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
        }
      } else if (!doCleanupOnly) {
        // Only handle non‑object values
        if (typeof masterValue === 'string') {
          const existingTargetValue = targetObj[key];
          const isMissing = !Object.prototype.hasOwnProperty.call(targetObj, key) || existingTargetValue === '' || existingTargetValue === null;

          if (isMissing) {
            if (checkOnly) {
              console.log(`Would translate "${currentKeyPath}" for ${targetLangCode}.json`);
              meaningfulChanges++; // Translation is meaningful
            } else if (doEnforceOrder) {
              // Skip translations for new keys
              console.log(`Skipping new key "${currentKeyPath}" for ${targetLangCode}.json`);
            } else {
              // Sync translation
              const result = await translateText(masterValue, targetLangCode, currentKeyPath);
              if (result == "") {
                return "UNSUPPORTED";
              }
              targetObj[key] = result;
              changesMade = true;
            }
          }
        } else {
          if (!Object.prototype.hasOwnProperty.call(targetObj, key)) {
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
  }

  // Second pass: Remove obsolete keys that exist in target but not in master
  const keysToRemove = [];
  for (const key in targetObj) {
    if (Object.prototype.hasOwnProperty.call(targetObj, key)) {
      if (!Object.prototype.hasOwnProperty.call(masterObj, key)) {
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
  const isSync = !checkOnly && !enforceOrder && !cleanupOnly;
  const isEnforceOrder = enforceOrder && !cleanupOnly;
  const isCleanup = cleanupOnly;

  if (checkOnly) {
    console.log("--- Checking for translation changes (no translations will be performed) ---");
  } else if (isCleanup) {
    console.warn("--- Only obsolete keys will be removed - No translations, or reordering will happen ---");
  } else if (isEnforceOrder) {
    console.warn("--- Enforcing order of all files to match master language (no translations will be performed) ---");
  } else if (isSync) {
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
    let originalTargetContent = null;
    let fileExisted = await fs.pathExists(targetFile);

    if (fileExisted) {
      try {
        originalTargetContent = await fs.readJson(targetFile);
        targetContent = JSON.parse(JSON.stringify(originalTargetContent)); // start from original
      } catch (e) {
        console.warn(`Warning: Could not parse ${targetFile}. Starting fresh. Error: ${e.message}`);
        targetContent = {};
        originalTargetContent = null;
      }
    } else {
      if (isCleanup) continue;
      console.log(`\nTarget file ${targetFile} not found. Will create for language: ${targetLangCode}.`);
    }

    const result = await processKeys(masterContent, targetContent, targetLangCode, [], isEnforceOrder, isCleanup);

    let reorderedContent = targetContent;
    let orderChanged = false;
    if (isEnforceOrder && originalTargetContent && !checkOnly) {
      reorderedContent = reorderObject(targetContent, masterContent);
      // Compare the reordered content with the original to see if order changed
      const origStr = JSON.stringify(originalTargetContent);
      const reorderedStr = JSON.stringify(reorderedContent);
      if (origStr !== reorderedStr) {
        orderChanged = true;
        console.log(`- Reordered keys in ${targetLangCode}.json`);
      }
    }

    if (checkOnly) {
      let wouldChange = false;
      if (typeof result === 'number' && result > 0) {
        meaningfulChanges += result;
        wouldChange = true;
        console.log(`Found ${result} meaningful changes needed for ${targetLangCode}.json`);
      }
      if (!fileExisted && !isCleanup) {
        wouldChange = true;
        console.log(`Would create new file for ${targetLangCode}.json`);
      }
      if (wouldChange) {
        hasMeaningfulChanges = true;
      }
    } else {
      const needsWrite = result || orderChanged || (!isCleanup && !fileExisted);
      if (needsWrite) {
        const finalContent = (isEnforceOrder && orderChanged) ? reorderedContent : targetContent;
        try {
          await fs.writeJson(targetFile, finalContent, { spaces: 2 });
          if (!isCleanup && !isEnforceOrder) {
            console.log(`Successfully ${result ? 'updated' : 'created'} ${targetFile}`);
          }
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
      console.log(`\n⚠️  Found ${meaningfulChanges} meaningful translation changes needed. -- update via "make sync-translations"`);
      return 1; // Exit code 1 for meaningful changes needed
    } else {
      console.log('\n✅ No meaningful translation changes needed.');
      return 0; // Exit code 0 for no meaningful changes
    }
  } else {
    if (isCleanup) {
      console.log('\n✅ Cleanup complete (obsolete keys removed).');
    } else if (isEnforceOrder) {
      console.log('\n✅ Enforce order complete.');
    } else {
      console.log('\n✅ Translation synchronization complete (via DeepL).');
    }
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