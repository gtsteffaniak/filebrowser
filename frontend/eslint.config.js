// eslint.config.js

import vue from 'eslint-plugin-vue';

export default [
  {
    ignores: ['node_modules/**'],
  },
  {
    files: ['**/*.js', '**/*.vue'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
    },
    env: {
      browser: true,
      es2021: true,
    },
    plugins: {
      vue,
    },
    extends: [
      'eslint:recommended',
      'plugin:vue/vue3-essential',
    ],
    rules: {
      'vue/multi-word-component-names': 'off',
      'vue/no-reserved-component-names': 'warn',
      'vue/no-mutating-props': 'off',
      'vue/no-deprecated-v-bind-sync': 'warn',
    },
  },
];
