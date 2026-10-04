import { globalVars } from "@/utils/constants";

const DEFAULT_PASSWORD_MIN_LENGTH = 5;

export function passwordMinLength() {
  const n = Number(globalVars?.passwordMinLength);
  if (!Number.isFinite(n) || n < 1) {
    return DEFAULT_PASSWORD_MIN_LENGTH;
  }
  return n;
}

export function evaluatePasswordPolicy(password, confirmPassword) {
  const a = String(password ?? "");
  const b = String(confirmPassword ?? "");
  const min = passwordMinLength();
  const minLengthMet = a.length >= min;
  const passwordsMatch = a.length > 0 && b.length > 0 && a === b;
  const valid = minLengthMet && passwordsMatch;
  return {
    minLength: min,
    minLengthMet,
    passwordsMatch,
    valid,
    showMismatch: b.length > 0 && a !== b,
    showLengthHint: a.length > 0,
  };
}
