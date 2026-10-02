// PostCSS plugin that drops legacy IE, old-Firefox CSS that most browsers now ignore or log warnings.

const LEGACY_SELECTOR = /::?-ms-|::-moz-focus-(inner|outer)/;
const LEGACY_PROP = /^-ms-/;
const LEGACY_PROP_NAMES = new Set(['-moz-osx-font-smoothing']);
const HACK_PROP = /^[*_]/;

// Old IE value syntax
const LEGACY_VALUE = /progid:|expression\(/i;

// At-rules like @-ms-viewport, @-ms-keyframes ...
const LEGACY_AT_RULE = /^-ms-/;

const plugin = () => ({
  postcssPlugin: 'strip-legacy-vendor-css',

  Rule(rule) {
    if (!LEGACY_SELECTOR.test(rule.selector)) return;
    const kept = rule.selectors.filter((s) => !LEGACY_SELECTOR.test(s));
    if (kept.length) rule.selectors = kept;
    else rule.remove();
  },

  Declaration(decl) {
    const legacy =
      LEGACY_PROP.test(decl.prop) ||
      LEGACY_PROP_NAMES.has(decl.prop) ||
      HACK_PROP.test(decl.prop) ||
      LEGACY_VALUE.test(decl.value);
    if (!legacy) return;

    const parent = decl.parent;
    decl.remove();
    if (parent?.type === 'rule' && parent.nodes.length === 0) {
      parent.remove();
    }
  },

  AtRule(atRule) {
    if (LEGACY_AT_RULE.test(atRule.name)) atRule.remove();
  },
});
plugin.postcss = true;

export default plugin;
