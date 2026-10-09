import { toStandardLocale } from "../i18n/index.ts";

export function fromNow(date, locale) {
    date = normalizeDate(date);
    const now = new Date();
    const diffInSeconds = Math.floor((now - date) / 1000);
    const intervals = [
        { label: 'year', seconds: 60 * 60 * 24 * 365 },
        { label: 'month', seconds: 2592000 },
        { label: 'week', seconds: 604800 },
        { label: 'day', seconds: 86400 },
        { label: 'hour', seconds: 3600 },
        { label: 'minute', seconds: 60 },
        { label: 'second', seconds: 1 },
    ];
    const formatter = new Intl.RelativeTimeFormat(toStandardLocale(locale), { numeric: 'auto' });
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

export function formatTimestamp(date, locale = 'en-us', { seconds = true } = {}) {
    // Ensure `normalizeDate` returns a valid Date object
    date = normalizeDate(date);

    if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
        console.error('Invalid date object:', date);
        return 'Invalid Date';
    }

    const options = {
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
        return new Intl.DateTimeFormat(toStandardLocale(locale), options).format(date);
    } catch (error) {
        console.error('Error formatting date:', error);
        return 'Invalid Date';
    }
}

function normalizeDate(date) {
    let normalizedDate;

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
 * @returns {number | null}
 */
export function utcStartOfDaySecondsFromDateInput(isoDate) {
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
