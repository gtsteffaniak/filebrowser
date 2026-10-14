/** Shared shapes/defaults for the runtime media settings (settings/media + prompts). */

export const MEDIA_TRANSCODE_PROFILES = [
  { id: 'quality', label: 'general.quality' },
  { id: 'balanced', label: 'general.balanced' },
  { id: 'datasaver', label: 'general.dataSaver' },
];

export function emptyTranscodeProfile(maxResolution = 0, avgBitrateKbps = 0) {
  return { enabled: true, maxResolution, avgBitrateKbps };
}

export function emptyMediaTranscode() {
  return {
    enabled: false,
    maxConcurrent: 2,
    maxConcurrentPerUser: 1,
    cacheMaxSizeMB: 10240,
    cacheRetentionH: 24,
    profiles: {
      quality: emptyTranscodeProfile(0, 0),
      balanced: emptyTranscodeProfile(1080, 0),
      // 720p data saver: 1500 kbps matches the backend default.
      datasaver: emptyTranscodeProfile(720, 1500),
    },
  };
}

export function emptyMediaSettings() {
  return {
    convert: { imagePreview: {}, videoPreview: {} },
    debug: false,
    extractEmbeddedSubtitles: false,
    hardwareAcceleration: false,
    transcode: emptyMediaTranscode(),
  };
}

/** Overlay a server response onto the defaults so partial payloads stay usable. */
export function mergeMediaSettings(loaded) {
  const base = emptyMediaSettings();
  const value = loaded || {};
  return {
    ...base,
    ...value,
    convert: {
      imagePreview: { ...base.convert.imagePreview, ...(value.convert?.imagePreview || {}) },
      videoPreview: { ...base.convert.videoPreview, ...(value.convert?.videoPreview || {}) },
    },
    transcode: {
      ...base.transcode,
      ...(value.transcode || {}),
      profiles: {
        quality: { ...base.transcode.profiles.quality, ...(value.transcode?.profiles?.quality || {}) },
        balanced: { ...base.transcode.profiles.balanced, ...(value.transcode?.profiles?.balanced || {}) },
        datasaver: { ...base.transcode.profiles.datasaver, ...(value.transcode?.profiles?.datasaver || {}) },
      },
    },
  };
}

/** Event emitted after a media settings prompt saves, so the section reloads. */
export const MEDIA_SETTINGS_CHANGED_EVENT = 'mediaSettingsChanged';
