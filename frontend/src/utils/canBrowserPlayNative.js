const CONTAINER_MIME = {
  mkv: 'video/x-matroska',
  webm: 'video/webm',
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  mov: 'video/mp4',
  avi: 'video/avi',
  wmv: 'video/x-ms-wmv',
  flv: 'video/x-flv',
  ts: 'video/mp2t',
  m2ts: 'video/mp2t',
  ogv: 'video/ogg',
};

const FFPROBE_TO_MIME = {
  h264: 'avc1.42E01E',
  avc: 'avc1.42E01E',
  avc1: 'avc1.42E01E',
  hevc: 'hvc1.1.6.L93.B0',
  h265: 'hvc1.1.6.L93.B0',
  vp9: 'vp9',
  av1: 'av01.0.05M.08',
  aac: 'mp4a.40.2',
  mp3: 'mp3',
  vorbis: 'vorbis',
  opus: 'opus',
  ac3: 'ac-3',
  eac3: 'ec-3',
  flac: 'flac',
};

function mimeCodec(name) {
  if (!name) {
    return '';
  }
  return FFPROBE_TO_MIME[name.toLowerCase()] || name.toLowerCase();
}

function extensionFromName(fileName) {
  if (!fileName || typeof fileName !== 'string') {
    return '';
  }
  const dot = fileName.lastIndexOf('.');
  if (dot < 0) {
    return '';
  }
  return fileName.slice(dot + 1).toLowerCase();
}

function containerMimeFromFileName(fileName, mimeType) {
  const ext = extensionFromName(fileName);
  if (ext && CONTAINER_MIME[ext]) {
    return CONTAINER_MIME[ext];
  }
  if (mimeType && (mimeType.startsWith('video/') || mimeType.startsWith('audio/'))) {
    return mimeType;
  }
  return 'video/mp4';
}

function buildCodecMime(containerMime, videoCodec, audioCodec) {
  const parts = [];
  const v = mimeCodec(videoCodec);
  const a = mimeCodec(audioCodec);
  if (v) {
    parts.push(v);
  }
  if (a) {
    parts.push(a);
  }
  if (parts.length === 0) {
    return containerMime;
  }
  return `${containerMime}; codecs="${parts.join(', ')}"`;
}

function probeCanPlayType(mime) {
  if (!mime || typeof document === 'undefined') {
    return null;
  }
  const video = document.createElement('video');
  const support = video.canPlayType(mime);
  if (support === 'probably') {
    return true;
  }
  if (support === 'maybe') {
    return null;
  }
  if (support === '') {
    return false;
  }
  return null;
}

/**
 * Returns true when the browser can likely play the source natively,
 * false when transcode is recommended, or null when unknown.
 *
 * MKV and other containers are evaluated by codec + container via canPlayType,
 * not by extension alone (Chrome supports H.264 in MKV; Firefox often does not).
 */
export function canBrowserPlayNative({ videoCodec, audioCodec, mimeType, fileName } = {}) {
  const containerMime = containerMimeFromFileName(fileName, mimeType);
  const ext = extensionFromName(fileName);

  if (videoCodec || audioCodec) {
    const codecMime = buildCodecMime(containerMime, videoCodec, audioCodec);
    const canPlay = probeCanPlayType(codecMime);
    if (canPlay === true || canPlay === false) {
      return canPlay;
    }
    if (
      typeof MediaSource !== 'undefined'
      && typeof MediaSource.isTypeSupported === 'function'
      && MediaSource.isTypeSupported(codecMime)
    ) {
      return true;
    }
  }

  const containerOnly = probeCanPlayType(containerMime);
  if (containerOnly === true) {
    return true;
  }
  if (containerOnly === false) {
    if (!videoCodec && !audioCodec && (ext === 'mp4' || ext === 'm4v' || ext === 'webm')) {
      return null;
    }
    return false;
  }

  if (ext === 'mp4' || ext === 'm4v') {
    return null;
  }
  if (ext === 'mkv' || ext === 'webm') {
    return null;
  }
  // Containers browsers never play natively (WMV, AVI, FLV, …).
  if (ext === 'wmv' || ext === 'avi' || ext === 'flv' || ext === 'vob') {
    return false;
  }

  return null;
}

export function needsTranscodeFirst({ videoCodec, audioCodec, mimeType, fileName } = {}) {
  const result = canBrowserPlayNative({ videoCodec, audioCodec, mimeType, fileName });
  return result === false;
}
