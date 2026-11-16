import fs from "node:fs";
import { parse } from "vue/compiler-sfc";
import { locales } from "../../src/i18n/locales.ts";
import { isTestFile, maskSource, readLocale, rel, sourceFiles, translationFiles } from "./shared.ts";

const IGNORED_ELEMENTS = new Set(["i", "v-icon", "kbd"]);
const IGNORE_COMMENT = "i18n-ignore";
const TEXT_ATTRIBUTES = new Set(["placeholder", "title", "alt"]);

// Vue's NodeTypes values
const ELEMENT = 1;
const TEXT = 2;
const COMMENT = 3;
const INTERPOLATION = 5;
const ATTRIBUTE = 6;
const DIRECTIVE = 7;

interface TemplateNode {
  type: number;
  tag?: string;
  content?: unknown;
  props?: {
    type?: number;
    name?: string;
    value?: { content: string };
    arg?: { content?: string; isStatic?: boolean };
    exp?: { content?: string };
    loc?: { start: { line: number } };
  }[];
  children?: TemplateNode[];
  loc: { start: { line: number } };
}

/** The code of a source file that can use a key -- comments and <style> blocks are dropped */
const codeOf = (file: string) => maskSource(file, fs.readFileSync(file, "utf8"));

/** Every key in en.json must appear as a string literal in src/ */
export function checkUnusedKeys(): string[] {
  const code = [...sourceFiles()].filter((file) => !isTestFile(file)).map(codeOf).join("\n");
  const isUsed = (key: string) => ["'", '"', "`"].some((quote) => code.includes(`${quote}${key}${quote}`));
  return Object.keys(readLocale("en.json"))
    .filter((key) => !isUsed(key))
    .map((key) => `en.json: unused key ${key}`);
}


export function checkLocaleRegistry(): string[] {
  const files = new Set(["en.json", ...translationFiles()]);
  const listed = new Set(Object.values(locales).map(({ file }) => `${file}.json`));
  return [
    ...[...files].filter((file) => !listed.has(file)).map((file) => `${file}: not listed in src/i18n/locales.ts`),
    ...Object.entries(locales)
      .filter(([, { file }]) => !files.has(`${file}.json`))
      .map(([id, { file }]) => `locales.ts: ${id} has no file ${file}.json`),
  ];
}

const isIgnored = (node: TemplateNode) =>
  IGNORED_ELEMENTS.has(node.tag ?? "") ||
  (node.props ?? []).some((prop) => prop.name === "class" && prop.value?.content.includes("material-symbols"));

const TRANSLATION_CALL = /^\$?t\(.*\)$/s;

function rawAttributeMessage(name: string, value: string): string {
  const text = value.trim();
  if (TRANSLATION_CALL.test(text)) {
    return `${name}="${text}" is a plain string, not a translation, add a colon: :${name}="${text}"`;
  }
  return `raw ${name} "${text}", use t()`;
}

function stringLiteralOf(expression: string): string | null {
  const match = /^\s*(?:'([^'\\\n]*)'|"([^"\\\n]*)"|`([^`$\\]*)`)\s*$/.exec(expression);
  return match ? (match[1] ?? match[2] ?? match[3] ?? "") : null;
}

const hasLetters = (text: string) => /\p{L}/u.test(text);

/** Templates shouldn't have plain text, to ignore them we can use <!-- i18n-ignore --> */
export function checkRawText(): string[] {
  const problems: string[] = [];
  for (const file of sourceFiles()) {
    if (!file.endsWith(".vue")) continue;
    const ast = parse(fs.readFileSync(file, "utf8"), { filename: file }).descriptor.template?.ast;
    const walk = (node: TemplateNode) => {
      for (const prop of node.type === ELEMENT ? (node.props ?? []) : []) {
        const line = prop.loc?.start.line ?? node.loc.start.line;
        if (prop.type === ATTRIBUTE && TEXT_ATTRIBUTES.has(prop.name ?? "") && hasLetters(prop.value?.content ?? "")) {
          problems.push(`${rel(file)}:${line}: ${rawAttributeMessage(prop.name ?? "", prop.value?.content ?? "")}`);
        } else if (prop.type === DIRECTIVE && prop.name === "bind" && prop.arg?.isStatic && TEXT_ATTRIBUTES.has(prop.arg.content ?? "")) {
          const literal = stringLiteralOf(prop.exp?.content ?? "");
          if (literal !== null && hasLetters(literal)) {
            problems.push(`${rel(file)}:${line}: raw ${prop.arg.content} "${literal.trim()}" bound as a string, use t()`);
          }
        }
      }
      if (node.type === ELEMENT && isIgnored(node)) return;
      if (node.type === INTERPOLATION) {
        const literal = stringLiteralOf((node.content as { content?: string } | undefined)?.content ?? "");
        if (literal !== null && hasLetters(literal)) problems.push(`${rel(file)}:${node.loc.start.line}: raw text "${literal.trim()}" in {{ }}, use t()`);
      }
      if (node.type === TEXT && typeof node.content === "string" && /\p{L}/u.test(node.content)) {
        problems.push(`${rel(file)}:${node.loc.start.line}: raw text "${node.content.trim()}", use t()`);
      }
      let skip = false;
      for (const child of node.children ?? []) {
        if (!skip) walk(child);
        if (child.type === COMMENT) skip = typeof child.content === "string" && child.content.trim() === IGNORE_COMMENT;
        else if (child.type === ELEMENT || (child.type === TEXT && String(child.content).trim())) skip = false;
      }
    };
    if (ast) walk(ast as unknown as TemplateNode);
  }
  return problems;
}
