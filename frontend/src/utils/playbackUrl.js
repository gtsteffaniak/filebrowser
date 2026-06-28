/** Query key for shareable playback position (human-readable duration). */
export const PLAYBACK_TIME_QUERY_KEY = 'time';

/** Legacy numeric seconds key; still accepted when reading URLs. */
export const LEGACY_PLAYBACK_TIME_QUERY_KEY = 't';

/** Transcode profile in shareable links (`quality` | `datasaver`). */
export const PLAYBACK_TRANSCODE_QUERY_KEY = 'transcode';

const VALID_TRANSCODE_MODES = new Set(['quality', 'datasaver']);

/** @param {string} ch */
function isDigit(ch) {
  return ch >= '0' && ch <= '9';
}

/** @param {string} str @param {number} index */
function charAt(str, index) {
  return str.charAt(index);
}

/** @param {string} str */
function isDurationTokenString(str) {
  let i = 0;
  const len = str.length;
  while (i < len) {
    const start = i;
    while (i < len && isDigit(charAt(str, i))) {
      i += 1;
    }
    if (i === start || i >= len) {
      return false;
    }
    const unit = charAt(str, i);
    if (unit !== 'h' && unit !== 'm' && unit !== 's') {
      return false;
    }
    i += 1;
  }
  return i > 0;
}

/** @param {string} str */
function parsePlainSecondsString(str) {
  let i = 0;
  let intPart = 0;
  while (i < str.length && isDigit(charAt(str, i))) {
    intPart = (intPart * 10) + (str.charCodeAt(i) - 48);
    i += 1;
  }
  if (i === 0) {
    return null;
  }
  let frac = 0;
  let fracDiv = 1;
  if (i < str.length && charAt(str, i) === '.') {
    i += 1;
    while (i < str.length && isDigit(charAt(str, i))) {
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

/** @param {string} str @returns {number | null} */
function parseDurationTokens(str) {
  let i = 0;
  let total = 0;
  while (i < str.length) {
    const start = i;
    while (i < str.length && isDigit(charAt(str, i))) {
      i += 1;
    }
    if (i === start || i >= str.length) {
      return null;
    }
    const num = parseInt(str.slice(start, i), 10);
    const unit = charAt(str, i);
    i += 1;
    if (unit === 'h') {
      total += num * 3600;
    } else if (unit === 'm') {
      total += num * 60;
    } else if (unit === 's') {
      total += num;
    } else {
      return null;
    }
  }
  return total;
}

/**
 * Parse a human-readable duration (`2h3m4s`, `3m`, `30s`, …) or plain seconds.
 * @param {unknown} raw
 * @returns {number | null} seconds
 */
export function parseDurationString(raw) {
  if (raw === null || raw === undefined || raw === '') {
    return null;
  }
  const str = String(raw).trim().toLowerCase();
  if (!str) {
    return null;
  }

  const plain = parsePlainSecondsString(str);
  if (plain !== null) {
    return Number.isFinite(plain) && plain >= 0 ? plain : null;
  }

  if (!isDurationTokenString(str)) {
    return null;
  }

  return parseDurationTokens(str);
}

/**
 * @param {number} seconds
 * @returns {string}
 */
export function formatDurationForQuery(seconds) {
  const total = Math.max(0, Math.round(seconds));
  if (total === 0) {
    return '0s';
  }
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  let out = '';
  if (h > 0) {
    out += `${h}h`;
  }
  if (m > 0) {
    out += `${m}m`;
  }
  if (s > 0 || out === '') {
    out += `${s}s`;
  }
  return out;
}

/**
 * @param {Record<string, unknown> | undefined} query
 * @returns {number | null}
 */
export function parsePlaybackTimeFromQuery(query) {
  if (!query) {
    return null;
  }
  const primary = query.time;
  if (primary !== null && primary !== undefined && primary !== '') {
    return parseDurationString(primary);
  }
  const legacy = query.t;
  if (legacy !== null && legacy !== undefined && legacy !== '') {
    return parseDurationString(legacy);
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
  const raw = query.transcode;
  if (raw === null || raw === undefined || raw === '') {
    return null;
  }
  const mode = String(raw).toLowerCase();
  return VALID_TRANSCODE_MODES.has(mode) ? mode : null;
}

/** @deprecated use formatDurationForQuery */
export function formatPlaybackTimeQueryValue(seconds) {
  return formatDurationForQuery(seconds);
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
    timeValue = parsedTime === null ? undefined : formatDurationForQuery(parsedTime);
  }
  let transcodeValue = queryTranscode;

  if (Object.hasOwn(patch, 'time')) {
    const { time } = patch;
    if (time !== null && time !== undefined && Number.isFinite(time) && time > 0.05) {
      timeValue = formatDurationForQuery(time);
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
    next.time = timeValue;
  }
  if (transcodeValue !== undefined) {
    next.transcode = transcodeValue;
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
