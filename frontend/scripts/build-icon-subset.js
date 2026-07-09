// scripts/build-icon-subset.js
import { spawnSync } from 'node:child_process';
import fs from 'fs-extra';
import * as glob from 'glob';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..');
const srcDir = path.join(frontendRoot, 'src');
const inventoryPath = path.join(srcDir, 'utils/icon-inventory-core.json');
const outlinedFontPath = path.join(frontendRoot, 'public/fonts/material-symbols-core-outlined.woff2');
const filledFontPath = path.join(frontendRoot, 'public/fonts/material-symbols-core-filled.woff2');
const venvPython = path.join(frontendRoot, '.venv-icons/bin/python');

const args = process.argv.slice(2);
const checkOnly = args.includes('--check') || args.includes('-c');

const ICON_NAME = '[a-z][a-z0-9_]*';
const ICON_NAME_RE = new RegExp(`^${ICON_NAME}$`);

const ICON_MAP_FILES = [
  'views/bars/Default.vue',
  'views/files/ListingView.vue',
];

/** Icons referenced dynamically (not visible to template scan). */
const EXTRA_ICONS = [
  'arrow_downward',
  'arrow_upward',
  'autorenew',
  'build',
  'check',
  'cloud_off',
  'copy_all',
  'create_new_folder',
  'do_not_disturb_on',
  'done',
  'error_outline',
  'file',
  'file_copy',
  'forward',
  'forward_10',
  'grid_view',
  'gps_off',
  'insert_drive_file',
  'ios_share',
  'keyboard_arrow_up',
  'lock',
  'lock_open',
  'more_vert',
  'movie',
  'note_add',
  'photo',
  'progress_activity',
  'qr_code',
  'replay_10',
  'repeat',
  'repeat_one',
  'select_all',
  'table_rows_narrow',
  'unarchive',
  'view_list',
  'view_module',
  'volume_up',
  'download',
];

function addIcon(icons, value) {
  if (typeof value !== 'string') return;
  const name = value.trim();
  if (!name || !ICON_NAME_RE.test(name)) return;
  icons.add(name);
}

function scanVueAndTemplates(icons) {
  const files = glob.sync(path.join(srcDir, '**/*.vue'));
  const materialTag = new RegExp(
    `material-symbols(?:-outlined)?[^>]*>\\s*(${ICON_NAME})\\s*<`,
    'g',
  );
  const materialInHtml = new RegExp(
    `material-symbols(?:-outlined)?[^>]*>(${ICON_NAME})<`,
    'g',
  );
  const iconProp = new RegExp(`\\bicon="(${ICON_NAME})"`, 'g');
  const ternaryIcons = new RegExp(
    `\\?\\s*["'](${ICON_NAME})["']\\s*:\\s*["'](${ICON_NAME})["']`,
    'g',
  );

  for (const file of files) {
    const content = fs.readFileSync(file, 'utf8');
    for (const match of content.matchAll(materialTag)) addIcon(icons, match[1]);
    for (const match of content.matchAll(materialInHtml)) addIcon(icons, match[1]);
    for (const match of content.matchAll(iconProp)) addIcon(icons, match[1]);
    for (const match of content.matchAll(ternaryIcons)) {
      addIcon(icons, match[1]);
      addIcon(icons, match[2]);
    }
  }
}

function scanIconMapLiterals(icons) {
  for (const rel of ICON_MAP_FILES) {
    const content = fs.readFileSync(path.join(srcDir, rel), 'utf8');
    const block = content.match(/const icons = \{([^}]+)\}/s);
    if (!block) continue;
    for (const match of block[1].matchAll(/["']([a-z][a-z0-9_]*)["']/g)) {
      addIcon(icons, match[1]);
    }
  }
}

function scanMimetype(icons) {
  const content = fs.readFileSync(path.join(srcDir, 'utils/mimetype.js'), 'utf8');
  const materialSymbol = new RegExp(`materialSymbol:\\s*["'](${ICON_NAME})["']`, 'g');
  for (const match of content.matchAll(materialSymbol)) addIcon(icons, match[1]);
}

function scanConstants(icons) {
  const content = fs.readFileSync(path.join(srcDir, 'utils/constants.js'), 'utf8');
  const iconField = new RegExp(`\\bicon:\\s*["'](${ICON_NAME})["']`, 'g');
  for (const match of content.matchAll(iconField)) addIcon(icons, match[1]);
}

function scanPlaybackQueue(icons) {
  const content = fs.readFileSync(path.join(srcDir, 'utils/playbackQueue.js'), 'utf8');
  const switchReturn = new RegExp(`return\\s+['"](${ICON_NAME})['"]`, 'g');
  for (const match of content.matchAll(switchReturn)) addIcon(icons, match[1]);
}

function scanErrors(icons) {
  const content = fs.readFileSync(path.join(srcDir, 'views/Errors.vue'), 'utf8');
  const errorIcon = new RegExp(`\\bicon:\\s*["'](${ICON_NAME})["']`, 'g');
  for (const match of content.matchAll(errorIcon)) addIcon(icons, match[1]);
}

function scanSearchFilters(icons) {
  const content = fs.readFileSync(path.join(srcDir, 'components/Search.vue'), 'utf8');
  const filterIcon = new RegExp(`\\bicon:\\s*["'](${ICON_NAME})["']`, 'g');
  for (const match of content.matchAll(filterIcon)) addIcon(icons, match[1]);
}

function scanMarkdownViewer(icons) {
  const content = fs.readFileSync(path.join(srcDir, 'views/files/MarkdownViewer.vue'), 'utf8');
  const materialInHtml = new RegExp(
    `material-symbols(?:-outlined)?[^>]*>(${ICON_NAME})<`,
    'g',
  );
  for (const match of content.matchAll(materialInHtml)) addIcon(icons, match[1]);
}

function collectIcons() {
  const icons = new Set(EXTRA_ICONS);
  scanVueAndTemplates(icons);
  scanIconMapLiterals(icons);
  scanMimetype(icons);
  scanConstants(icons);
  scanPlaybackQueue(icons);
  scanErrors(icons);
  scanSearchFilters(icons);
  scanMarkdownViewer(icons);
  icons.delete('close_back');
  return [...icons].sort();
}

function readInventory() {
  if (!fs.existsSync(inventoryPath)) return null;
  return fs.readJsonSync(inventoryPath);
}

function writeInventory(icons) {
  fs.writeJsonSync(inventoryPath, icons, { spaces: 2 });
}

function inventoriesMatch(a, b) {
  if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
  return a.every((icon, index) => icon === b[index]);
}

function coreFontsExist() {
  return fs.existsSync(outlinedFontPath) && fs.existsSync(filledFontPath);
}

function resolvePython() {
  if (fs.existsSync(venvPython)) return venvPython;
  const candidates = ['python3', 'python'];
  for (const cmd of candidates) {
    const probe = spawnSync(cmd, ['-c', 'import fontTools'], { encoding: 'utf8' });
    if (probe.status === 0) return cmd;
  }
  return null;
}

async function fetchGoogleSubsetCss(iconNames, fill) {
  const family = `Material+Symbols+Outlined:opsz,wght,FILL,GRAD@24,400,${fill},0`;
  const iconNamesParam = iconNames.join(',');
  const url = `https://fonts.googleapis.com/css2?family=${family}&icon_names=${iconNamesParam}`;
  const response = await fetch(url, {
    headers: { 'User-Agent': 'Mozilla/5.0 (compatible; filebrowser-icon-build/1.0)' },
  });
  if (!response.ok) {
    throw new Error(`Google Fonts API returned ${response.status} for FILL=${fill} subset`);
  }
  return response.text();
}

function extractFontUrl(css) {
  const match = css.match(/src:\s*url\(([^)]+)\)\s*format\('truetype'\)/);
  if (!match) {
    throw new Error('Could not find font URL in Google Fonts CSS response');
  }
  return match[1];
}

async function downloadTtf(url) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to download subset font (${response.status})`);
  }
  return Buffer.from(await response.arrayBuffer());
}

function convertTtfToWoff2(ttfBuffer, outputPath) {
  const python = resolvePython();
  if (!python) {
    console.error('❌ fonttools not found. Run: python3 -m venv .venv-icons && .venv-icons/bin/pip install fonttools brotli');
    process.exit(1);
  }

  const tmpDir = fs.mkdtempSync(path.join(frontendRoot, '.tmp-icons-'));
  const ttfPath = path.join(tmpDir, 'subset.ttf');
  fs.writeFileSync(ttfPath, ttfBuffer);

  const result = spawnSync(
    python,
    ['-m', 'fontTools.ttLib.woff2', 'compress', ttfPath, '-o', outputPath],
    { encoding: 'utf8' },
  );

  fs.removeSync(tmpDir);

  if (result.status !== 0) {
    console.error(result.stderr || result.stdout || 'woff2 compression failed');
    process.exit(1);
  }
}

async function buildCoreFontVariant(icons, fill, outputPath) {
  const css = await fetchGoogleSubsetCss(icons, fill);
  const fontUrl = extractFontUrl(css);
  const ttfBuffer = await downloadTtf(fontUrl);
  convertTtfToWoff2(ttfBuffer, outputPath);
}

async function buildCoreFonts(icons) {
  await buildCoreFontVariant(icons, 0, outlinedFontPath);
  await buildCoreFontVariant(icons, 1, filledFontPath);
}

async function main() {
  const icons = collectIcons();
  const existing = readInventory();

  if (checkOnly) {
    if (!existing) {
      console.error('❌ Missing icon-inventory-core.json. Run: make sync-icons');
      process.exit(1);
    }
    if (!inventoriesMatch(existing, icons)) {
      const existingSet = new Set(existing);
      const missing = icons.filter((icon) => !existingSet.has(icon));
      const extra = existing.filter((icon) => !new Set(icons).has(icon));
      console.error('❌ Core icon inventory is out of date. Run: make sync-icons');
      if (missing.length) console.error(`   Missing from inventory (${missing.length}): ${missing.join(', ')}`);
      if (extra.length) console.error(`   Stale in inventory (${extra.length}): ${extra.join(', ')}`);
      process.exit(1);
    }
    console.log(`✅ Core icon inventory up to date (${icons.length} icons).`);
    process.exit(0);
  }

  writeInventory(icons);
  if (existing && inventoriesMatch(existing, icons) && coreFontsExist()) {
    const outlinedKb = Math.round(fs.statSync(outlinedFontPath).size / 1024);
    const filledKb = Math.round(fs.statSync(filledFontPath).size / 1024);
    console.log(`✅ Core icon inventory unchanged (${icons.length} icons, ${outlinedKb}+${filledKb} KB fonts)`);
    return;
  }

  await buildCoreFonts(icons);
  const outlinedKb = Math.round(fs.statSync(outlinedFontPath).size / 1024);
  const filledKb = Math.round(fs.statSync(filledFontPath).size / 1024);
  console.log(`✅ Generated ${icons.length} core icons → outlined ${outlinedKb} KB + filled ${filledKb} KB`);
}

main().catch((error) => {
  console.error('❌ Icon subset build failed:', error);
  process.exit(1);
});
