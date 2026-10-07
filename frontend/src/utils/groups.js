/**
 * Client-side group name rules mirroring backend access.NormalizeGroupName.
 * They apply only to manually created groups; existing names are always valid.
 */
import i18n from '@/i18n';

export const MAX_GROUP_NAME_LENGTH = 128;
export const MIN_GROUP_NAME_LENGTH = 2;

// eslint-disable-next-line no-control-regex
const CONTROL_CHARS = /[\x00-\x1f\x7f-\x9f]/;

/**
 * Returns an i18n error key for a new group name, or null when valid.
 * @param {string} name
 * @returns {string|null} key under the "access.*" namespace
 */
export function groupNameError(name) {
  const trimmed = (name || "").trim();
  if (!trimmed) {
    return "groupNameRequired";
  }
  if (trimmed.length < MIN_GROUP_NAME_LENGTH) {
    return "groupNameTooShort";
  }
  if (trimmed.length > MAX_GROUP_NAME_LENGTH) {
    return "groupNameTooLong";
  }
  if (CONTROL_CHARS.test(trimmed)) {
    return "groupNameInvalid";
  }
  return null;
}

/**
 * Translated validation message for a new group name, or null when valid.
 * @param {string} name
 * @returns {string|null}
 */
export function groupNameErrorText(name) {
  // Literal keys for compile-time i18n validation.
  switch (groupNameError(name)) {
    case "groupNameRequired":
      return i18n.global.t("access.groupNameRequired");
    case "groupNameTooShort":
      return i18n.global.t("access.groupNameTooShort", { min: MIN_GROUP_NAME_LENGTH });
    case "groupNameTooLong":
      return i18n.global.t("access.groupNameTooLong", { max: MAX_GROUP_NAME_LENGTH });
    case "groupNameInvalid":
      return i18n.global.t("access.groupNameInvalid");
    default:
      return null;
  }
}
