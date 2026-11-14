/** biome-ignore-all lint/suspicious/noTemplateCurlyInString: the tests need it */

import fs from "node:fs";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { maskJs, maskSource } from "../shared.ts";
import { syncTranslations } from "../sync.ts";
import { literalCalls } from "../types.ts";

// The sync reads and writes the locales folder: point it at a temporary one.
const dir = await vi.hoisted(async () => {
  const { mkdtempSync } = await import("node:fs");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  return mkdtempSync(join(tmpdir(), "i18n-test-"));
});

vi.mock("../shared.ts", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../shared.ts")>();
  const { readdirSync } = await import("node:fs");
  return {
    ...actual,
    localesDir: dir,
    translationFiles: () => readdirSync(dir).filter((file) => file.endsWith(".json") && file !== "en.json").sort(),
  };
});

vi.mock("deepl-node", () => ({
  DeepLClient: class {
    async translateText(text: string) {
      return { text: `[${text}]` };
    }
  },
}));

const write = (file: string, content: unknown) => fs.writeFileSync(path.join(dir, file), JSON.stringify(content));
const read = (file: string): unknown => JSON.parse(fs.readFileSync(path.join(dir, file), "utf8"));

describe("sync with a different shape in a locale", () => {
  beforeEach(() => {
    process.env.DEEPL_API_KEY = "test-key";
    vi.spyOn(console, "log").mockImplementation(() => {});
    vi.spyOn(console, "warn").mockImplementation(() => {});
  });
  afterEach(() => {
    vi.restoreAllMocks();
    for (const file of fs.readdirSync(dir)) fs.rmSync(path.join(dir, file));
  });

  const run = (options: { enforceOrder?: boolean; cleanup?: boolean } = {}) =>
    syncTranslations({ enforceOrder: false, cleanup: false, ...options });

  describe("an object where en.json has a string", () => {
    beforeEach(() => {
      write("en.json", { general: { title: "Title" } });
      write("de.json", { general: { title: { old: "Alt" } } });
    });

    it("replaces the subtree with a translation", async () => {
      expect(await run()).toBe(0);
      expect(read("de.json")).toEqual({ general: { title: "[Title]" } });
    });

    it("removes the obsolete subtree on cleanup", async () => {
      expect(await run({ cleanup: true })).toBe(0);
      expect(read("de.json")).toEqual({ general: {} });
    });

    it("removes the obsolete subtree when enforcing the order", async () => {
      expect(await run({ enforceOrder: true })).toBe(0);
      expect(read("de.json")).toEqual({ general: {} });
    });
  });

  describe("a string where en.json has an object", () => {
    beforeEach(() => {
      write("en.json", { group: { a: "A" } });
      write("de.json", { group: "legacy" });
    });

    it("replaces the string with the translated keys", async () => {
      await run();
      expect(read("de.json")).toEqual({ group: { a: "[A]" } });
    });

    it("removes the obsolete string on cleanup", async () => {
      await run({ cleanup: true });
      expect(read("de.json")).toEqual({});
    });
  });
});

describe("maskJs", () => {
  it("blanks comments and keeps every offset and newline", () => {
    const code = `a(); // t("x")\n/* t("y")\n */ b();`;
    const masked = maskJs(code);
    expect(masked).toHaveLength(code.length);
    expect(masked).not.toContain("t(");
    expect(masked.split("\n")).toHaveLength(3);
    expect(masked).toContain("a();");
    expect(masked).toContain("b();");
  });

  it("keeps `//` inside strings", () => {
    expect(maskJs(`const u = "https://example.com"; t("k");`)).toContain("https://example.com");
  });

  it("blanks the content of quoted strings only on request, and keeps template literals", () => {
    const code = `x("a 't(' b", 'c'); y(\`\${t("k")}\`);`;
    const bare = maskJs(code, true);
    expect(bare).toHaveLength(code.length);
    expect(bare).not.toContain("a 't(' b");
    expect(bare).toContain('t(" ")');
    expect(maskJs(code)).toContain("a 't(' b");
  });

  it("does not take an apostrophe left open on a line for a string", () => {
    expect(maskJs(`don't\nt("k")`, true)).toContain('t("');
  });
});

describe("maskJs with template literals", () => {
  const masked = (code: string) => maskJs(code, true);

  it("template literal keeps its ${} expressions", () => {
    const code = 'const example = `t("example.only") ${t("real.key")} end`;';
    const bare = masked(code);
    expect(bare).toHaveLength(code.length);
    expect(bare).not.toContain("example.only");
    expect(bare).toContain('t("        ")');
    expect(bare.startsWith("const example = `")).toBe(true);
    expect(bare.endsWith("`;")).toBe(true);
  });

  it("keeps the text of a template literal when the strings are not masked", () => {
    const code = 'const example = `t("example.only") ${t("real.key")}`; // t("c.d")';
    const plain = maskJs(code);
    expect(plain).toContain('`t("example.only") ${t("real.key")}`');
    expect(plain).not.toContain("c.d");
  });

  it("keeps the offsets and the newlines of a template literal that spans lines", () => {
    const code = "const a = `first t(\"x.y\")\nsecond ${\n  t(\"real.key\")\n} third`;\nb();";
    const bare = masked(code);
    expect(bare).toHaveLength(code.length);
    expect(bare.split("\n")).toHaveLength(code.split("\n").length);
    expect(bare.indexOf('t("        ")')).toBe(code.indexOf('t("real.key")'));
    expect(bare).not.toContain("x.y");
    expect(bare).toContain("b();");
  });

  it("does not take `//` or `/*` in the text of a template for a comment", () => {
    const code = "const u = `https://example.com/*path*/` + t('k');";
    expect(maskJs(code)).toBe(code);
    expect(masked(code)).toMatch(/` \+ t\(' '\);$/);
  });

  it("masks a comment inside an expression and the text of a template nested in it", () => {
    const code = "`${ /* t('c.d') */ cond ? `t(\"nested.text\")` : t('real.key') }`";
    const bare = masked(code);
    expect(bare).toHaveLength(code.length);
    expect(bare).not.toContain("c.d");
    expect(bare).not.toContain("nested.text");
    expect(bare).toContain("t('        ')");
  });

  it("closes an expression at its own brace, not at the brace of an object inside it", () => {
    const code = "`a ${ fn({ x: 1 }) } t(\"after.only\") ${ t('real.key') }`";
    const bare = masked(code);
    expect(bare).not.toContain("after.only");
    expect(bare).toContain("t('        ')");
  });

  it("treats an escaped `${` and an escaped backtick as text", () => {
    const code = "`\\${t('escaped.key')} \\` t('after.escape')` + t('real.key')";
    const bare = masked(code);
    expect(bare).toHaveLength(code.length);
    expect(bare).not.toContain("escaped.key");
    expect(bare).not.toContain("after.escape");
    expect(bare).toMatch(/` \+ t\(' +'\)$/);
  });

  it("copes with a template or an expression that is never closed", () => {
    for (const code of ["const a = `t('x.y')", "const a = `${t('x.y')", "const a = `trailing \\"]) {
      expect(masked(code)).toHaveLength(code.length);
      expect(maskJs(code)).toHaveLength(code.length);
    }
  });
});

describe("maskSource for .vue files", () => {
  const vue = [
    "<template>",
    "  <!-- {{ t('a.b') }} -->",
    "  <p>{{ t('c.d') }}</p>",
    "</template>",
    '<script setup lang="ts">',
    "// t('e.f')",
    "const x = t('g.h');",
    "</script>",
    "<style>",
    ".a::after { content: 't(\"i.j\")'; }",
    "</style>",
  ].join("\n");

  it("drops comments and styles but keeps the template and script code at the same offsets", () => {
    const masked = maskSource("A.vue", vue);
    expect(masked).toHaveLength(vue.length);
    for (const dropped of ["a.b", "e.f", "i.j"]) expect(masked).not.toContain(dropped);
    for (const kept of ["c.d", "g.h"]) expect(masked.indexOf(kept)).toBe(vue.indexOf(kept));
  });
});

describe("literalCalls", () => {
  const calls = (source: string) => [...literalCalls(maskJs(source), maskJs(source, true))];

  it("reads a parameter that holds an escaped quote", () => {
    const [call] = calls(`t("prompts.uploadSkipped", { count: "a\\"b".length })`);
    expect(call?.params).toEqual({ names: new Set(["count"]), dynamic: false });
  });

  it("skips a key that is concatenated with an expression", () => {
    expect(calls(`t('prompts.' + name)`)).toEqual([]);
    expect(calls(`t("a" + b, { x: 1 })`)).toEqual([]);
  });

  it("ignores calls in comments and strings, and finds those in template literals", () => {
    const found = calls(`// t('example.key')\nconst s = "t('example.str')";\nconst u = \`\${t("real.key")}\`;`);
    expect(found.map((call) => call.key)).toEqual(["real.key"]);
  });

  it("does not check the text of a template literal as a call", () => {
    expect(calls('const example = `t("example.only")`;')).toEqual([]);
    expect(calls('const example = `see t("example.only") and ${t("real.key")}`;').map((call) => call.key)).toEqual(["real.key"]);
  });

  it("finds a call in an expression nested in a template, not in the template nested in that", () => {
    const found = calls("const a = `${ cond ? `t('nested.text')` : t('real.key', { count: 1 }) }`;");
    expect(found.map((call) => call.key)).toEqual(["real.key"]);
  });

  it("is not confused by a comment in the params", () => {
    const [call] = calls(`t("k", { count, /* it's */ other })`);
    expect(call?.params.names).toEqual(new Set(["count", "other"]));
  });
});
