import { toStandardLocale } from "../i18n/index.ts";

type DateInput = string | number | Date;

const relativeFormatters = new Map<string, Intl.RelativeTimeFormat>();
const dateTimeFormatters = new Map<string, Intl.DateTimeFormat>();

function cachedFormatter<T, O extends object>(
  cache: Map<string, T>,
  Ctor: new (locale: string, options?: O) => T,
  locale: string,
  options: O,
): T {
  const key = JSON.stringify([locale, options]);
  let formatter = cache.get(key);
  if (!formatter) {
    formatter = new Ctor(locale, options);
    cache.set(key, formatter);
  }
  return formatter;
}

export function fromNow(date: DateInput, locale: string = 'en-us'): string {
  const normalized = normalizeDate(date);
  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - normalized.getTime()) / 1000);
  const intervals: { label: Intl.RelativeTimeFormatUnit; seconds: number }[] = [
    { label: 'year', seconds: 60 * 60 * 24 * 365 },
    { label: 'month', seconds: 2592000 },
    { label: 'week', seconds: 604800 },
    { label: 'day', seconds: 86400 },
    { label: 'hour', seconds: 3600 },
    { label: 'minute', seconds: 60 },
    { label: 'second', seconds: 1 },
  ];
  const formatter = cachedFormatter(relativeFormatters, Intl.RelativeTimeFormat, toStandardLocale(locale), { numeric: 'auto' });
  // Use absolute value for calculations
  const absDiffInSeconds = Math.abs(diffInSeconds);
  for (const interval of intervals) {
    const count = Math.floor(absDiffInSeconds / interval.seconds);
    if (count > 0) {
      // For past dates (diffInSeconds > 0), we want negative values
      // For future dates (diffInSeconds < 0), we want positive values
      const formattedCount = diffInSeconds > 0 ? -count : count;
      return formatter.format(formattedCount, interval.label);
    }
  }
  return 'just now';
}

export function formatTimestamp(date: DateInput, locale: string = 'en-us', { seconds = true }: { seconds?: boolean } = {}): string {
  const normalized = normalizeDate(date);

  if (!(normalized instanceof Date) || Number.isNaN(normalized.getTime())) {
    console.error('Invalid date object:', normalized);
    return 'Invalid Date';
  }

  const options: Intl.DateTimeFormatOptions = {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  };
  if (seconds) {
    options.second = '2-digit';
  }

  try {
    return cachedFormatter(dateTimeFormatters, Intl.DateTimeFormat, toStandardLocale(locale), options).format(normalized);
  } catch (error) {
    console.error('Error formatting date:', error);
    return 'Invalid Date';
  }
}

function normalizeDate(date: DateInput): Date {
  let normalizedDate: Date;

  if (typeof date === 'string') {
    // Parse the date string
    normalizedDate = new Date(date);
  } else if (typeof date === 'number') {
    // Convert seconds to milliseconds if necessary
    normalizedDate = new Date(date * (date < 1e12 ? 1000 : 1));
  } else if (date instanceof Date && !Number.isNaN(date.getTime())) {
    // It's already a valid Date object
    normalizedDate = date;
  } else {
    throw new Error("Invalid date provided");
  }

  return normalizedDate;
}

/**
 * YYYY-MM-DD from `<input type="date">` → Unix seconds at 00:00:00 UTC (search date filters).
 */
export function utcStartOfDaySecondsFromDateInput(isoDate: unknown): number | null {
  if (isoDate === "" || typeof isoDate !== "string") {
    return null;
  }
  const parts = isoDate.split("-");
  if (parts.length !== 3) {
    return null;
  }
  const y = Number(parts[0]);
  const m = Number(parts[1]);
  const d = Number(parts[2]);
  if (!Number.isFinite(y) || !Number.isFinite(m) || !Number.isFinite(d)) {
    return null;
  }
  return Math.floor(Date.UTC(y, m - 1, d) / 1000);
}

export default {
  formatTimestamp,
  fromNow,
  utcStartOfDaySecondsFromDateInput,
};
