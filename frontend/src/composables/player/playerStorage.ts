import { getObjectProperty } from "@/utils/object.js";

// Storage keys keep the legacy "plyr" prefix so users do not lose their saved preferences.
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
      const parsed: unknown = JSON.parse(raw);
      if (typeof parsed === "object" && parsed !== null) {
        data = parsed as Record<string, unknown>;
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

export function readDesktopPanelOpen(): boolean {
  return localStorage.getItem(DESKTOP_PANEL_KEY) === "1";
}

export function writeDesktopPanelOpen(open: boolean): void {
  localStorage.setItem(DESKTOP_PANEL_KEY, open ? "1" : "0");
}

export function readMobileLyricsOpen(): boolean {
  return sessionStorage.getItem(MOBILE_LYRICS_KEY) === "1";
}

export function writeMobileLyricsOpen(open: boolean): void {
  sessionStorage.setItem(MOBILE_LYRICS_KEY, open ? "1" : "0");
}

export function readAlbumArtSize(): number {
  return Number.parseFloat(sessionStorage.getItem(ALBUM_ART_SIZE_KEY) ?? "") || 24;
}

export function writeAlbumArtSize(size: number): void {
  sessionStorage.setItem(ALBUM_ART_SIZE_KEY, size.toString());
}
