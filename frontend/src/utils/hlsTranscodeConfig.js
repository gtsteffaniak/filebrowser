/** Default on-demand HLS delivery — mirrors backend ffmpeg.DefaultOnDemandHLSConfig(). */
export const DEFAULT_HLS_TRANSCODE_CONFIG = Object.freeze({
  mode: 'on-demand',
  segmentDurationSec: 4,
  playerBufferSegments: 3,
  warmPlaylistSegments: 3,
});

const HEADER_MODE = 'X-HLS-Mode';
const HEADER_SEGMENT_DUR = 'X-HLS-Segment-Duration-Sec';
const HEADER_BUFFER_SEGMENTS = 'X-HLS-Player-Buffer-Segments';
const PLAYLIST_CONFIG_RE = /^#EXT-X-FB-CONFIG:mode=([^;]+);seg=([^;]+);buffer=(\d+)\s*$/m;

function parsePositiveFloat(value) {
  const n = Number.parseFloat(value);
  return Number.isFinite(n) && n > 0 ? n : null;
}

function parsePositiveInt(value) {
  const n = Number.parseInt(value, 10);
  return Number.isFinite(n) && n > 0 ? n : null;
}

/** Read one header from a fetch Response or xhr-like object. */
function readHeader(source, name) {
  if (!source) {
    return null;
  }
  if (typeof source.get === 'function') {
    return source.get(name);
  }
  if (typeof source.getResponseHeader === 'function') {
    return source.getResponseHeader(name);
  }
  return null;
}

/** Parse #EXT-X-FB-CONFIG from playlist text (fallback when headers are unavailable). */
export function parseHLSConfigFromPlaylist(playlistText) {
  if (!playlistText || typeof playlistText !== 'string') {
    return null;
  }
  const match = playlistText.match(PLAYLIST_CONFIG_RE);
  if (!match) {
    return null;
  }
  const segmentDurationSec = parsePositiveFloat(match[2]);
  const playerBufferSegments = parsePositiveInt(match[3]);
  if (segmentDurationSec === null || playerBufferSegments === null) {
    return null;
  }
  return {
    mode: match[1] || DEFAULT_HLS_TRANSCODE_CONFIG.mode,
    segmentDurationSec,
    playerBufferSegments,
  };
}

/** Parse delivery config from response headers (playlist prefetch or xhr). */
export function parseHLSConfigFromHeaders(headerSource) {
  if (!headerSource) {
    return null;
  }
  const segmentDurationSec = parsePositiveFloat(readHeader(headerSource, HEADER_SEGMENT_DUR));
  const playerBufferSegments = parsePositiveInt(readHeader(headerSource, HEADER_BUFFER_SEGMENTS));
  if (segmentDurationSec === null || playerBufferSegments === null) {
    return null;
  }
  const mode = readHeader(headerSource, HEADER_MODE) || DEFAULT_HLS_TRANSCODE_CONFIG.mode;
  return { mode, segmentDurationSec, playerBufferSegments };
}

/** Merge partial config with defaults. */
export function normalizeHLSConfig(partial = {}) {
  const segmentDurationSec = parsePositiveFloat(partial.segmentDurationSec)
    ?? DEFAULT_HLS_TRANSCODE_CONFIG.segmentDurationSec;
  const playerBufferSegments = parsePositiveInt(partial.playerBufferSegments)
    ?? DEFAULT_HLS_TRANSCODE_CONFIG.playerBufferSegments;
  const warmPlaylistSegments = parsePositiveInt(partial.warmPlaylistSegments)
    ?? DEFAULT_HLS_TRANSCODE_CONFIG.warmPlaylistSegments;
  return {
    mode: partial.mode || DEFAULT_HLS_TRANSCODE_CONFIG.mode,
    segmentDurationSec,
    playerBufferSegments,
    warmPlaylistSegments,
  };
}

/** Player tuning derived from server delivery config. */
export function hlsPlayerTuning(config) {
  const normalized = normalizeHLSConfig(config);
  const bufferAheadSec = normalized.segmentDurationSec * normalized.playerBufferSegments;
  return {
    ...normalized,
    bufferAheadSec,
    /** Seek jumps larger than one segment invalidate session cache ping. */
    seekJumpSec: normalized.segmentDurationSec,
  };
}
