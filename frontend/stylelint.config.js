export default {
  extends: [
    "stylelint-config-standard",
    "stylelint-config-standard-vue",
  ],
  ignoreFiles: [
    "**/dist/**",
    "**/node_modules/**",
    "**/public/**",
    "!**/public/css/**",
    "**/pkg/**",
  ],
 "referenceFiles": [
   "src/css/_variables.css",
   "src/css/_fab.css",
   "public/css/variables.css"
 ],
  rules: {
    "no-descending-specificity": true,
    "at-rule-no-deprecated": true,
    "media-type-no-deprecated": true,
    "declaration-property-value-no-unknown": true,
    "selector-class-pattern": null,
    "selector-id-pattern": null, // disable enforcing of kebab-case
    "custom-property-pattern": null,
    "rule-empty-line-before": [
      "always-multi-line",
      {
        except: ["first-nested"],
        ignore: ["after-comment", "inside-block"],
      },
    ],
    "comment-empty-line-before": null,
    "declaration-empty-line-before": null,
    "font-family-no-missing-generic-family-keyword": null,
    "no-unknown-animations": true,
    "no-unknown-custom-properties": true,
    "no-unknown-custom-media": true,
    "function-no-unknown": true,
    "selector-no-deprecated": true,
    "color-no-invalid-hex": true,
    "selector-no-invalid": true,
    "function-linear-gradient-no-nonstandard-direction": true,
    "selector-pseudo-element-disallowed-list": [/^-ms-/, /^-moz-focus-/],
    "property-disallowed-list": [/^-ms-/, "-moz-osx-font-smoothing"],
  },
};
