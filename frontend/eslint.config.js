import { defineConfig } from "eslint/config";
import * as tsParser from "@typescript-eslint/parser";
import pluginVue from "eslint-plugin-vue";
import vueI18n from "@intlify/eslint-plugin-vue-i18n";

export default defineConfig(
  ...pluginVue.configs["flat/base"],
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
);
