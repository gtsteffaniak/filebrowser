export function fromNow(date, locale) {
    if (locale === undefined) {
        locale = 'en-us';
    }
    if (locale == "") {
        locale = 'en-us';
    }
    console.log(locale)
    // Convert date to Date object if it is a string
    const inputDate = typeof date === 'string' ? new Date(date) : date;

    if (!(inputDate instanceof Date) || isNaN(inputDate.getTime())) {
        throw new Error("Invalid date provided");
    }

    const now = new Date();
    const diffInSeconds = Math.floor((now - inputDate) / 1000);
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
    // Convert date to Date object if it is a string
    const inputDate = typeof date === 'string' ? new Date(date) : date;

    if (!(inputDate instanceof Date) || isNaN(inputDate.getTime())) {
        throw new Error("Invalid date provided");
    }

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
    const dateParts = dateFormatter.formatToParts(inputDate);
    const timeParts = timeFormatter.formatToParts(inputDate);

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
