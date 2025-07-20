import { toStandardLocale } from "../i18n";

export function fromNow(date, locale) {
    date = normalizeDate(date);
    const now = new Date();
    const diffInSeconds = Math.floor((now - date) / 1000);
    const intervals = [
        { label: 'year', seconds: 31536000 },
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
    for (let interval of intervals) {
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

export function formatTimestamp(date, locale = 'en-us') {
    // Ensure `normalizeDate` returns a valid Date object
    date = normalizeDate(date);

    if (!(date instanceof Date) || isNaN(date)) {
        console.error('Invalid date object:', date);
        return 'Invalid Date';
    }

    const standardLocale = toStandardLocale(locale);

    // Define options for formatting
    const dateOptions = {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric'
    };

    const timeOptions = {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
    };

    // Format date and time using locale
    const dateFormatter = new Intl.DateTimeFormat(standardLocale, dateOptions);
    const timeFormatter = new Intl.DateTimeFormat(standardLocale, timeOptions);

    try {
        // Extract date and time components
        const dateParts = dateFormatter.formatToParts(date);
        const timeParts = timeFormatter.formatToParts(date);

        // Construct formatted timestamp
        const dateMap = new Map(dateParts.map(part => [part.type, part.value]));
        const timeMap = new Map(timeParts.map(part => [part.type, part.value]));

        const formattedDate = standardLocale.includes('en')
            ? `${dateMap.get('month')}/${dateMap.get('day')}/${dateMap.get('year')}`
            : `${dateMap.get('day')}/${dateMap.get('month')}/${dateMap.get('year')}`;

        // Time formatting: hh:mm:ss
        const formattedTime = `${timeMap.get('hour')}:${timeMap.get('minute')}:${timeMap.get('second')}`;

        // Combine date and time
        return `${formattedDate} ${formattedTime}`;
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
    } else if (date instanceof Date && !isNaN(date.getTime())) {
        // It's already a valid Date object
        normalizedDate = date;
    } else {
        throw new Error("Invalid date provided");
    }

    return normalizedDate;
}

export default {
    formatTimestamp,
    fromNow,
};
