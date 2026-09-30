export const MB = 1024 * 1024;
export const GB = 1024 * 1024 * 1024;

/**
 * @param {number} amount
 * @param {'mb' | 'gb'} unit
 */
export function bytesFromCustomAmount(amount, unit) {
  const n = Number(amount);
  if (!Number.isFinite(n) || n <= 0) {
    return 0;
  }
  const mult = unit === "gb" ? GB : MB;
  return Math.round(n * mult);
}

/**
 * Pick display amount and unit when loading a byte limit into custom mode.
 * @param {number} bytes
 * @returns {{ amount: number; unit: 'mb' | 'gb' }}
 */
export function customAmountFromBytes(bytes) {
  const b = Number(bytes);
  if (!Number.isFinite(b) || b <= 0) {
    return { amount: 1, unit: "gb" };
  }
  if (b % GB === 0) {
    return { amount: b / GB, unit: "gb" };
  }
  if (b % MB === 0) {
    return { amount: b / MB, unit: "mb" };
  }
  if (b < GB) {
    return { amount: Math.round((b / MB) * 100) / 100, unit: "mb" };
  }
  return { amount: Math.round((b / GB) * 10) / 10, unit: "gb" };
}
