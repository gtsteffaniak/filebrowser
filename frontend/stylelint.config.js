export default {
  extends: [
    "stylelint-config-standard",
    "stylelint-config-standard-vue",
  ],
  ignoreFiles: ["**/dist/**", "**/node_modules/**", "**/public/**"],
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
        except: ["first-nested", "inside-block"],
        ignore: ["after-comment"],
      },
    ],
    "comment-empty-line-before": null,
    "font-family-no-missing-generic-family-keyword": null
  },
};
