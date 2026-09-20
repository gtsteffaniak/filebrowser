/**
 * Cross-browser metrics gathered from standard W3C APIs.
 *
 * These work in chromium, firefox and webkit, so they give the local
 * three-browser comparison real substance even though only chromium is
 * baseline-gated. Anything an engine does not implement is recorded as
 * unavailable rather than silently omitted, so a missing metric is visible in
 * the report instead of looking like a zero.
 */

export type WebVitals = {
  navigation: {
    domInteractive: number | null;
    domContentLoaded: number | null;
    loadEventEnd: number | null;
    responseStart: number | null;
    ttfb: number | null;
    transferSize: number | null;
    decodedBodySize: number | null;
  };
  paint: {
    firstPaint: number | null;
    firstContentfulPaint: number | null;
    largestContentfulPaint: number | null;
  };
  layoutShift: {
    cumulativeLayoutShift: number | null;
  };
  network: {
    resourceCount: number;
    transferSize: number;
    decodedBodySize: number;
    /** Slowest resource by duration. */
    slowestResource: { name: string; durationMs: number; transferSize: number } | null;
  };
  /** Metrics this engine does not expose, so absence is explainable. */
  unsupported: string[];
};

export type InteractionTiming = {
  /** Raw event-timing durations observed for the scenario. */
  samples: number;
  p50: number;
  p95: number;
  max: number;
  /** Processing (script) time, the INP-relevant component. */
  processingP95: number;
};

/** Install observers for web-vitals style entries. Call before navigation. */
export function installWebVitals(page: {
  addInitScript: (fn: () => void) => Promise<unknown>;
}): Promise<unknown> {
  return page.addInitScript(() => {
    const state = {
      lcp: null as number | null,
      cls: 0,
      clsSupported: false,
      lcpSupported: false,
      eventTimings: [] as number[],
      eventProcessing: [] as number[],
      activeFrom: 0,
      collectingEvents: false,
    };
    (window as unknown as { __vitals: typeof state }).__vitals = state;

    // Largest Contentful Paint (chromium/webkit; firefox lacks it).
    try {
      const po = new PerformanceObserver((list) => {
        const entries = list.getEntries();
        const last = entries[entries.length - 1];
        if (last) state.lcp = last.startTime;
      });
      po.observe({ type: "largest-contentful-paint", buffered: true } as PerformanceObserverInit);
      state.lcpSupported = true;
    } catch {
      /* unsupported */
    }

    // Cumulative Layout Shift.
    try {
      const po = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          const e = entry as PerformanceEntry & {
            value?: number;
            hadRecentInput?: boolean;
          };
          if (!e.hadRecentInput) state.cls += e.value ?? 0;
        }
      });
      po.observe({ type: "layout-shift", buffered: true } as PerformanceObserverInit);
      state.clsSupported = true;
    } catch {
      /* unsupported */
    }

    // Event timing for interaction latency (INP component).
    try {
      const po = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          const e = entry as PerformanceEntry & {
            processingStart?: number;
            processingEnd?: number;
          };
          if (!state.collectingEvents) continue;
          if (e.startTime < state.activeFrom) continue;
          state.eventTimings.push(e.duration);
          if (typeof e.processingStart === "number" && typeof e.processingEnd === "number") {
            state.eventProcessing.push(e.processingEnd - e.processingStart);
          }
        }
      });
      po.observe({ type: "event", buffered: true, durationThreshold: 0 } as PerformanceObserverInit);
    } catch {
      /* unsupported */
    }

    (
      window as unknown as { __startEventWindow: () => void }
    ).__startEventWindow = () => {
      state.eventTimings = [];
      state.eventProcessing = [];
      state.activeFrom = performance.now();
      state.collectingEvents = true;
    };

    (
      window as unknown as { __stopEventWindow: () => unknown }
    ).__stopEventWindow = () => {
      state.collectingEvents = false;
      const pct = (arr: number[], p: number) => {
        if (arr.length === 0) return 0;
        const sorted = [...arr].sort((a, b) => a - b);
        const idx = Math.min(
          sorted.length - 1,
          Math.max(0, Math.ceil((p / 100) * sorted.length) - 1),
        );
        return Math.round(sorted[idx] * 100) / 100;
      };
      return {
        samples: state.eventTimings.length,
        p50: pct(state.eventTimings, 50),
        p95: pct(state.eventTimings, 95),
        max: pct(state.eventTimings, 100),
        processingP95: pct(state.eventProcessing, 95),
      };
    };
  });
}

/** Collect everything from the standard performance timeline. */
export async function readWebVitals(page: {
  evaluate: <T>(fn: () => T) => Promise<T>;
}): Promise<WebVitals> {
  return page.evaluate(() => {
    const unsupported: string[] = [];
    const nav = performance.getEntriesByType(
      "navigation",
    )[0] as PerformanceNavigationTiming | undefined;

    const paintEntries = performance.getEntriesByType("paint");
    const paintOf = (name: string): number | null => {
      const found = paintEntries.find((p) => p.name === name);
      return found ? round2(found.startTime) : null;
    };

    const vitals = (window as unknown as {
      __vitals?: { lcp: number | null; cls: number; lcpSupported: boolean; clsSupported: boolean };
    }).__vitals;

    const resources = performance.getEntriesByType(
      "resource",
    ) as PerformanceResourceTiming[];

    let transferSize = 0;
    let decodedBodySize = 0;
    let slowest: { name: string; durationMs: number; transferSize: number } | null = null;
    for (const r of resources) {
      transferSize += r.transferSize || 0;
      decodedBodySize += r.decodedBodySize || 0;
      if (!slowest || r.duration > slowest.durationMs) {
        slowest = {
          name: shortName(r.name),
          durationMs: round2(r.duration),
          transferSize: r.transferSize || 0,
        };
      }
    }

    if (!nav) unsupported.push("navigation-timing");
    if (paintOf("first-paint") === null) unsupported.push("first-paint");
    if (paintOf("first-contentful-paint") === null) unsupported.push("first-contentful-paint");
    if (!vitals?.lcpSupported) unsupported.push("largest-contentful-paint");
    if (!vitals?.clsSupported) unsupported.push("layout-shift");

    const ttfb =
      nav && typeof nav.responseStart === "number"
        ? round2(nav.responseStart - nav.startTime)
        : null;

    return {
      navigation: {
        domInteractive: nav ? round2(nav.domInteractive) : null,
        domContentLoaded: nav ? round2(nav.domContentLoadedEventEnd) : null,
        loadEventEnd: nav ? round2(nav.loadEventEnd) : null,
        responseStart: nav ? round2(nav.responseStart) : null,
        ttfb,
        transferSize: nav ? nav.transferSize || 0 : null,
        decodedBodySize: nav ? nav.decodedBodySize || 0 : null,
      },
      paint: {
        firstPaint: paintOf("first-paint"),
        firstContentfulPaint: paintOf("first-contentful-paint"),
        largestContentfulPaint: vitals?.lcp ? round2(vitals.lcp) : null,
      },
      layoutShift: {
        cumulativeLayoutShift: vitals?.clsSupported ? round2(vitals.cls) : null,
      },
      network: {
        resourceCount: resources.length,
        transferSize,
        decodedBodySize,
        slowestResource: slowest,
      },
      unsupported,
    };

    function round2(n: number): number {
      return Math.round(n * 100) / 100;
    }

    function shortName(url: string): string {
      try {
        const u = new URL(url, window.location.origin);
        return u.pathname + (u.search || "");
      } catch {
        return url.slice(0, 120);
      }
    }
  });
}

export async function startEventWindow(page: {
  evaluate: (fn: () => void) => Promise<unknown>;
}): Promise<void> {
  await page.evaluate(() => {
    (window as unknown as { __startEventWindow?: () => void }).__startEventWindow?.();
  });
}

export async function stopEventWindow(page: {
  evaluate: <T>(fn: () => T) => Promise<T>;
}): Promise<InteractionTiming> {
  return page.evaluate(() => {
    const stop = (
      window as unknown as { __stopEventWindow?: () => InteractionTiming }
    ).__stopEventWindow;
    return (
      stop?.() ?? {
        samples: 0,
        p50: 0,
        p95: 0,
        max: 0,
        processingP95: 0,
      }
    );
  });
}
