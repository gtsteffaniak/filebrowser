import { defineConfig } from "eslint/config";
import * as tsParser from "@typescript-eslint/parser";
import pluginVue from "eslint-plugin-vue";
import vueI18n from "@intlify/eslint-plugin-vue-i18n";

export default defineConfig(
  { ignores: ["**/dist/**", "tests/playwright-files/**"] },

  ...pluginVue.configs["flat/essential"],
  ...vueI18n.configs.recommended,

  // js/ts is parsed for vue-i18n to check t() calls
  { files: ["**/*.js", "**/*.ts"], languageOptions: { parser: tsParser } },
  { files: ["**/*.vue"], languageOptions: { parserOptions: { parser: tsParser } } },
  {
    settings: {
      "vue-i18n": {
        localeDir: "src/i18n/en.json",
        messageSyntaxVersion: "^11.0.0",
      },
    },
    rules: {
      "@intlify/vue-i18n/no-missing-keys": "error",
      "@intlify/vue-i18n/no-unused-keys": ["error", {
        src: "./src",
        extensions: [".js", ".vue", ".ts"],
        ignores: ["/^languages\\./"],
      }],
      "@intlify/vue-i18n/no-raw-text": ["error", {
        ignoreNodes: ["i", "v-icon"],
      }],
      "@intlify/vue-i18n/no-missing-keys-in-other-locales": "warn",
    },
  },
  {
    files: ["**/*.vue"],
    rules: {
      // this are the most common ones that likely biome also has
      // we can add more to the list or if some if buggy in biome better enable it here.
      "vue/no-duplicate-attributes": "off",
      "vue/no-use-v-if-with-v-for": "off",
      "vue/require-v-for-key": "off",
      "vue/valid-template-root": "off",
      "vue/valid-v-bind": "off",
      "vue/valid-v-cloak": "off",
      "vue/valid-v-else": "off",
      "vue/valid-v-else-if": "off",
      "vue/valid-v-for": "off",
      "vue/valid-v-html": "off",
      "vue/valid-v-if": "off",
      "vue/valid-v-on": "off",
      "vue/valid-v-once": "off",
      "vue/valid-v-pre": "off",
      "vue/valid-v-text": "off",
      "vue/no-deprecated-v-on-number-modifiers": "off",
      "vue/no-dupe-keys": "off",
      "vue/no-reserved-keys": "off",
      "vue/no-reserved-props": "off",
      "vue/no-deprecated-data-object-declaration": "off",
      "vue/no-arrow-functions-in-watch": "off",
      "vue/no-ref-as-operand": "off",
      "vue/prefer-import-from-vue": "off",
      "vue/multi-word-component-names": "off",
      "vue/no-reserved-component-names": "off",
      "vue/no-parsing-error": "off",
      "vue/no-unused-components": "warn",
      "vue/no-v-text-v-html-on-component": "warn",
    },
  },
);
