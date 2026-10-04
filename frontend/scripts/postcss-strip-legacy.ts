// PostCSS plugin that drops legacy IE, old-Firefox CSS that most browsers now ignore or log warnings.

import type {
  AtRule as PostcssAtRule,
  Declaration as PostcssDeclaration,
  PluginCreator,
  Rule as PostcssRule,
} from "postcss";

const LEGACY_SELECTOR = /::?-ms-|::-moz-focus-(inner|outer)/;
const LEGACY_PROP = /^-ms-/;
const LEGACY_PROP_NAMES = new Set(["-moz-osx-font-smoothing"]);
// Old IE value syntax
const LEGACY_VALUE = /progid:|expression\(/i;
const STRING_OR_URL = /"[^"]*"|'[^']*'|url\([^)]*\)/gi;
// At-rules like @-ms-viewport, @-ms-keyframes ...
const LEGACY_AT_RULE = /^-ms-/;

const plugin: PluginCreator<void> = Object.assign(
  () => ({
    postcssPlugin: "strip-legacy-vendor-css",

    Rule(rule: PostcssRule) {
      if (!LEGACY_SELECTOR.test(rule.selector)) return;
      const kept = rule.selectors.filter((s) => !LEGACY_SELECTOR.test(s));
      if (kept.length) rule.selectors = kept;
      else rule.remove();
    },

    Declaration(decl: PostcssDeclaration) {
      const legacy =
        LEGACY_PROP.test(decl.prop) ||
        LEGACY_PROP_NAMES.has(decl.prop) ||
        LEGACY_VALUE.test(decl.value.replace(STRING_OR_URL, '""'));
      if (!legacy) return;

      const parent = decl.parent;
      decl.remove();
      if (parent?.type === "rule" && parent.nodes.length === 0) {
        parent.remove();
      }
    },
    AtRule(atRule: PostcssAtRule) {
      if (LEGACY_AT_RULE.test(atRule.name)) atRule.remove();
    },
  }),
  { postcss: true as const },
);

export default plugin;
