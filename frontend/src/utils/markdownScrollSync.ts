// src/utils/markdownScrollSync.ts

export interface ScrollSyncGuard {
  schedule(publish: () => void): void;
  suppress(ms?: number): void;
  applyRemote(write: () => void): void;
  cancel(): boolean;
}

export function createScrollSyncGuard(suppressMs = 300): ScrollSyncGuard {
  let frame: number | null = null;
  let suppressed = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  return {
    // Throttle outgoing publishes to one per frame, skipped while suppressed.
    schedule(publish: () => void) {
      if (suppressed || frame) return;
      frame = requestAnimationFrame(() => {
        frame = null;
        publish();
      });
    },
    suppress(ms = suppressMs) {
      suppressed = true;
      clearTimeout(timer);
      timer = setTimeout(() => { suppressed = false; }, ms);
    },
    applyRemote(write: () => void) {
      this.suppress();
      write();
    },
    cancel() {
      if (!frame) return false;
      cancelAnimationFrame(frame);
      frame = null;
      return true;
    },
  };
}
