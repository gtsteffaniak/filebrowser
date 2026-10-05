import { defineConfig } from "eslint/config";
import * as tsParser from "@typescript-eslint/parser";
import pluginVue from "eslint-plugin-vue";
import vueI18n from "@intlify/eslint-plugin-vue-i18n";
import biome from "eslint-config-biome";

export default defineConfig(
  { ignores: ["**/dist/**", "tests/playwright-files/**"] },

  ...pluginVue.configs["flat/essential"],
  ...vueI18n.configs.recommended,
  biome,

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
      "vue/no-reserved-component-names": "off",
      "vue/multi-word-component-names": "off",
      "vue/no-mutating-props": ["error", { shallowOnly: true }],
      //"vue/order-in-components": "warn",
      "vue/no-side-effects-in-computed-properties": "error",
      "vue/no-async-in-computed-properties": "error",
      "vue/return-in-computed-property": "error",
      "vue/no-lifecycle-after-await": "error",
      "vue/no-watch-after-await": "error",
      "vue/no-unused-vars": "error",
      "vue/valid-v-model": "error",
      "vue/valid-v-slot": "error",
      "vue/valid-v-show": "error",
      "vue/no-dupe-v-else-if": "error",
      "vue/no-unused-components": "warn",
      "vue/no-v-text-v-html-on-component": "warn",
    },
  },
);
