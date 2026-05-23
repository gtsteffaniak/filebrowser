import { defineConfig } from "eslint/config";
import js from "@eslint/js";
import tseslint from "typescript-eslint";
import pluginVue from "eslint-plugin-vue";
import pluginI18n from "@intlify/eslint-plugin-vue-i18n";
import vueParser from "vue-eslint-parser";
import globals from "globals";

export default defineConfig(
  {
    ignores: [
      "**/dist/**",
      "**/node_modules/**",
      "**/public/**",
    ],
  },

  // Defaults
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...pluginVue.configs["flat/essential"],

  // Shared globals + rule overrides
  {
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
      "vue/no-reserved-component-names": "off",
      "vue/no-unused-components": "warn",
      "eqeqeq": ["warn", "always"],
      "no-var": "error",
      "prefer-const": "warn",
      "no-unused-expressions": ["error", { allowShortCircuit: true, allowTernary: true }],
      "prefer-template": "warn",
      "@typescript-eslint/consistent-type-definitions": "warn",
      "@typescript-eslint/prefer-optional-chain": "warn",
      "no-prototype-builtins": "error",
      "no-restricted-globals": [
        "error",
        { name: "isNaN", message: "Use Number.isNaN instead." },
        { name: "isFinite", message: "Use Number.isFinite instead." }
      ],
    },
  },

  {
    files: ["**/*.ts", "**/*.js" ],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
        extraFileExtensions: ['.vue'],
      },
    },
  },

  // i18n
  {
    files: ["**/*.vue", "**/*.js", "**/*.ts"],
    plugins: {
      "@intlify/vue-i18n": pluginI18n,
    },
    settings: {
      "vue-i18n": {
        localeDir: "src/i18n/en.json",
        messageSyntaxVersion: "^9.0.0",
      },
    },
    rules: {
      "@intlify/vue-i18n/no-missing-keys": "error",
      "@intlify/vue-i18n/no-unused-keys": ["error", {
        src: "./src",
        extensions: [".js", ".vue", ".ts"],
      }],
      "@intlify/vue-i18n/no-raw-text": ["error", {
        ignoreNodes: ["i", "v-icon"],
      }],
      "@intlify/vue-i18n/no-missing-keys-in-other-locales": "warn",
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
    },
  },
);
