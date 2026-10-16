/** Shared shapes/defaults for the runtime media settings (settings/media + prompts). */

import { patchMediaSettings } from '@/api/settings.js';

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
      // 720p balanced / 480p data saver: matches the backend defaults.
      balanced: emptyTranscodeProfile(720, 3000),
      datasaver: emptyTranscodeProfile(480, 1500),
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

/** Enabled transcode profile ids in player-menu order. */
export function enabledTranscodeProfileIds(media) {
  const profiles = media?.transcode?.profiles || {};
  return MEDIA_TRANSCODE_PROFILES
    .filter((profile) => profiles[profile.id]?.enabled)
    .map((profile) => profile.id);
}

/**
 * Normalize the media settings payload for playback. The settings API returns
 * the full object to admins and a player-only subset to everyone else, so both
 * shapes collapse to the same fields here.
 */
export function normalizeMediaPlayerSettings(data) {
  const transcode = data?.transcode || {};
  return {
    enabled: Boolean(data?.enabled ?? transcode.enabled),
    profiles: Array.isArray(data?.profiles) ? data.profiles : enabledTranscodeProfileIds(data),
    maxConcurrentPerUser: Number(data?.maxConcurrentPerUser ?? transcode.maxConcurrentPerUser) || 1,
    hardwareAcceleration: Boolean(data?.hardwareAcceleration),
  };
}

function deleteAtPath(target, path) {
  const parts = String(path).split('.');
  let node = target;
  for (let i = 0; i < parts.length - 1; i += 1) {
    if (!node || typeof node !== 'object') {
      return;
    }
    node = node[parts[i]];
  }
  if (node && typeof node === 'object') {
    delete node[parts[parts.length - 1]];
  }
}

/**
 * Remove config-locked leaf paths from a partial media settings patch.
 *
 * The backend rejects any patch that touches a path pinned in `config.yaml`, and
 * the settings sections submit whole sub-objects (e.g. the full `transcode`
 * object). Without this, locking one field would make every sibling unsavable.
 */
export function stripLockedMediaPaths(patch, lockedFromConfigPaths = []) {
  if (!patch) {
    return patch;
  }
  const locked = Array.isArray(lockedFromConfigPaths) ? lockedFromConfigPaths : [];
  if (locked.length === 0) {
    return patch;
  }
  const stripped = JSON.parse(JSON.stringify(patch));
  for (const path of locked) {
    deleteAtPath(stripped, path);
  }
  return stripped;
}

/** True when a (possibly nested) patch still carries at least one leaf value. */
function hasPatchContent(value) {
  if (value === null || typeof value !== 'object') {
    return true;
  }
  return Object.values(value).some(hasPatchContent);
}

/**
 * Patch media settings, skipping paths locked from the config file.
 * @returns {Promise<{ skipped: boolean, media?: object }>}
 */
export async function saveMediaSettingsPatch(partial, lockedFromConfigPaths = []) {
  const patch = stripLockedMediaPaths(partial, lockedFromConfigPaths);
  if (!hasPatchContent(patch)) {
    return { skipped: true };
  }
  const media = await patchMediaSettings(patch);
  return { skipped: false, media: mergeMediaSettings(media) };
}
