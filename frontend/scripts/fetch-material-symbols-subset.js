#!/usr/bin/env node
/**
 * Re-downloads Material Symbols Outlined as a glyph subset from Google Fonts.
 *
 * HarfBuzz-based subsetters still perform full GSUB closure on this variable font (~3.8MB).
 * Google’s CSS API with `text=` returns a compact VF (~280KB) when `text` is the set of
 * characters that appear in every supported icon name (Material names use [a-z0-9_] only).
 *
 * Requires network:
 *   npm run fonts:subset
 *
 * Audit: validates that every icon name matches /^[a-z0-9_]+$/, flags duplicate entries in
 * material-symbols.js, and sanity-checks output size. It does not parse the binary font to
 * prove each ligature exists (Google’s pipeline is the source of truth once the charset matches).
 *
 * License: SIL Open Font License (same family as Material Symbols).
 */

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { materialSymbols } from "../src/utils/material-symbols.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "..");

/** Material symbol glyph names use only these characters; required for `text=` subsetting. */
const ICON_NAME_RE = /^[a-z0-9_]+$/;

/** Chrome UA so Google serves `format('woff2')` in the CSS. */
const CHROME_UA =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";

/** Reject obviously broken downloads (empty/error page, wrong asset). */
const MIN_SUBSET_BYTES = 80_000;
const MAX_SUBSET_BYTES = 2_000_000;

function auditMaterialSymbolsList() {
  const seen = new Set();
  for (let i = 0; i < materialSymbols.length; i++) {
    const name = materialSymbols[i];
    if (typeof name !== "string") {
      throw new Error(`material-symbols.js: entry at index ${i} is not a string`);
    }
    if (!ICON_NAME_RE.test(name)) {
      throw new Error(
        `material-symbols.js: "${name}" must match ${ICON_NAME_RE} or Google Fonts cannot subset it via charset text=`,
      );
    }
    seen.add(name);
  }
  const duplicateSlots = materialSymbols.length - seen.size;
  if (duplicateSlots > 0) {
    console.warn(
      `Audit: material-symbols.js has ${duplicateSlots} redundant list entries (${materialSymbols.length} total, ${seen.size} unique names)`,
    );
  }
  return seen.size;
}

function collectNamesAndCharset() {
  auditMaterialSymbolsList();

  const mimeSrc = fs.readFileSync(path.join(root, "src/utils/mimetype.js"), "utf8");
  const fromMime = [...mimeSrc.matchAll(/materialSymbol:\s*"([^"]+)"/g)].map((m) => m[1]);

  const staticExtras = [
    "replay_10",
    "forward_10",
    "expand_more",
    "arrow_upward",
    "arrow_downward",
    "repeat_one",
    "repeat",
    "chevron_left",
    "chevron_right",
    "list_alt",
    "animation",
    "group",
    "table_rows_narrow",
    "horizontal_rule",
    "help",
    "cloud_download",
    "notifications",
    "share",
    "admin_panel_settings",
    "visibility",
    "cloud_upload",
    "insert_drive_file",
    "sync_problem",
    "do_not_disturb_on",
    "interests",
    "description",
    "error",
    "info",
  ];

  const names = new Set([...materialSymbols, ...fromMime, ...staticExtras]);

  for (const name of names) {
    if (typeof name !== "string") {
      throw new Error(`collectNames: non-string in combined icon set`);
    }
    if (!ICON_NAME_RE.test(name)) {
      throw new Error(
        `MIME map or static extras: icon name "${name}" must match ${ICON_NAME_RE}`,
      );
    }
  }

  const chars = new Set();
  for (const name of names) {
    for (const ch of name) {
      chars.add(ch);
    }
  }

  const subsetText = [...chars].sort().join("");
  return {
    subsetText,
    uniqueListSize: materialSymbols.length,
    uniqueIconNames: names.size,
    mimeCount: fromMime.length,
  };
}

function parseWoff2Url(css) {
  const m = css.match(/url\((https:\/\/fonts\.gstatic\.com[^)]+)\)\s+format\(['"]woff2['"]\)/);
  return m ? m[1] : null;
}

async function main() {
  const { subsetText, uniqueListSize, uniqueIconNames, mimeCount } = collectNamesAndCharset();

  const cssUrl = `https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined&display=swap&text=${encodeURIComponent(subsetText)}`;

  const cssRes = await fetch(cssUrl, { headers: { "User-Agent": CHROME_UA } });
  if (!cssRes.ok) {
    throw new Error(`Google Fonts CSS failed: ${cssRes.status} ${cssRes.statusText}`);
  }
  const css = await cssRes.text();
  const fontUrl = parseWoff2Url(css);
  if (!fontUrl) {
    throw new Error("Could not find woff2 URL in Google Fonts CSS (response missing or format changed).");
  }

  const fontRes = await fetch(fontUrl, { headers: { "User-Agent": CHROME_UA } });
  if (!fontRes.ok) {
    throw new Error(`Font download failed: ${fontRes.status} ${fontRes.statusText}`);
  }
  const buffer = Buffer.from(await fontRes.arrayBuffer());

  if (buffer.length < MIN_SUBSET_BYTES || buffer.length > MAX_SUBSET_BYTES) {
    throw new Error(
      `Downloaded font size ${buffer.length} bytes is outside expected range [${MIN_SUBSET_BYTES}, ${MAX_SUBSET_BYTES}]; refusing to write`,
    );
  }

  const outPath = path.join(root, "public/fonts/material-symbols.woff2");
  const prev = fs.existsSync(outPath) ? fs.statSync(outPath).size : 0;
  fs.writeFileSync(outPath, buffer);

  console.log(
    `Audit: ${uniqueListSize} picker entries → ${uniqueIconNames} unique icon names (${mimeCount} MIME mappings + extras); charset ${subsetText.length} chars`,
  );
  console.log(
    `Material Symbols subset: ${(prev / 1024).toFixed(0)} KB → ${(buffer.length / 1024).toFixed(0)} KB`,
  );
  console.log(`Wrote ${path.relative(root, outPath)}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
