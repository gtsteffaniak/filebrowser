import type { Page } from "@playwright/test";

/**
 * CDP `Performance.getMetrics` splits into two very different kinds of value:
 *
 *  - **Cumulative counters** (`LayoutDuration`, `ScriptDuration`, ...). These
 *    only mean something as a *delta across the measurement window*. Reading
 *    them once, after a scenario, always yields the page-lifetime total (or 0
 *    when nothing was flushed), which is why the original harness recorded
 *    `LayoutDuration: 0` at every scale.
 *  - **Gauges** (`Nodes`, `JSEventListeners`, `JSHeapUsedSize`, ...). These are
 *    instantaneous and only meaningful as an absolute reading.
 *
 * We therefore sample before and after each scenario and keep both shapes.
 */

/** Counters that accumulate and must be differenced. */
export const CUMULATIVE_CDP_METRICS = [
  "LayoutCount",
  "RecalcStyleCount",
  "LayoutDuration",
  "RecalcStyleDuration",
  "ScriptDuration",
  "V8CompileDuration",
  "TaskDuration",
  "TaskOtherDuration",
  "DevToolsCommandDuration",
  "ProcessTime",
  "ThreadTime",
] as const;

/** Instantaneous readings, meaningful as absolutes. */
export const GAUGE_CDP_METRICS = [
  "Nodes",
  "JSEventListeners",
  "JSHeapUsedSize",
  "JSHeapTotalSize",
  "Documents",
  "Frames",
  "LayoutObjects",
  "Resources",
  "ContextLifecycleStateObservers",
  "V8PerContextDatas",
  "WorkerGlobalScopes",
  "ArrayBufferContents",
  "DetachedScriptStates",
] as const;

export type CdpMetricMap = Record<string, number>;

export type CdpSample = {
  cumulative: CdpMetricMap;
  gauge: CdpMetricMap;
};

export type CdpDelta = {
  /** Differences of cumulative counters over the scenario window. */
  delta: CdpMetricMap;
  /** Absolute gauge readings taken after the scenario. */
  gauge: CdpMetricMap;
  /** Absolute cumulative readings before/after, for debugging. */
  before: CdpMetricMap;
  after: CdpMetricMap;
  /** Wall-clock duration of the CDP window, for rate calculations. */
  windowMs: number;
};

export function isChromiumPage(page: Page): boolean {
  return page.context().browser()?.browserType().name() === "chromium";
}

async function newCdpSession(page: Page) {
  const session = await page.context().newCDPSession(page);
  await session.send("Performance.enable");
  return session;
}

function splitMetrics(metrics: { name: string; value: number }[]): CdpSample {
  const cumulative: CdpMetricMap = {};
  const gauge: CdpMetricMap = {};
  const cumulativeSet = new Set<string>(CUMULATIVE_CDP_METRICS);
  const gaugeSet = new Set<string>(GAUGE_CDP_METRICS);

  for (const m of metrics) {
    if (cumulativeSet.has(m.name)) {
      cumulative[m.name] = m.value;
    } else if (gaugeSet.has(m.name)) {
      gauge[m.name] = m.value;
    }
  }
  return { cumulative, gauge };
}

/**
 * Samples CDP metrics around `fn`.
 *
 * Returns `null` on non-Chromium browsers rather than a misleading empty
 * object, so the report can distinguish "not supported here" from "zero".
 */
export async function withCdpDelta<T>(
  page: Page,
  fn: () => Promise<T>,
): Promise<{ result: T; cdp: CdpDelta | null }> {
  if (!isChromiumPage(page)) {
    return { result: await fn(), cdp: null };
  }

  let session: Awaited<ReturnType<typeof newCdpSession>> | null = null;
  try {
    session = await newCdpSession(page);
    const before = splitMetrics(
      (await session.send("Performance.getMetrics")).metrics,
    );

    const startedAt = Date.now();
    const result = await fn();
    const windowMs = Date.now() - startedAt;

    const after = splitMetrics(
      (await session.send("Performance.getMetrics")).metrics,
    );

    const delta: CdpMetricMap = {};
    for (const key of CUMULATIVE_CDP_METRICS) {
      const b = before.cumulative[key];
      const a = after.cumulative[key];
      if (typeof a === "number" && typeof b === "number") {
        // Counters can reset if the renderer navigated; clamp at 0 rather than
        // reporting a negative duration.
        delta[key] = Math.max(0, a - b);
      }
    }

    return {
      result,
      cdp: {
        delta,
        gauge: after.gauge,
        before: before.cumulative,
        after: after.cumulative,
        windowMs,
      },
    };
  } catch {
    return { result: await fn(), cdp: null };
  } finally {
    try {
      await session?.detach();
    } catch {
      /* session already gone */
    }
  }
}

/** Flatten an optional CDP delta into report-friendly scalar keys. */
export function flattenCdp(
  cdp: CdpDelta | null | undefined,
): CdpMetricMap | null {
  if (!cdp) return null;
  const out: CdpMetricMap = {};
  for (const [k, v] of Object.entries(cdp.delta)) {
    out[`${k}Delta`] = round(v);
  }
  for (const [k, v] of Object.entries(cdp.gauge)) {
    out[k] = round(v);
  }
  out.cdpWindowMs = cdp.windowMs;
  return out;
}

function round(n: number): number {
  return Math.round(n * 1000) / 1000;
}
