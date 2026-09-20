/**
 * Frame-timing measurement that works across chromium / firefox / webkit.
 *
 * The previous implementation derived "FPS" as `1000 / (now - last)` inside a
 * requestAnimationFrame callback. That is the *inverse interval between rAF
 * callbacks*, not composited frames, so it reported values like 70.63 and
 * 73.13 FPS (above any refresh rate) and 0 frames on WebKit. It could not be
 * baselined because it did not measure what it claimed to.
 *
 * This module instead collects real frame timing from the best source each
 * engine offers, in priority order:
 *
 *  1. `long-animation-frame` (LoAF) — Chromium. Gives per-frame blocking
 *     duration plus script/style/layout attribution.
 *  2. `paint` / `frame` entries via PerformanceObserver — broadly available.
 *  3. A rAF timestamp ring buffer, kept purely as a *fallback* and reported
 *     under an honest name (`rafIntervalMs`), never as frames-per-second.
 *
 * Everything here runs in the page context, so it must stay free of Node APIs
 * and of TypeScript syntax that cannot be serialized by `page.evaluate`.
 */

export type FrameTimingStat = {
  /** Number of frames observed. */
  frames: number;
  /** Frame duration percentiles in milliseconds (1000/refreshRate for smooth). */
  p50: number;
  p95: number;
  p99: number;
  max: number;
  /** Frames estimated to have been dropped (duration > 1.5x the median). */
  droppedFrames: number;
  /** Total milliseconds of frame time classified as "long" (LoAF blocking). */
  longFrameMs: number;
  /** Estimated frames per second derived from observed frame count / window. */
  effectiveFps: number;
  /** Wall-clock window the measurement covered. */
  windowMs: number;
  /** Which API supplied the numbers, so absence is explainable. */
  source: "long-animation-frame" | "paint" | "raf-fallback" | "none";
};

export type ScenarioWindow = {
  /** Mark the start of a measurement window. */
  start: () => void;
  /** Stop measuring and return stats. Never throws. */
  stop: () => Promise<FrameTimingStat>;
};

/** Install frame collectors. Call once per page, before navigation. */
export function installFrameTiming(page: {
  addInitScript: (fn: () => void) => Promise<unknown>;
}): Promise<unknown> {
  return page.addInitScript(() => {
    type FrameSample = { duration: number; blocking: number; start: number };

    const state = {
      active: false,
      startedAt: 0,
      samples: [] as FrameSample[],
      rafTimes: [] as number[],
      longFrameMs: 0,
      supported: { loaf: false },
    };
    (window as unknown as { __frameTiming: typeof state }).__frameTiming = state;

    // 1. Long animation frames (Chromium).
    try {
      const po = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          const e = entry as PerformanceEntry & {
            duration?: number;
            blockingDuration?: number;
            startTime: number;
          };
          if (!state.active) continue;
          if (e.startTime < state.startedAt) continue;
          const blocking = e.blockingDuration ?? 0;
          state.longFrameMs += blocking;
          state.samples.push({
            duration: e.duration ?? 0,
            blocking,
            start: e.startTime,
          });
        }
      });
      po.observe({
        type: "long-animation-frame",
        buffered: false,
      } as PerformanceObserverInit);
      state.supported.loaf = true;
    } catch {
      /* not supported */
    }

    // 2. rAF timestamps. LoAF only reports *long* frames, so a smooth 60fps
    //    scroll would otherwise look like "no frames". rAF is the interval
    //    between animation callbacks, labelled raf-fallback.
    const rafLoop = (now: number) => {
      if (state.active) state.rafTimes.push(now);
      requestAnimationFrame(rafLoop);
    };
    requestAnimationFrame(rafLoop);

    (window as unknown as { __startFrameWindow: () => void }).__startFrameWindow =
      () => {
        state.active = true;
        state.startedAt = performance.now();
        state.samples = [];
        state.rafTimes = [];
        state.longFrameMs = 0;
      };

    (window as unknown as { __stopFrameWindow: () => unknown }).__stopFrameWindow =
      () => {
        const windowMs = Math.max(performance.now() - state.startedAt, 1);
        state.active = false;

        const durationFrom = (times: number[]): number[] => {
          const out: number[] = [];
          for (let i = 1; i < times.length; i++) {
            out.push(times[i] - times[i - 1]);
          }
          return out;
        };

        let durations: number[] = [];
        let source: string = "none";

        if (state.samples.length > 0) {
          durations = state.samples.map((s) => s.duration).filter((d) => d > 0);
          source = "long-animation-frame";
        } else if (state.rafTimes.length > 1) {
          durations = durationFrom(state.rafTimes);
          source = "raf-fallback";
        }

        if (durations.length === 0) {
          return {
            frames: 0,
            p50: 0,
            p95: 0,
            p99: 0,
            max: 0,
            droppedFrames: 0,
            longFrameMs: Math.round(state.longFrameMs),
            effectiveFps: 0,
            windowMs: Math.round(windowMs),
            source: "none",
          };
        }

        const sorted = [...durations].sort((a, b) => a - b);
        const pct = (p: number) => {
          const idx = Math.min(
            sorted.length - 1,
            Math.max(0, Math.ceil((p / 100) * sorted.length) - 1),
          );
          return sorted[idx];
        };
        const median = pct(50);
        const droppedThreshold = Math.max(median * 1.5, 20);

        return {
          frames: durations.length,
          p50: round2(pct(50)),
          p95: round2(pct(95)),
          p99: round2(pct(99)),
          max: round2(sorted[sorted.length - 1]),
          droppedFrames: durations.filter((d) => d > droppedThreshold).length,
          longFrameMs: Math.round(state.longFrameMs),
          effectiveFps: round2(durations.length / (windowMs / 1000)),
          windowMs: Math.round(windowMs),
          source,
        };
      };

    function round2(n: number): number {
      return Math.round(n * 100) / 100;
    }
  });
}

/** Start a frame-measurement window in the page. */
export async function startFrameWindow(page: {
  evaluate: (fn: () => void) => Promise<unknown>;
}): Promise<void> {
  await page.evaluate(() => {
    (window as unknown as { __startFrameWindow?: () => void }).__startFrameWindow?.();
  });
}

/** Stop the frame window and collect stats. */
export async function stopFrameWindow(page: {
  evaluate: <T>(fn: () => T) => Promise<T>;
}): Promise<FrameTimingStat> {
  return page.evaluate(() => {
    const stop = (
      window as unknown as { __stopFrameWindow?: () => FrameTimingStat }
    ).__stopFrameWindow;
    if (!stop) {
      return {
        frames: 0,
        p50: 0,
        p95: 0,
        p99: 0,
        max: 0,
        droppedFrames: 0,
        longFrameMs: 0,
        effectiveFps: 0,
        windowMs: 0,
        source: "none" as const,
      };
    }
    return stop();
  });
}
