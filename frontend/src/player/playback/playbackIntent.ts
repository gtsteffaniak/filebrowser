import { errorFields, type PlayerError } from "@/player/playerErrors.ts";
import { transcodeLog, transcodeWarn } from "@/player/transcoding/transcodeDebug.ts";

/**
 * Helpers for preserving playback intent across src/HLS handoffs.
 * Mirrors native attachVideoStreamAndPlay semantics.
 */

/** Outcome of an attempt to resume playback. `blocked`: the autoplay policy refused it. */
export interface PlayOutcome {
  played: boolean;
  blocked: boolean;
}

export interface ResumeOptions {
  playCall?: () => void | Promise<void>;
  onPlaying?: () => void;
  isCancelled?: () => boolean;
}

export type PlayPrime = Promise<void> | null;

export function isPlayBlockedError(err?: PlayerError | null): boolean {
  if (!err) {
    return false;
  }
  const { name, message = "" } = errorFields(err);
  if (name === "NotAllowedError") {
    return true;
  }
  return message.includes("user didn") || message.includes("not allowed");
}

export function isPlayTeardownError(err?: PlayerError | null): boolean {
  if (!err) {
    return false;
  }
  const { name, message = "" } = errorFields(err);
  if (name === "AbortError" || name === "NotSupportedError") {
    return true;
  }
  return message.includes("no supported source");
}

/** Call synchronously during a user gesture; returns a pending play promise. */
export function primePlay(video?: HTMLMediaElement | null): PlayPrime {
  if (!video) {
    return null;
  }
  let promise: Promise<void>;
  try {
    promise = video.play();
  } catch (err) {
    promise = Promise.reject(err);
  }
  void promise.catch(() => {});
  return promise;
}

/** How long resumeWhenReady waits for canplay before giving up. */
const PLAY_READY_TIMEOUT_MS = 10_000;

export function isPlaybackProgressing(video?: HTMLMediaElement | null): boolean {
  return Boolean(video) && !video?.paused && !video?.ended && (video?.readyState ?? 0) >= HTMLMediaElement.HAVE_FUTURE_DATA;
}

export function resumeWhenReady(
  video: HTMLMediaElement | null | undefined,
  { playCall, onPlaying, isCancelled }: ResumeOptions = {},
): Promise<PlayOutcome> {
  return new Promise((resolve) => {
    if (!video) {
      transcodeWarn("resumeWhenReady skipped — no video element");
      resolve({ played: false, blocked: false });
      return;
    }
    if (isCancelled?.()) {
      transcodeLog("resumeWhenReady cancelled before start");
      resolve({ played: false, blocked: false });
      return;
    }
    if (isPlaybackProgressing(video)) {
      transcodeLog("resumeWhenReady skipped — already playing");
      onPlaying?.();
      resolve({ played: true, blocked: false });
      return;
    }

    let settled = false;
    let playAttempted = false;
    let timerId: ReturnType<typeof setTimeout> | undefined;
    const finish = (result: PlayOutcome) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timerId);
      video.removeEventListener("canplay", tryPlay);
      resolve(result);
    };
    const tryPlay = () => {
      if (settled || playAttempted) {
        return;
      }
      // The owning preview may have been closed/unmounted while we waited for canplay.
      if (isCancelled?.()) {
        transcodeLog("resumeWhenReady cancelled before play");
        finish({ played: false, blocked: false });
        return;
      }
      playAttempted = true;
      video.removeEventListener("canplay", tryPlay);
      const invoke = playCall ?? (() => video.play());
      transcodeLog("resumeWhenReady tryPlay", {
        readyState: video.readyState,
        paused: video.paused,
        hasPlayCall: Boolean(playCall),
      });
      let playResult: void | Promise<void>;
      try {
        playResult = invoke();
      } catch (err) {
        playResult = Promise.reject(err);
      }
      void Promise.resolve(playResult)
        .then(() => {
          if (settled) return;
          if (isCancelled?.()) {
            transcodeLog("resumeWhenReady cancelled while play was pending");
            finish({ played: false, blocked: false });
            return;
          }
          transcodeLog("resumeWhenReady play resolved");
          onPlaying?.();
          finish({ played: true, blocked: false });
        })
        .catch((err: PlayerError) => {
          const { name, message } = errorFields(err);
          transcodeWarn("resumeWhenReady play rejected", { name, message });
          finish({
            played: false,
            blocked: !isCancelled?.() && isPlayBlockedError(err),
          });
        });
    };

    timerId = setTimeout(() => {
      transcodeWarn("resumeWhenReady timed out waiting for canplay", { readyState: video.readyState });
      finish({ played: false, blocked: false });
    }, PLAY_READY_TIMEOUT_MS);

    if (video.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
      tryPlay();
      return;
    }
    transcodeLog("resumeWhenReady waiting for canplay", { readyState: video.readyState });
    video.addEventListener("canplay", tryPlay);
  });
}

/** Await a promise, but stop waiting once cancelled or the ready deadline passes. */
function settleWithin(promise: Promise<void>, isCancelled?: () => boolean): Promise<void> {
  return new Promise((resolve, reject) => {
    const deadline = Date.now() + PLAY_READY_TIMEOUT_MS;
    const poll = setInterval(() => {
      if (isCancelled?.() || Date.now() >= deadline) {
        clearInterval(poll);
        resolve();
      }
    }, 250);
    promise.then(
      () => {
        clearInterval(poll);
        resolve();
      },
      (err) => {
        clearInterval(poll);
        reject(err);
      },
    );
  });
}

/** Await a play promise after HLS attach, or fall back to resumeWhenReady. */
export async function continuePlay(
  video: HTMLMediaElement | null | undefined,
  playPromise: PlayPrime | undefined,
  { playCall, onPlaying, isCancelled }: ResumeOptions = {},
): Promise<PlayOutcome> {
  transcodeLog("continuePlay", {
    hasPlayPromise: Boolean(playPromise),
    readyState: video?.readyState,
    paused: video?.paused,
  });
  if (playPromise !== null && playPromise !== undefined) {
    let primedError: PlayerError | null = null;
    try {
      await settleWithin(playPromise, isCancelled);
    } catch (err) {
      primedError = err as PlayerError;
    }
    if (isCancelled?.()) {
      transcodeLog("continuePlay cancelled after primed promise");
      return { played: false, blocked: false };
    }
    if (primedError) {
      const { name, message } = errorFields(primedError);
      transcodeWarn("continuePlay primed promise rejected", { name, message });
      if (isPlayBlockedError(primedError)) {
        transcodeWarn("continuePlay blocked by autoplay policy");
        return { played: false, blocked: true };
      }
    } else if (isPlaybackProgressing(video)) {
      // The primed promise may belong to a source that has since been replaced
      transcodeLog("continuePlay primed promise resolved - current source playing");
      onPlaying?.();
      return { played: true, blocked: false };
    } else {
      transcodeWarn("continuePlay primed promise resolved but current source is not playing", {
        readyState: video?.readyState,
        paused: video?.paused,
      });
    }
  }
  return resumeWhenReady(video, { playCall, onPlaying, isCancelled });
}

export function appendMediaFragment(url: string, seconds: number): string;
export function appendMediaFragment(url: string | null | undefined, seconds: number): string | null | undefined;
export function appendMediaFragment(url: string | null | undefined, seconds: number): string | null | undefined {
  if (!url || !Number.isFinite(seconds) || seconds <= 0) {
    return url;
  }
  const base = String(url).split("#")[0];
  return `${base}#t=${seconds}`;
}
