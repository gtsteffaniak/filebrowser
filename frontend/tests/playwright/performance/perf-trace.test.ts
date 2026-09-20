import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { analyzeTrace } from "./perf-trace";

async function writeTrace(events: unknown[]): Promise<string> {
  const dir = await mkdtemp(path.join(tmpdir(), "perf-trace-"));
  const file = path.join(dir, "trace.json");
  await writeFile(file, JSON.stringify(events));
  return file;
}

describe("analyzeTrace", () => {
  it("returns null for a missing file", async () => {
    expect(await analyzeTrace("/nonexistent/trace.json")).toBeNull();
  });

  it("returns null for malformed JSON", async () => {
    const dir = await mkdtemp(path.join(tmpdir(), "perf-trace-"));
    const file = path.join(dir, "bad.json");
    await writeFile(file, "{not json");
    expect(await analyzeTrace(file)).toBeNull();
  });

  it("returns null for an empty event list", async () => {
    const file = await writeTrace([]);
    expect(await analyzeTrace(file)).toBeNull();
  });

  it("counts events and computes the trace span", async () => {
    // Chrome trace timestamps are microseconds.
    const file = await writeTrace([
      { ph: "X", name: "Task", cat: "devtools.timeline", ts: 1_000_000, dur: 2_000_000 },
      { ph: "X", name: "Task", cat: "devtools.timeline", ts: 4_000_000, dur: 1_000_000 },
    ]);
    const analysis = await analyzeTrace(file);
    expect(analysis).not.toBeNull();
    expect(analysis!.eventCount).toBe(2);
    // Earliest start 1.0s to latest end 5.0s => 4000 ms.
    expect(analysis!.spanMs).toBe(4000);
  });

  it("extracts CPU profile function attribution", async () => {
    const file = await writeTrace([
      {
        name: "ProfileChunk",
        args: {
          data: {
            cpuProfile: {
              nodes: [
                {
                  id: 1,
                  callFrame: {
                    functionName: "renderRows",
                    url: "http://localhost/assets/index-abc.js",
                    lineNumber: 41,
                  },
                },
                {
                  id: 2,
                  callFrame: {
                    functionName: "handleIntersect",
                    url: "http://localhost/assets/index-abc.js",
                    lineNumber: 99,
                  },
                },
              ],
              samples: [1, 1, 1, 2],
            },
          },
        },
      },
    ]);

    const analysis = await analyzeTrace(file);
    expect(analysis).not.toBeNull();
    const names = analysis!.topFunctions.map((f) => f.name);
    expect(names).toContain("renderRows");
    // renderRows had 3 of 4 samples, so it ranks first by self time.
    expect(analysis!.topFunctions[0].name).toBe("renderRows");
    expect(analysis!.topFunctions[0].selfMs).toBe(3);
  });

  it("counts GC events", async () => {
    const file = await writeTrace([
      { ph: "X", name: "MajorGC", cat: "v8", ts: 0, dur: 50000 },
      { ph: "X", name: "MinorGC", cat: "v8", ts: 60000, dur: 20000 },
    ]);
    const analysis = await analyzeTrace(file);
    expect(analysis!.gcEvents).toBe(2);
    expect(analysis!.gcMs).toBe(70);
  });

  it("records long slices over 50ms", async () => {
    const file = await writeTrace([
      { ph: "X", name: "FunctionCall", cat: "devtools.timeline", ts: 0, dur: 100_000 },
      { ph: "X", name: "Small", cat: "devtools.timeline", ts: 200_000, dur: 1_000 },
    ]);
    const analysis = await analyzeTrace(file);
    expect(analysis!.longTaskSlices).toHaveLength(1);
    expect(analysis!.longTaskSlices[0].durationMs).toBe(100);
  });

  it("produces an actionable finding for the hottest function", async () => {
    const file = await writeTrace([
      {
        name: "ProfileChunk",
        args: {
          data: {
            cpuProfile: {
              nodes: [
                {
                  id: 7,
                  callFrame: {
                    functionName: "expensiveSort",
                    url: "http://localhost/assets/index-abc.js",
                    lineNumber: 10,
                  },
                },
              ],
              samples: [7, 7],
            },
          },
        },
      },
    ]);
    const analysis = await analyzeTrace(file);
    expect(analysis!.findings.some((f) => f.includes("expensiveSort"))).toBe(true);
  });

  it("accepts the {traceEvents:[...]} envelope form", async () => {
    const dir = await mkdtemp(path.join(tmpdir(), "perf-trace-"));
    const file = path.join(dir, "envelope.json");
    await writeFile(
      file,
      JSON.stringify({
        traceEvents: [
          { ph: "X", name: "Task", cat: "devtools.timeline", ts: 0, dur: 1000 },
        ],
      }),
    );
    const analysis = await analyzeTrace(file);
    expect(analysis!.eventCount).toBe(1);
  });
});
