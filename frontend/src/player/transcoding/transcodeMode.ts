/** Whether this server can serve transcoded playback right now. */

import type { TranscodeProfile, TranscodeStatus } from "@/types/transcode.ts";
import { state } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.ts";

/**
 * `ffmpegAvailable` is a static page constant that mirrors ffmpeg availability, which
 * cannot change without a restart. Transcoding is ffmpeg-only, so it gates the feature
 * outright. The transcode toggle is read only from the live media settings, so an admin
 * change applies without a page reload. Before those settings load (or for anonymous
 * users and shares, which never load them) transcoding reports unavailable.
 */
export function isTranscodeAvailable(): boolean {
  if (globalVars?.ffmpegAvailable !== true) {
    return false;
  }
  return state.mediaSettings?.enabled === true;
}

/** Modes that use server-side HLS transcoding - each one is the profile its session encodes with. */
export type TranscodeHlsMode = TranscodeProfile;

/** Playback modes in the player UI. */
export type TranscodeMode = "native" | TranscodeHlsMode;

/** Transcode playback modes in the player UI. */
export const TRANSCODE_MODES: readonly TranscodeMode[] = ["native", "quality", "balanced", "datasaver"];

/** Modes that use server-side HLS transcoding. */
export const TRANSCODE_HLS_MODES: readonly TranscodeHlsMode[] = ["quality", "balanced", "datasaver"];

/** Strict membership check; `normalizeTranscodeMode` is the one that falls back to native. */
export function isTranscodeMode(value?: string | null): value is TranscodeMode {
  return TRANSCODE_MODES.some((known) => known === value);
}

export function normalizeTranscodeMode(mode?: string | null): TranscodeMode {
  const value = String(mode || "native").toLowerCase();
  return TRANSCODE_MODES.find((known) => known === value) ?? "native";
}

export function isTranscodeHlsMode(mode?: string | null): mode is TranscodeHlsMode {
  const normalized = normalizeTranscodeMode(mode);
  return TRANSCODE_HLS_MODES.some((known) => known === normalized);
}

/** Treat same-file active sessions as capacity-neutral for the current viewer. */
export function normalizeTranscodeSessionStatus(
  status: TranscodeStatus,
  { source, path }: { source?: string; path?: string } = {},
): TranscodeStatus {
  if (status.canStart || status.blockReason !== "user_limit") {
    return status;
  }
  const sameFileActive = (status.sessions || []).some((session) => session.source === source && session.path === path);
  if (!sameFileActive) {
    return status;
  }
  return { ...status, canStart: true, blockReason: "" };
}
