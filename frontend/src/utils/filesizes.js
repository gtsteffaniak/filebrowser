export function getHumanReadableFilesize(fileSizeBytes) {
    let size;   // size in the specified unit
    let unit;  // the unit name
    unit = 'bytes';
    size = fileSizeBytes;

    switch (true) {
        case fileSizeBytes < 1024:
            break;
        case fileSizeBytes < 1024 ** 2:  // 1 KB - 1 MB
            size = fileSizeBytes / 1024;
            unit = 'KB';
            break;
        case fileSizeBytes < 1024 ** 3:  // 1 MB - 1 GB
            size = fileSizeBytes / (1024 ** 2);
            unit = 'MB';
            break;
        case fileSizeBytes < 1024 ** 4:  // 1 GB - 1 TB
            size = fileSizeBytes / (1024 ** 3);
            unit = 'GB';
            break;
        case fileSizeBytes < 1024 ** 5:  // 1 TB - 1 PB
            size = fileSizeBytes / (1024 ** 4);
            unit = 'TB';
            break;
        default:  // >= 1 PB
            size = fileSizeBytes / (1024 ** 5);
            unit = 'PB';
            break;
    }
    return `${size.toFixed(1)} ${unit}`;
}