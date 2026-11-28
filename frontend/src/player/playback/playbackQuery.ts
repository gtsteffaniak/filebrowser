import type { TranscodeProfile } from "@/types/transcode.ts";
import { getObjectProperty, setObjectProperty } from "@/utils/object.js";

/** A URL query value, as vue-router parses it (a key may be absent or repeated) */
type QueryValue = string | null | (string | null)[] | undefined;

/** Route query that the playback helpers read and rewrite */
export type PlaybackQuery = Record<string, QueryValue>;

export interface PlaybackQueryPatch {
  time?: number | null;
  transcodeMode?: TranscodeProfile | null;
}

/** Query key for shareable playback position (`HH:MM:SS`) */
export const PLAYBACK_TIME_QUERY_KEY = 'time';

/** Legacy numeric seconds key; still accepted when reading URLs */
export const LEGACY_PLAYBACK_TIME_QUERY_KEY = 't';

/** Transcode profile in shareable links (`quality` | `balanced` | `datasaver`) */
export const PLAYBACK_TRANSCODE_QUERY_KEY = 'transcode';

const VALID_TRANSCODE_MODES: ReadonlySet<string> = new Set<TranscodeProfile>(["quality", "balanced", "datasaver"]);

function isTranscodeProfile(value: string): value is TranscodeProfile {
  return VALID_TRANSCODE_MODES.has(value);
}

function isValidClockPart(part: string): boolean {
  if (!part) {
    return false;
  }
  let sawDot = false;
  for (let i = 0; i < part.length; i += 1) {
    const ch = part.charAt(i);
    if (ch >= '0' && ch <= '9') {
      continue;
    }
    if (ch === '.' && !sawDot) {
      sawDot = true;
      continue;
    }
    return false;
  }
  return true;
}

function parsePlainSecondsString(str: string): number | null {
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
 * @returns seconds, or null when the value is not a valid clock time
 */
export function parseClockTimeString(raw: unknown, options: { requireHours?: boolean } = {}): number | null {
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

  const values: number[] = [];
  for (const partRaw of parts) {
    const part = partRaw.trim();
    if (!part || !isValidClockPart(part)) {
      return null;
    }
    const value = Number(part);
    if (!Number.isFinite(value) || value < 0) {
      return null;
    }
    values.push(value);
  }

  // Three parts are HH:MM:SS, two are MM:SS; the length was validated above.
  const [hours = 0, minutes = 0, seconds = 0] = values.length === 3 ? values : [0, ...values];

  if (minutes >= 60 || seconds >= 60) {
    return null;
  }

  const total = (hours * 3600) + (minutes * 60) + seconds;
  return Number.isFinite(total) && total >= 0 ? total : null;
}

export function formatTimeForQuery(seconds: number): string {
  const total = Math.max(0, Math.round(seconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

export function parsePlaybackTimeFromQuery(query: Record<string, unknown> | undefined): number | null {
  if (!query) {
    return null;
  }
  const primary = getObjectProperty(query, PLAYBACK_TIME_QUERY_KEY);
  if (primary !== null && primary !== undefined && primary !== '') {
    return parseClockTimeString(primary, { requireHours: true });
  }
  const legacy = getObjectProperty(query, LEGACY_PLAYBACK_TIME_QUERY_KEY);
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

export function parseTranscodeModeFromQuery(query: Record<string, unknown> | undefined): TranscodeProfile | null {
  if (!query) {
    return null;
  }
  const raw = getObjectProperty(query, PLAYBACK_TRANSCODE_QUERY_KEY);
  if (raw === null || raw === undefined || raw === '') {
    return null;
  }
  const mode = String(raw).toLowerCase();
  return isTranscodeProfile(mode) ? mode : null;
}

export function buildPlaybackQueryPatch(query: PlaybackQuery = {}, patch: PlaybackQueryPatch = {}): PlaybackQuery {
  const {
    t: _legacyT,
    time: queryTime,
    transcode: queryTranscode,
    ...rest
  } = query;
  let next: PlaybackQuery = { ...rest };
  let timeValue: QueryValue = queryTime;
  if (timeValue === undefined || timeValue === null || timeValue === '') {
    const parsedTime = parsePlaybackTimeFromQuery(query);
    timeValue = parsedTime === null ? undefined : formatTimeForQuery(parsedTime);
  }
  let transcodeValue: QueryValue = queryTranscode;

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
    if (transcodeMode && isTranscodeProfile(transcodeMode)) {
      transcodeValue = transcodeMode;
    } else {
      transcodeValue = undefined;
    }
  }
  if (timeValue !== undefined) {
    next = setObjectProperty(next, PLAYBACK_TIME_QUERY_KEY, timeValue) as PlaybackQuery;
  }
  if (transcodeValue !== undefined) {
    next = setObjectProperty(next, PLAYBACK_TRANSCODE_QUERY_KEY, transcodeValue) as PlaybackQuery;
  }
  return next;
}

export function playbackQueryChanged(prev: Record<string, unknown> = {}, next: Record<string, unknown> = {}): boolean {
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

/** True when two routes differ only in the playback query, so the file doesn't refetch again */
export function isPlaybackOnlyRouteChange(
  to: { path: string; hash?: string; query?: PlaybackQuery },
  from: { path: string; hash?: string; query?: PlaybackQuery },
): boolean {
  if (to.path !== from.path || (to.hash ?? '') !== (from.hash ?? '')) {
    return false;
  }
  const strip = (query: PlaybackQuery = {}) =>
    JSON.stringify(buildPlaybackQueryPatch(query, { time: null, transcodeMode: null }));
  return strip(to.query) === strip(from.query);
}
