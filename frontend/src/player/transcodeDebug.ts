import type { LoaderResponse } from "hls.js";

/**
 * Transcode/HLS console diagnostics ([transcode] prefix).
 * Verbose hls.js events: localStorage.setItem('transcodeDebug', '1') then reload.
 */

export type TranscodeLogValue =
  | string
  | number
  | boolean
  | null
  | undefined
  | LoaderResponse
  | TranscodeLogValue[]
  | { [key: string]: TranscodeLogValue };

export type TranscodeLogDetail = { [key: string]: TranscodeLogValue } | null;

export function isTranscodeDebugVerbose(): boolean {
  try {
    return localStorage.getItem("transcodeDebug") === "1";
  } catch {
    return false;
  }
}

export function transcodeLog(message: string, detail?: TranscodeLogDetail): void {
  if (detail === undefined) {
    console.info("[transcode]", message);
    return;
  }
  console.info("[transcode]", message, detail);
}

export function transcodeWarn(message: string, detail?: TranscodeLogDetail): void {
  if (detail === undefined) {
    console.warn("[transcode]", message);
    return;
  }
  console.warn("[transcode]", message, detail);
}

export function transcodeError(message: string, detail?: TranscodeLogDetail): void {
  if (detail === undefined) {
    console.error("[transcode]", message);
    return;
  }
  console.error("[transcode]", message, detail);
}

export function transcodeVerbose(message: string, detail?: TranscodeLogDetail): void {
  if (!isTranscodeDebugVerbose()) {
    return;
  }
  transcodeLog(`[verbose] ${message}`, detail);
}
