/**
 * Transcode/HLS diagnostic logging.
 * Always logs to console with the [transcode] prefix.
 * Set localStorage.transcodeDebug = '1' for verbose HLS event noise.
 */
export function isTranscodeDebugVerbose() {
  try {
    return localStorage.getItem('transcodeDebug') === '1';
  } catch {
    return false;
  }
}

export function transcodeLog(message, detail) {
  if (detail === undefined) {
    console.info('[transcode]', message);
    return;
  }
  console.info('[transcode]', message, detail);
}

export function transcodeWarn(message, detail) {
  if (detail === undefined) {
    console.warn('[transcode]', message);
    return;
  }
  console.warn('[transcode]', message, detail);
}

export function transcodeError(message, detail) {
  if (detail === undefined) {
    console.error('[transcode]', message);
    return;
  }
  console.error('[transcode]', message, detail);
}

/** @param {string} message @param {unknown} [detail] */
export function transcodeVerbose(message, detail) {
  if (!isTranscodeDebugVerbose()) {
    return;
  }
  transcodeLog(`[verbose] ${message}`, detail);
}
