/**
 *   npm run i18n:types           regenerate messages.generated.ts
 *   npm run i18n:sync            regenerate the types, translate new keys (DEEPL_API_KEY)
 *   npm run i18n:enforce-order   regenerate the types, reorder locales like en.json
 *   npm run i18n:cleanup         regenerate the types, remove obsolete keys
 *   npm run i18n:check           translations, types, unused keys and raw text
 *   npm run i18n:check:strict    checks for raw `$t` and 'vue-i18n' imports, including JS files
 */

import { checkRawText, checkUnusedKeys } from "./check.ts";
import { report } from "./shared.ts";
import { checkTranslations, syncTranslations } from "./sync.ts";
import { checkTypes, writeTypes } from "./types.ts";

const [command, ...flags] = process.argv.slice(2);
const has = (flag: string) => flags.includes(flag);

try {
  if (command === "types") {
    process.exitCode = writeTypes();
  } else if (command === "sync") {
    process.exitCode = writeTypes() || (await syncTranslations({ enforceOrder: has("--enforce-order"), cleanup: has("--cleanup") }));
  } else if (command === "check") {
    const translations = checkTranslations();
    const problems = [...translations, ...checkTypes(has("--strict")), ...checkUnusedKeys(), ...checkRawText()];
    if (problems.length) {
      report(problems);
      if (translations.length) console.error("Run: npm run i18n:sync (or i18n:cleanup)");
      process.exitCode = 1;
    } else {
      console.log("✅ i18n checks passed");
    }
  } else {
    console.error("Usage: node scripts/i18n/index.ts <types | sync [--enforce-order | --cleanup] | check [--strict]>");
    process.exitCode = 1;
  }
} catch (error) {
  console.error("\n❌ i18n script failed:", error);
  process.exitCode = 1;
}
