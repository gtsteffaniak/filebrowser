/** Query key for shareable playback position (`HH:MM:SS`). */
export const PLAYBACK_TIME_QUERY_KEY = 'time';

/** Legacy numeric seconds key; still accepted when reading URLs. */
export const LEGACY_PLAYBACK_TIME_QUERY_KEY = 't';

/** Transcode profile in shareable links (`quality` | `datasaver`). */
export const PLAYBACK_TRANSCODE_QUERY_KEY = 'transcode';

const VALID_TRANSCODE_MODES = new Set(['quality', 'datasaver']);

/** @param {string} str */
function parsePlainSecondsString(str) {
  if (str.includes(':')) {
    return null;
  }
  let i = 0;
  let intPart = 0;
  while (i < str.length && str.charAt(i) >= '0' && str.charAt(i) <= '9') {
    intPart = (intPart * 10) + (str.charCodeAt(i) - 48);
    i += 1;
  }
  if (i === 0) {
    return null;
  }
  let frac = 0;
  let fracDiv = 1;
  if (i < str.length && str.charAt(i) === '.') {
    i += 1;
    while (i < str.length && str.charAt(i) >= '0' && str.charAt(i) <= '9') {
      frac = (frac * 10) + (str.charCodeAt(i) - 48);
      fracDiv *= 10;
      i += 1;
    }
  }
  if (i !== str.length) {
    return null;
  }
  return intPart + (frac / fracDiv);
}

/**
 * Parse `HH:MM:SS` or `MM:SS` clock time.
 * @param {unknown} raw
 * @param {{ requireHours?: boolean }} [options]
 * @returns {number | null} seconds
 */
export function parseClockTimeString(raw, options = {}) {
  if (raw === null || raw === undefined || raw === '') {
    return null;
  }
  const str = String(raw).trim();
  if (!str) {
    return null;
  }

  const parts = str.split(':');
  if (options.requireHours) {
    if (parts.length !== 3) {
      return null;
    }
  } else if (parts.length !== 2 && parts.length !== 3) {
    return null;
  }

  /** @type {number[]} */
  const values = [];
  for (let i = 0; i < parts.length; i += 1) {
    const part = parts[i].trim();
    if (!part || !/^\d+(\.\d+)?$/.test(part)) {
      return null;
    }
    const value = Number(part);
    if (!Number.isFinite(value) || value < 0) {
      return null;
    }
    values.push(value);
  }

  let hours = 0;
  let minutes = 0;
  let seconds = 0;
  if (values.length === 3) {
    [hours, minutes, seconds] = values;
  } else {
    [minutes, seconds] = values;
  }

  if (minutes >= 60 || seconds >= 60) {
    return null;
  }

  const total = (hours * 3600) + (minutes * 60) + seconds;
  return Number.isFinite(total) && total >= 0 ? total : null;
}

/**
 * @param {number} seconds
 * @returns {string}
 */
export function formatTimeForQuery(seconds) {
  const total = Math.max(0, Math.round(seconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

/**
 * @param {Record<string, unknown> | undefined} query
 * @returns {number | null}
 */
export function parsePlaybackTimeFromQuery(query) {
  if (!query) {
    return null;
  }
  const primary = query[PLAYBACK_TIME_QUERY_KEY];
  if (primary !== null && primary !== undefined && primary !== '') {
    return parseClockTimeString(primary, { requireHours: true });
  }
  const legacy = query[LEGACY_PLAYBACK_TIME_QUERY_KEY];
  if (legacy !== null && legacy !== undefined && legacy !== '') {
    const str = String(legacy).trim();
    const clock = parseClockTimeString(str);
    if (clock !== null) {
      return clock;
    }
    const plain = parsePlainSecondsString(str);
    return plain !== null && Number.isFinite(plain) && plain >= 0 ? plain : null;
  }
  return null;
}

/**
 * @param {Record<string, unknown> | undefined} query
 * @returns {'quality' | 'datasaver' | null}
 */
export function parseTranscodeModeFromQuery(query) {
  if (!query) {
    return null;
  }
  const raw = query[PLAYBACK_TRANSCODE_QUERY_KEY];
  if (raw === null || raw === undefined || raw === '') {
    return null;
  }
  const mode = String(raw).toLowerCase();
  return VALID_TRANSCODE_MODES.has(mode) ? mode : null;
}

/**
 * @param {Record<string, string | undefined>} query
 * @param {{ time?: number | null, transcodeMode?: string | null }} [patch]
 * @returns {Record<string, string | undefined>}
 */
export function buildPlaybackQueryPatch(query = {}, patch = {}) {
  const {
    t: _legacyT,
    time: queryTime,
    transcode: queryTranscode,
    ...rest
  } = query;
  /** @type {Record<string, string | undefined>} */
  const next = { ...rest };
  let timeValue = queryTime;
  if (timeValue === undefined) {
    const parsedTime = parsePlaybackTimeFromQuery(query);
    timeValue = parsedTime === null ? undefined : formatTimeForQuery(parsedTime);
  }
  let transcodeValue = queryTranscode;

  if (Object.hasOwn(patch, 'time')) {
    const { time } = patch;
    if (time !== null && time !== undefined && Number.isFinite(time) && time > 0.05) {
      timeValue = formatTimeForQuery(time);
    } else {
      timeValue = undefined;
    }
  }

  if (Object.hasOwn(patch, 'transcodeMode')) {
    const { transcodeMode } = patch;
    if (transcodeMode && VALID_TRANSCODE_MODES.has(transcodeMode)) {
      transcodeValue = transcodeMode;
    } else {
      transcodeValue = undefined;
    }
  }

  if (timeValue !== undefined) {
    next[PLAYBACK_TIME_QUERY_KEY] = timeValue;
  }
  if (transcodeValue !== undefined) {
    next[PLAYBACK_TRANSCODE_QUERY_KEY] = transcodeValue;
  }
  return next;
}

/**
 * @param {Record<string, unknown> | undefined} prev
 * @param {Record<string, unknown> | undefined} next
 * @returns {boolean}
 */
export function playbackQueryChanged(prev = {}, next = {}) {
  const prevSec = parsePlaybackTimeFromQuery(prev);
  const nextSec = parsePlaybackTimeFromQuery(next);
  const prevRounded = prevSec === null ? null : Math.round(prevSec);
  const nextRounded = nextSec === null ? null : Math.round(nextSec);
  const prevTranscode = parseTranscodeModeFromQuery(prev);
  const nextTranscode = parseTranscodeModeFromQuery(next);
  return (
    prevRounded !== nextRounded
    || prevTranscode !== nextTranscode
  );
}

/** @deprecated use formatTimeForQuery */
export function formatDurationForQuery(seconds) {
  return formatTimeForQuery(seconds);
}

/** @deprecated use formatTimeForQuery */
export function formatPlaybackTimeQueryValue(seconds) {
  return formatTimeForQuery(seconds);
}
