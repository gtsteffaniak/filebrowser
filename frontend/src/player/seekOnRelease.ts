/**
 * Plyr seeks on every range "input" while dragging, which triggers media byte-range
 * fetches for each step. Block that via config.listeners.seek and commit on "change"
 * (mouseup / touchend / keyboard release).
 */

/** Receives the intended target in seconds before the browser clamps it to the seekable range. */
export type SeekTargetHandler = (targetSec: number) => void;

/** The part of Plyr that seeks handling reads and drives. */
export interface SeekablePlayer {
  readonly duration: number;
  currentTime: number;
  elements: {
    progress?: HTMLElement | null;
    inputs?: { seek?: HTMLInputElement };
  };
}

export interface SeekOnReleaseOptions {
  onSeekTarget?: SeekTargetHandler;
}

export function readSeekPercent(input: HTMLInputElement): number | null {
  const attr = input.getAttribute("seek-value");
  if (attr !== null && attr !== "") {
    input.removeAttribute("seek-value");
    const parsed = Number(attr);
    return Number.isFinite(parsed) ? parsed : null;
  }
  const parsed = Number(input.value);
  return Number.isFinite(parsed) ? parsed : null;
}

function isMouseEvent(event: Event): event is MouseEvent {
  return "clientX" in event;
}

function isTouchEvent(event: Event): event is TouchEvent {
  return "touches" in event || "changedTouches" in event;
}

function clientXFromEvent(event?: Event | null): number | null {
  if (!event) return null;
  if (isMouseEvent(event) && Number.isFinite(event.clientX)) {
    return event.clientX;
  }
  const touch = isTouchEvent(event) ? (event.changedTouches?.[0] ?? event.touches?.[0]) : undefined;
  return touch && Number.isFinite(touch.clientX) ? touch.clientX : null;
}

/** @param onSeekTarget receives the intended target in seconds before the browser clamps it to the seekable range */
export function commitSeek(player: SeekablePlayer, event?: Event, onSeekTarget?: SeekTargetHandler): void {
  const { elements } = player;
  const seek = elements?.inputs?.seek;
  const duration = player?.duration;
  if (!seek || !Number.isFinite(duration) || duration <= 0) {
    return;
  }
  const progress = elements?.progress;
  const clientX = clientXFromEvent(event);
  if (progress && clientX !== null) {
    const rect = progress.getBoundingClientRect();
    if (rect.width > 0) {
      const percent = Math.max(0, Math.min(100, ((clientX - rect.left) / rect.width) * 100));
      const targetSec = (percent / 100) * duration;
      onSeekTarget?.(targetSec);
      player.currentTime = targetSec;
      return;
    }
  }
  const target = event?.currentTarget instanceof HTMLInputElement ? event.currentTarget : seek;
  const percent = readSeekPercent(target);
  if (percent === null) {
    return;
  }
  const targetSec = (percent / Number(seek.max)) * duration;
  onSeekTarget?.(targetSec);
  player.currentTime = targetSec;
}

export function enableSeekOnRelease(player: SeekablePlayer, { onSeekTarget }: SeekOnReleaseOptions = {}): () => void {
  const seek = player?.elements?.inputs?.seek;
  if (!seek) {
    return () => {};
  }

  let ignoreNextChange = false;
  let releaseValue: string | null = null;

  const onCommit = (event: Event) => {
    if (event.type === "change" && ignoreNextChange) {
      const sameAsRelease = seek.value === releaseValue;
      ignoreNextChange = false;
      releaseValue = null;
      if (sameAsRelease) {
        return;
      }
    }
    if (event.type === "mouseup" || event.type === "touchend") {
      ignoreNextChange = true;
      releaseValue = seek.value;
    }
    commitSeek(player, event, onSeekTarget);
  };

  const captureOpts = { capture: true };
  const touchCaptureOpts = { capture: true, passive: true };

  seek.addEventListener("mouseup", onCommit, captureOpts);
  seek.addEventListener("touchend", onCommit, touchCaptureOpts);
  seek.addEventListener("change", onCommit);

  return () => {
    seek.removeEventListener("mouseup", onCommit, captureOpts);
    seek.removeEventListener("touchend", onCommit, touchCaptureOpts);
    seek.removeEventListener("change", onCommit);
  };
}

/** Plyr listeners.seek hook: return false to skip seek-on-input. */
export function blockSeekOnInput(): false {
  return false;
}
