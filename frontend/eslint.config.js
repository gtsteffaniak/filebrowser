import { defineConfig } from "eslint/config";
import js from "@eslint/js";
import tseslint from "typescript-eslint";
import pluginVue from "eslint-plugin-vue";
import vueI18n from '@intlify/eslint-plugin-vue-i18n';
import vueParser from "vue-eslint-parser";
import globals from "globals";
import security from "eslint-plugin-security";

export default defineConfig(
  {
    ignores: [
      "**/dist/**",
      "**/node_modules/**",
      "**/public/**",
      "**/*.json",
    ],
  },

  // Defaults
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...pluginVue.configs["flat/essential"],
  ...vueI18n.configs.recommended,
  security.configs.recommended,

  // i18n
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

  // Shared globals + rule overrides
  {
    files: ["**/*.js", "**/*.ts", "**/*.vue"],
    languageOptions: {
      globals: {
        ...globals.node,
        ...globals.browser,
        ...globals.es2022,
        globalVars: "readonly",
        router: "readonly",
        $t: "readonly",
        next: "readonly",
        downloadFiles: "readonly",
      },
    },
    rules: {
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-empty-object-type": "warn",
      "@typescript-eslint/ban-ts-comment": "warn",
      "@typescript-eslint/no-unused-vars": ["error", {
        argsIgnorePattern: "^_",
        varsIgnorePattern: "^_",
        caughtErrors: "none",
      }],
      "eqeqeq": ["warn", "always"],
      "no-var": "error",
      "prefer-const": "warn",
      "no-unused-expressions": ["error", { allowShortCircuit: true, allowTernary: true }],
      "prefer-template": "warn",
      "@typescript-eslint/consistent-type-definitions": "warn",
      "@typescript-eslint/prefer-optional-chain": "warn",
      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-unnecessary-condition": "off", // this one is useful, but is pretty noisy and found lot of false positives
      "@typescript-eslint/no-dynamic-delete": "warn",
      "@typescript-eslint/no-misused-promises": "error",
      "prefer-object-has-own": "error",
      "no-prototype-builtins": "error",
      "no-implied-eval": "error",
      "no-restricted-globals": [
        "error",
        { name: "isNaN", message: "Use Number.isNaN instead." },
        { name: "isFinite", message: "Use Number.isFinite instead." }
      ],
    },
  },

  {
    files: ["**/*.ts", "**/*.js"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: "latest",
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
        extraFileExtensions: ['.vue'],
      },
    },
  },

  // Vue files
  {
    files: ["**/*.vue"],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        parser: tseslint.parser,
        ecmaVersion: "latest",
        sourceType: "module",
        projectService: true,
        extraFileExtensions: ['.vue'],
      },
    },
    rules: {
      "vue/multi-word-component-names": "off",
      "vue/no-mutating-props": ["error", { shallowOnly: true }],
      //"vue/order-in-components": "warn",
      "vue/require-v-for-key": "error",
      "vue/no-reserved-component-names": "off",
      "vue/no-unused-components": "warn",
      //"vue/no-v-html": "warn",
      "vue/no-v-text-v-html-on-component": "warn",
    },
  },
  // Relax the rules a bit for tests files
  {
    files: ["tests/**/*.ts", "**/*.spec.ts", "**/*.test.ts"],
    rules: {
      "preserve-caught-error": "off",
      "no-empty-pattern": "off",
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unused-vars": ["error", { caughtErrors: "none" }],
      "no-unused-expressions": "off",
      "security/detect-non-literal-regexp": "off",
    },
  },
);
