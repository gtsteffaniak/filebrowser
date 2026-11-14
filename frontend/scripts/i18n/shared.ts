import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "vue/compiler-sfc";

export const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
export const srcDir = path.join(root, "src");
export const i18nDir = path.join(srcDir, "i18n");
export const localesDir = path.join(i18nDir, "locales");
export const generatedFile = path.join(i18nDir, "messages.generated.ts");

export type Messages = Record<string, string>;

/** Tests t/$t with throwaway keys */
export const isTestFile = (file: string) => /\.test\.[jt]s$/.test(file);
export const rel = (file: string) => path.relative(root, file);
export const readJson = (file: string): unknown => JSON.parse(fs.readFileSync(file, "utf8"));

/** Flat nested messages into `{ "a.b.c": "message" }`; null counts as empty message. */
export function flatten(tree: unknown, prefix = ""): Messages {
  const result: Messages = {};
  for (const [key, value] of Object.entries(tree as Record<string, unknown>)) {
    const fullKey = prefix + key;
    if (typeof value === "string") {
      result[fullKey] = value;
    } else if (value === null) {
      result[fullKey] = "";
    } else if (typeof value === "object") {
      Object.assign(result, flatten(value, `${fullKey}.`));
    } else {
      throw new Error(`${fullKey}: messages must be strings or objects`);
    }
  }
  return result;
}

export const readLocale = (file: string): Messages => flatten(readJson(path.join(localesDir, file)));

/** File names of every locale except en.json, sorted */
export const translationFiles = (): string[] =>
  fs
    .readdirSync(localesDir)
    .filter((file) => file.endsWith(".json") && file !== "en.json")
    .sort();

/** .vue, .ts and .js in src/ without the locales and the generated types */
export function* sourceFiles(skipI18nDir = false, dir = srcDir): Generator<string> {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (full !== localesDir && !(skipI18nDir && full === i18nDir)) yield* sourceFiles(skipI18nDir, full);
    } else if (/\.(vue|ts|js)$/.test(entry.name) && full !== generatedFile) {
      yield full;
    }
  }
}

export function report(problems: string[]): void {
  for (const problem of problems) console.error(`✗ ${problem}`);
  console.error(`\n${problems.length} problem(s) found.`);
}

const blank = (text: string) => text.replace(/[^\n]/g, " ");

/** Replaces the comments of JS/TS code by spaces and, with `strings`, the content of '...' and "..." literals too */
export function maskJs(text: string, strings = false): string {
  const scan = (start: number, inExpression: boolean): [string, number] => {
    let out = "";
    let i = start;
    let depth = 0;
    while (i < text.length) {
      const ch = text.charAt(i);
      const next = text.charAt(i + 1);
      if (ch === "/" && (next === "/" || next === "*")) {
        const close = next === "/" ? text.indexOf("\n", i) : text.indexOf("*/", i + 2);
        const end = close === -1 ? text.length : next === "/" ? close : close + 2;
        out += blank(text.slice(i, end));
        i = end;
      } else if (ch === "`") {
        const [masked, end] = maskTemplate(i);
        out += masked;
        i = end;
      } else if (ch === '"' || ch === "'") {
        let end = i + 1;
        while (end < text.length && text.charAt(end) !== ch) {
          if (text.charAt(end) === "\\") end++;
          else if (text.charAt(end) === "\n") break; // it's not a string
          end++;
        }
        const closed = text.charAt(end) === ch;
        const literal = text.slice(i, closed ? end + 1 : end);
        out += strings && closed ? ch + blank(literal.slice(1, -1)) + ch : literal;
        i += literal.length;
      } else {
        if (inExpression && ch === "{") depth++;
        else if (inExpression && ch === "}") {
          if (depth === 0) return [out, i];
          depth--;
        }
        out += ch;
        i++;
      }
    }
    return [out, i];
  };

  // A template literal from its opening backtick -- its `${...}` expressions are scanned
  const maskTemplate = (start: number): [string, number] => {
    const keep = (chunk: string) => (strings ? blank(chunk) : chunk);
    let out = "`";
    let i = start + 1;
    while (i < text.length) {
      const ch = text.charAt(i);
      if (ch === "\\") {
        out += keep(text.slice(i, i + 2));
        i += 2;
      } else if (ch === "`") {
        return [`${out}\``, i + 1];
      } else if (ch === "$" && text.charAt(i + 1) === "{") {
        const [code, end] = scan(i + 2, true);
        out += `\${${code}`;
        i = end;
        if (text.charAt(i) === "}") {
          out += "}";
          i++;
        }
      } else {
        out += keep(ch);
        i++;
      }
    }
    return [out, i];
  };

  return scan(0, false)[0];
}

/**
 * The code of a source file without comments at the same offsets as the file. In .vue files only the template and the scripts
 * are kept while styles no
 */
export function maskSource(file: string, text: string, strings = false): string {
  if (!file.endsWith(".vue")) return maskJs(text, strings);
  const { descriptor } = parse(text, { filename: file });
  let out = text;
  // each replacement keeps the length of what it replaces, so the blocks can be masked one after the other
  const mask = ({ start, end }: { start: { offset: number }; end: { offset: number } }, apply: (code: string) => string) => {
    out = out.slice(0, start.offset) + apply(text.slice(start.offset, end.offset)) + out.slice(end.offset);
  };
  for (const style of descriptor.styles) mask(style.loc, blank);
  for (const script of [descriptor.script, descriptor.scriptSetup]) {
    if (script) mask(script.loc, (code) => maskJs(code, strings));
  }
  if (descriptor.template) mask(descriptor.template.loc, (code) => code.replace(/<!--[\s\S]*?-->/g, blank));
  return out;
}
