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
    const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });

    for (let interval of intervals) {
        const count = Math.floor(diffInSeconds / interval.seconds);
        if (count > 0) {
            return formatter.format(-count, interval.label);
        }
    }
    return 'just now';
}

export function formatTimestamp(date, locale = 'en-us') {
    date = normalizeDate(date);
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
    const dateFormatter = new Intl.DateTimeFormat(locale, dateOptions);
    const timeFormatter = new Intl.DateTimeFormat(locale, timeOptions);

    // Extract date and time components
    const dateParts = dateFormatter.formatToParts(date);
    const timeParts = timeFormatter.formatToParts(date);

    // Construct formatted timestamp
    const dateMap = new Map(dateParts.map(part => [part.type, part.value]));
    const timeMap = new Map(timeParts.map(part => [part.type, part.value]));

    const formattedDate = locale.includes('en')
        ? `${dateMap.get('month')}/${dateMap.get('day')}/${dateMap.get('year')}`
        : `${dateMap.get('day')}/${dateMap.get('month')}/${dateMap.get('year')}`;

    // Time formatting: hh:mm:ss
    const formattedTime = `${timeMap.get('hour')}:${timeMap.get('minute')}:${timeMap.get('second')}`;

    // Combine date and time
    return `${formattedDate} ${formattedTime}`;
}

function normalizeDate(date) {
    if (typeof date === 'string') {
        date = new Date(date);
    } else if (typeof date === 'number') {
        date = new Date(date * 1000);
    } else {
        if (!(date instanceof Date) || isNaN(date.getTime())) {
            throw new Error("Invalid date provided");
        }
    }

    // Convert the time to milliseconds if it's in seconds
    if (date < 1e12) {
        date *= 1000;
    }
    // Create a Date object from the timestamp
    date = new Date(date);
    // Format the date as an ISO 8601 string with timezone offset
    const formattedDate = date.toISOString().replace('Z', getTimeZoneOffset(date));
    return formattedDate;
}

function getTimeZoneOffset(date) {
    const offset = -date.getTimezoneOffset();
    const sign = offset >= 0 ? '+' : '-';
    const pad = (num) => String(num).padStart(2, '0');
    const hours = pad(Math.floor(Math.abs(offset) / 60));
    const minutes = pad(Math.abs(offset) % 60);
    return `${sign}${hours}:${minutes}`;
}

export default {
    formatTimestamp,
    fromNow,
};
