/**
 * Percentage for usage labels, capped at 100 to match ProgressBar fill width.
 * Byte sizes in the label stay uncapped so nested-mount mismatches remain visible.
 */
export function cappedUsagePercent(val, max) {
  const n = Number(val);
  const m = Number(max);
  if (!Number.isFinite(n) || !Number.isFinite(m) || m <= 0) {
    return 0;
  }
  return Math.min(100, Math.round((n / m) * 100));
}
