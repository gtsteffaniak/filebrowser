import { TRANSCODE_HLS_MODES, type TranscodeHlsMode } from "@/player/transcodeMode.ts";
import { getObjectProperty } from "@/utils/object.js";
import { parseTranscodeModeFromQuery } from "@/player/playbackQuery";

// Storage keys keep the legacy "plyr" prefix
const PLAYER_STORAGE_KEY = "plyr";
const CAPTION_SIZE_FIELD = "captionSize";
const DESKTOP_PANEL_KEY = "plyrShowDesktopPanel";
const MOBILE_LYRICS_KEY = "plyrShowMobileLyrics";
const ALBUM_ART_SIZE_KEY = "plyrAlbumArtSize";

export const CAPTION_SIZE_IDS = ["small", "medium", "large", "xlarge"] as const;
export type CaptionSize = (typeof CAPTION_SIZE_IDS)[number];

const CAPTION_SIZE_LABELS: Record<CaptionSize, string> = {
  small: "Small",
  medium: "Medium",
  large: "Large",
  xlarge: "Extra large",
};

export function isCaptionSize(value: unknown): value is CaptionSize {
  return typeof value === "string" && (CAPTION_SIZE_IDS as readonly string[]).includes(value);
}

export function getCaptionSizeLabel(size: string): string {
  return isCaptionSize(size) ? CAPTION_SIZE_LABELS[size] : CAPTION_SIZE_LABELS.medium;
}

export function getStoredCaptionSize(): CaptionSize {
  try {
    const raw = localStorage.getItem(PLAYER_STORAGE_KEY);
    if (!raw) {
      return "medium";
    }
    const id = getObjectProperty(JSON.parse(raw), CAPTION_SIZE_FIELD);
    if (isCaptionSize(id)) {
      return id;
    }
  } catch {
    /* ignore */
  }
  return "medium";
}

/** Merge into the player's JSON store without clobbering `captions`, `language`, etc. */
export function mergePlayerStorage(partial: Record<string, unknown>): void {
  try {
    let data: Record<string, unknown> = {};
    const raw = localStorage.getItem(PLAYER_STORAGE_KEY);
    if (raw) {
      try {
        const parsed: unknown = JSON.parse(raw);
        if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
          data = parsed as Record<string, unknown>;
        }
      } catch {
        data = {};
      }
    }
    Object.assign(data, partial);
    localStorage.setItem(PLAYER_STORAGE_KEY, JSON.stringify(data));
  } catch {
    /* ignore */
  }
}

export function setStoredCaptionSize(id: string): void {
  if (!isCaptionSize(id)) {
    return;
  }
  mergePlayerStorage({ [CAPTION_SIZE_FIELD]: id });
}

function safeRead(storage: () => Storage, key: string): string | null {
  try {
    return storage().getItem(key);
  } catch {
    return null;
  }
}

function safeWrite(storage: () => Storage, key: string, value: string): void {
  try {
    storage().setItem(key, value);
  } catch {
    /* ignore */
  }
}

export function readDesktopPanelOpen(): boolean {
  return safeRead(() => localStorage, DESKTOP_PANEL_KEY) === "1";
}

export function writeDesktopPanelOpen(open: boolean): void {
  safeWrite(() => localStorage, DESKTOP_PANEL_KEY, open ? "1" : "0");
}

export function readMobileLyricsOpen(): boolean {
  return safeRead(() => sessionStorage, MOBILE_LYRICS_KEY) === "1";
}

export function writeMobileLyricsOpen(open: boolean): void {
  safeWrite(() => sessionStorage, MOBILE_LYRICS_KEY, open ? "1" : "0");
}

export function readAlbumArtSize(): number {
  return Number.parseFloat(safeRead(() => sessionStorage, ALBUM_ART_SIZE_KEY) ?? "") || 24;
}

export function writeAlbumArtSize(size: number): void {
  safeWrite(() => sessionStorage, ALBUM_ART_SIZE_KEY, size.toString());
}

/** localStorage key for the preferred HLS transcode profile. */
export const TRANSCODE_PREFERENCE_STORAGE_KEY = "filebrowser.transcodeMode";

export const DEFAULT_TRANSCODE_MODE: TranscodeHlsMode = "quality";

function asHlsMode(value: string | null): TranscodeHlsMode | undefined {
  return TRANSCODE_HLS_MODES.find((known) => known === value);
}

export function loadPreferredTranscodeMode(): TranscodeHlsMode {
  return asHlsMode(safeRead(() => localStorage, TRANSCODE_PREFERENCE_STORAGE_KEY)) ?? DEFAULT_TRANSCODE_MODE;
}

/** URL transcode param wins; otherwise use localStorage (default quality). */
export function resolveTranscodeModeForPlayback(query: Record<string, unknown> | undefined): TranscodeHlsMode {
  return parseTranscodeModeFromQuery(query) ?? loadPreferredTranscodeMode();
}

export function isTranscodeModeRequestedInUrl(query: Record<string, unknown> | undefined): boolean {
  return parseTranscodeModeFromQuery(query) !== null;
}

export function savePreferredTranscodeMode(mode: string): void {
  const valid = asHlsMode(mode);
  if (valid) {
    safeWrite(() => localStorage, TRANSCODE_PREFERENCE_STORAGE_KEY, valid);
  }
}
