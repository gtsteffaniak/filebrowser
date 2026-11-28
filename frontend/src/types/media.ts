import type { ListingResponse } from "./files.ts";
import type { TranscodeProfile } from "./transcode.ts";

export interface TranscodeProfileSettings {
  enabled: boolean;
  maxResolution: number;
  avgBitrateKbps: number;
  encoderBackend: string;
}

export interface MediaTranscodeSettings {
  enabled: boolean;
  maxConcurrent: number;
  maxConcurrentPerUser: number;
  cacheMaxSizeMB: number;
  cacheRetentionH: number;
  encoderBackend: string;
  readRate: number;
  profiles: Record<TranscodeProfile, TranscodeProfileSettings>;
}

export interface MediaSettings {
  convert: {
    imagePreview: Record<string, boolean>;
    videoPreview: Record<string, boolean>;
  };
  debug: boolean;
  hardwareAcceleration: boolean;
  accelDevice: string;
  alwaysUseFfmpeg: boolean;
  alwaysUseOnlyOffice: boolean;
  transcode: MediaTranscodeSettings;
}

/** Every field optional, except the flag maps (`convert.*Preview`) */
export type DeepPartial<T> = T extends Record<string, boolean> ? T : { [K in keyof T]?: DeepPartial<T[K]> };

export interface AccelDeviceOption {
  id: string;
  label?: string;
}

export interface EncoderBackendOption {
  id: string;
  label?: string;
}

/** Admin payload of GET settings/media: the settings plus the acceleration and config-lock metadata */
export type MediaSettingsResponse = DeepPartial<MediaSettings> & {
  accelDeviceOptions?: AccelDeviceOption[] | null;
  autoAccelDevice?: string;
  encoderBackendOptions?: EncoderBackendOption[] | null;
  lockedFromConfigPaths?: string[] | null;
  lockMessage?: string;
};

/** Runtime media state for all UI (player, listings, previews) */
export interface SharedMediaSettings {
  enabled: boolean;
  profiles: string[];
  maxConcurrentPerUser: number;
  /** Per image preview type (like heic, jxl) conversion active on the server */
  imagePreview: Partial<Record<'heic' | 'jxl' | 'jpeg', boolean>>;
}

export type PlaybackMode = 'single' | 'sequential' | 'shuffle';

export type PlaybackLoop = 'off' | 'all' | 'single';

/** from backend utils.SubtitleTrack. */
export interface SubtitleTrack {
  name: string;
  language?: string;
  srclang?: string;
  title?: string;
  index?: number;
  codec?: string;
  embedded: boolean;
}

/** from backend iteminfo.MediaMetadata. */
export interface MediaMetadata {
  title?: string;
  artist?: string;
  album?: string;
  year?: number;
  genre?: string;
  track?: number;
  duration?: number;
  videoCodec?: string;
  audioCodec?: string;
  container?: string;
  albumArt?: string;
  hasLyrics?: boolean;
}

/** GET /api/media/metadata: a directory with the metadata of its items, or a single file */
export type MediaResource = ListingResponse & {
  metadata?: MediaMetadata;
  subtitles?: SubtitleTrack[];
  hasPreview?: boolean;
};

export interface Lyrics {
  lyrics: string;
  format: string;
}
