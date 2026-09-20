import { readFile } from "node:fs/promises";

/**
 * Chromium trace analysis.
 *
 * `browser.startTracing` produces raw Chrome trace JSON (Chromium-only API,
 * distinct from Playwright Tracing). Linking to Perfetto is useful but stops
 * short of the goal of pointing at source code, so we aggregate the trace into
 * the shapes that directly identify a culprit:
 *
 *  - hottest functions by self time (where CPU actually went);
 *  - hottest by total time (what dominates wall clock, including callees);
 *  - forced synchronous layouts ("layout thrashing" — read/write interleaving);
 *  - major GC events, which explain frame drops without a hot function.
 */

export type TraceFunction = {
  name: string;
  selfMs: number;
  totalMs: number;
  category: string;
  samples: number;
  /** Source location when the trace carried one. */
  location: string | null;
};

export type TraceAnalysis = {
  tracePath: string;
  eventCount: number;
  totalMs: number;
  /** Wall-clock span of the trace. */
  spanMs: number;
  topFunctions: TraceFunction[];
  forcedLayoutEvents: number;
  forcedLayoutMs: number;
  gcEvents: number;
  gcMs: number;
  longTaskSlices: { name: string; durationMs: number; startMs: number }[];
  categories: Record<string, number>;
  /** Human-readable one-liners derived from the aggregates. */
  findings: string[];
};

type RawTraceEvent = {
  ph?: string;
  name?: string;
  cat?: string;
  ts?: number;
  dur?: number;
  pid?: number;
  tid?: number;
  args?: Record<string, unknown>;
};

/** Read and aggregate a trace file. Never throws on malformed input. */
export async function analyzeTrace(
  tracePath: string,
): Promise<TraceAnalysis | null> {
  let raw: string;
  try {
    raw = await readFile(tracePath, "utf8");
  } catch {
    return null;
  }

  let events: RawTraceEvent[];
  try {
    const parsed = JSON.parse(raw) as RawTraceEvent[] | { traceEvents?: RawTraceEvent[] };
    events = Array.isArray(parsed)
      ? parsed
      : (parsed.traceEvents ?? []);
  } catch {
    return null;
  }

  if (events.length === 0) return null;

  // Function attribution comes from the CPU profiler's ProfileChunk nodes,
  // which carry the call tree. We map node ids to names and accumulate samples.
  const nodeNames = new Map<number, { name: string; location: string | null }>();
  const selfByNode = new Map<number, number>();
  const totalByNode = new Map<number, number>();
  const categories: Record<string, number> = {};

  let forcedLayoutEvents = 0;
  let forcedLayoutMs = 0;
  let gcEvents = 0;
  let gcMs = 0;
  const longTaskSlices: { name: string; durationMs: number; startMs: number }[] = [];

  let minTs = Number.POSITIVE_INFINITY;
  let maxTs = 0;
  let totalMs = 0;

  for (const e of events) {
    if (typeof e.ts === "number") {
      minTs = Math.min(minTs, e.ts);
      maxTs = Math.max(maxTs, e.ts + (e.dur ?? 0));
    }

    if (e.name === "ProfileChunk") {
      collectProfileChunk(e, nodeNames, selfByNode, totalByNode);
      continue;
    }

    if (e.ph === "X" && typeof e.dur === "number") {
      totalMs += e.dur;
      if (e.cat) {
        const primary = e.cat.split(",")[0];
        categories[primary] = (categories[primary] ?? 0) + e.dur;
      }

      const name = e.name ?? "unknown";
      // Blink's forced synchronous layout markers.
      if (
        name === "Layout" &&
        (String(e.args?.beginData ?? "").includes("forced") ||
          (e.args && "dirtyObjects" in e.args && forcedMarker(e)))
      ) {
        forcedLayoutEvents += 1;
        forcedLayoutMs += e.dur;
      } else if (/^Layout$/.test(name) && isForced(e)) {
        forcedLayoutEvents += 1;
        forcedLayoutMs += e.dur;
      }

      if (name === "MajorGC" || name === "MinorGC" || name === "GC") {
        gcEvents += 1;
        gcMs += e.dur;
      }

      // Slice anything over 50ms as a long task.
      if (e.dur >= 50_000) {
        longTaskSlices.push({
          name,
          durationMs: round(e.dur / 1000),
          startMs: round((e.ts ?? 0) / 1000),
        });
      }
    }
  }

  const topFunctions = buildTopFunctions(nodeNames, selfByNode, totalByNode);
  const spanMs = Number.isFinite(minTs) ? (maxTs - minTs) / 1000 : 0;

  return {
    tracePath,
    eventCount: events.length,
    totalMs: round(totalMs / 1000),
    spanMs: round(spanMs),
    topFunctions,
    forcedLayoutEvents,
    forcedLayoutMs: round(forcedLayoutMs / 1000),
    gcEvents,
    gcMs: round(gcMs / 1000),
    longTaskSlices: longTaskSlices
      .sort((a, b) => b.durationMs - a.durationMs)
      .slice(0, 15),
    categories: Object.fromEntries(
      Object.entries(categories)
        .map(([k, v]) => [k, round(v / 1000)] as const)
        .sort((a, b) => b[1] - a[1])
        .slice(0, 15),
    ),
    findings: buildFindings({
      topFunctions,
      forcedLayoutEvents,
      forcedLayoutMs: round(forcedLayoutMs / 1000),
      gcMs: round(gcMs / 1000),
    }),
  };
}

function forcedMarker(e: RawTraceEvent): boolean {
  const d = e.args?.beginData;
  if (d && typeof d === "object" && d !== null) {
    const stack = (d as { stackTrace?: unknown }).stackTrace;
    return Array.isArray(stack) && stack.length > 0;
  }
  return false;
}

function isForced(e: RawTraceEvent): boolean {
  if (!e.args) return false;
  const begin = e.args.beginData;
  if (begin && typeof begin === "object") {
    const stack = (begin as { stackTrace?: unknown }).stackTrace;
    if (Array.isArray(stack) && stack.length > 0) return true;
    const url = (begin as { url?: unknown }).url;
    if (typeof url === "string" && url.includes("forced")) return true;
  }
  return false;
}

function collectProfileChunk(
  e: RawTraceEvent,
  nodeNames: Map<number, { name: string; location: string | null }>,
  selfByNode: Map<number, number>,
  totalByNode: Map<number, number>,
): void {
  const data = e.args?.data as
    | {
        cpuProfile?: {
          nodes?: {
            id: number;
            children?: number[];
            callFrame?: {
              functionName?: string;
              url?: string;
              lineNumber?: number;
            };
          }[];
          samples?: number[];
        };
      }
    | undefined;

  const profile = data?.cpuProfile;
  if (!profile) return;

  const parentOf = new Map<number, number>();
  for (const node of profile.nodes ?? []) {
    const frame = node.callFrame;
    if (!frame) continue;
    const name = frame.functionName || "(anonymous)";
    const location =
      frame.url && frame.lineNumber !== undefined
        ? `${shortUrl(frame.url)}:${frame.lineNumber + 1}`
        : null;
    nodeNames.set(node.id, { name, location });
    for (const childId of node.children ?? []) {
      parentOf.set(childId, node.id);
    }
  }

  for (const id of profile.samples ?? []) {
    // Each sample is one ~1ms tick attributed to the sampled node's self time.
    selfByNode.set(id, (selfByNode.get(id) ?? 0) + 1);
    let current: number | undefined = id;
    while (current !== undefined) {
      totalByNode.set(current, (totalByNode.get(current) ?? 0) + 1);
      current = parentOf.get(current);
    }
  }
}

function buildTopFunctions(
  nodeNames: Map<number, { name: string; location: string | null }>,
  selfByNode: Map<number, number>,
  totalByNode: Map<number, number>,
): TraceFunction[] {
  const byFunction = new Map<string, TraceFunction>();

  for (const [id, meta] of nodeNames) {
    const selfSamples = selfByNode.get(id) ?? 0;
    const totalSamples = totalByNode.get(id) ?? 0;
    if (selfSamples === 0 && totalSamples === 0) continue;

    const key = `${meta.name}@${meta.location ?? ""}`;
    const existing = byFunction.get(key);
    if (existing) {
      existing.selfMs += selfSamples;
      existing.totalMs += totalSamples;
      existing.samples += selfSamples;
      continue;
    }
    byFunction.set(key, {
      name: meta.name,
      location: meta.location,
      selfMs: selfSamples,
      totalMs: totalSamples,
      samples: selfSamples,
      category: categorize(meta.name, meta.location),
    });
  }

  return [...byFunction.values()]
    .map((f) => ({
      ...f,
      selfMs: round(f.selfMs),
      totalMs: round(f.totalMs),
    }))
    .filter((f) => f.selfMs > 0)
    .sort((a, b) => b.selfMs - a.selfMs)
    .slice(0, 25);
}

/**
 * Heuristic categorisation so the report can point at a subsystem
 * (Vue reactivity, layout, paint, our own bundle) rather than a bare name.
 */
function categorize(name: string, location: string | null): string {
  const loc = location ?? "";
  if (loc.includes("/src/") || loc.includes("/assets/")) {
    if (/vue/i.test(loc) || /reactivity/i.test(name)) return "app/vue";
    return "app";
  }
  if (/^(Layout|UpdateLayoutTree|RecalcStyle|Paint|CompositeLayers)/.test(name)) {
    return "rendering";
  }
  if (/GC$/.test(name)) return "gc";
  if (/^(ParseHTML|ParseAuthorStyleSheet|EvaluateScript|CompileScript|v8)/.test(name)) {
    return "parse/compile";
  }
  if (/^(Function\.call|TimerFire|EventDispatch|RunTask|Task)/.test(name)) return "task";
  return "other";
}

function shortUrl(url: string): string {
  try {
    const u = new URL(url);
    const parts = u.pathname.split("/");
    return parts.slice(-3).join("/");
  } catch {
    return url.slice(-80);
  }
}

function buildFindings(input: {
  topFunctions: TraceFunction[];
  forcedLayoutEvents: number;
  forcedLayoutMs: number;
  gcMs: number;
}): string[] {
  const out: string[] = [];
  const top = input.topFunctions[0];
  if (top) {
    out.push(
      `Hottest function: \`${top.name}\` (${top.selfMs} ms self${top.location ? `, ${top.location}` : ""})`,
    );
  }
  if (input.forcedLayoutEvents > 0) {
    out.push(
      `${input.forcedLayoutEvents} forced synchronous layout(s) totalling ${input.forcedLayoutMs} ms — layout thrashing`,
    );
  }
  if (input.gcMs > 50) {
    out.push(`Garbage collection consumed ${input.gcMs} ms`);
  }
  return out;
}

function round(n: number): number {
  return Math.round(n * 100) / 100;
}
