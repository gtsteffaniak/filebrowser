export function getFileExtension(filename) {
    const firstDotIndex = filename.indexOf('.');
    if (firstDotIndex === -1) {
      return filename; // No dot found, return the original filename
    }
    return filename.substring(firstDotIndex);
  }
