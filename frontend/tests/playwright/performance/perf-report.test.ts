import { describe, expect, it } from "vitest";
import { cdpDurationSecondsToMs } from "./perf-cdp";
import { buildContributors, buildRootCauseParagraph } from "./perf-report";
import type { TraceAnalysis } from "./perf-trace";

describe("cdpDurationSecondsToMs", () => {
  it("converts fractional seconds to rounded milliseconds", () => {
    expect(cdpDurationSecondsToMs(1.5)).toBe(1500);
    expect(cdpDurationSecondsToMs(0.056306)).toBe(56);
    expect(cdpDurationSecondsToMs(0)).toBe(0);
  });
});

describe("buildRootCauseParagraph", () => {
  it("prints CDP window and durations in milliseconds", () => {
    const text = buildRootCauseParagraph({
      browser: "chromium",
      scale: 1000,
      scenario: "load",
      timestamp: "",
      metrics: {
        loadListingMs: 5000,
        dom: { listingItemCount: 2000, documentElementCount: 20154 },
        probe: { addListenerCalls: 20134, intersectionObservers: 2000 },
        cdp: {
          delta: {
            ScriptDuration: 1.2,
            LayoutDuration: 0.05,
            RecalcStyleDuration: 0.01,
            LayoutCount: 39,
          },
          gauge: {},
          before: {},
          after: {},
          windowMs: 5123,
        },
      },
    });

    expect(text).toContain("CDP window (5123 ms)");
    expect(text).toContain("script 1200 ms");
    expect(text).toContain("layout 50 ms");
    expect(text).not.toContain("undefined ms");
  });

  it("prefers long tasks over IntersectionObserver in dominant-cost line", () => {
    const text = buildRootCauseParagraph({
      browser: "chromium",
      scale: 10000,
      scenario: "load",
      timestamp: "",
      metrics: {
        loadListingMs: 19000,
        dom: { listingItemCount: 20000, documentElementCount: 200154 },
        probe: {
          addListenerCalls: 200134,
          intersectionObservers: 20000,
          longTasks: [{ duration: 29000, startTime: 100 }],
        },
      },
    });

    expect(text).toContain("**Likely dominant cost:** main-thread long tasks");
    expect(text).not.toContain("IntersectionObserver per listing");
  });
});

describe("buildContributors", () => {
  it("includes chromium trace hotspots for app functions", () => {
    const trace: TraceAnalysis = {
      tracePath: "/tmp/trace.json",
      eventCount: 100,
      totalMs: 5000,
      spanMs: 5000,
      topFunctions: [
        {
          name: "updateScrollableContent",
          selfMs: 2400,
          totalMs: 2400,
          category: "app",
          samples: 2400,
          location: "static/assets/Scrollbar.js:137",
        },
      ],
      forcedLayoutEvents: 0,
      forcedLayoutMs: 0,
      gcEvents: 10,
      gcMs: 900,
      longTaskSlices: [],
      categories: {},
      findings: [],
    };

    const contributors = buildContributors(
      {
        browser: "chromium",
        scale: 10000,
        scenario: "load",
        timestamp: "",
        metrics: {
          loadListingMs: 19000,
          dom: { listingItemCount: 20000 },
          probe: { intersectionObservers: 20000, addListenerCalls: 200000 },
        },
      },
      trace,
    );

    const factors = contributors.map((c) => c.factor);
    expect(factors.some((f) => f.includes("updateScrollableContent"))).toBe(true);
    expect(factors.some((f) => f.includes("Garbage collection"))).toBe(true);
  });
});
