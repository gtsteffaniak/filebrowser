# Listing performance harness

Automated, cross-browser profiling of progressively larger directory listings,
with a committed **chromium baseline that gates CI**.

## Quick start

```bash
make perf-smoke       # ~10s: chromium only, scales 100/1000, one pass
make perf-check  # full local sweep: all 3 browsers (docker)
make perf-baseline      # regenerate the committed chromium baseline
make perf-dashboard     # view results at http://127.0.0.1:9323/dashboard/report.html
```

## How it works

Each scale (default `100,1000,10000` dirs + the same number of files) runs one
full listing mount, then measures four scenarios:

| Scenario | What it measures |
| --- | --- |
| `load` | cold mount of the listing |
| `scroll` | scroll stress, frame health |
| `resize` | viewport changes forcing layout |
| `select` | Ctrl/Meta multi-select reactivity |

Results are written per run, aggregated at teardown, and assembled into
`report.json` plus a human-readable `report.md`.

## CI vs local

| | Browsers | Repeats | Gating |
| --- | --- | --- | --- |
| **CI** (`Dockerfile.playwright-performance`, `ci` target) | chromium only | N=1 | **enforced** |
| **Local** (`make perf-check`) | chromium, firefox, webkit | N=3 (median) | advisory |

Only chromium feeds the baseline, because only chromium runs in CI. Firefox and
webkit results are marked `baselineEligible: false` and provide cross-browser
comparison insight without affecting the gate.

## Baseline

`perf-baseline.json` is committed and is the reference CI compares against. It is
chromium-only and keyed by `chromium@<scale>:<scenario>`.

Each metric entry stores its value plus the rules used to judge a regression:

- **`toleranceClass`** — `shape` (deterministic structural counts, **10%**
  tolerance), `timing` (wall-clock, **100%**), or `health` (noisy, **100%**).
- **`direction`** — `higher-is-better` metrics (e.g. effective FPS) invert the
  comparison.
- **`noiseFloor`** — absolute delta below which a change is ignored.
- **`gating`** — whether the metric may fail the build.
- **`min`/`max`/`spreadPct`** — observed range across the baseline's iterations,
  so the report shows how noisy each metric actually is.

### Which metrics gate, and why

Tolerances and gating flags are set from **measured spread**, not guesswork. A
baseline captured at N=3 showed a clean dividing line:

| Metrics | Observed spread | Gating |
| --- | --- | --- |
| `domNodes`, `listingItems`, `addEventListener` calls, `intersectionObservers`, `jSEventListeners`, `layoutCountDelta` | **0%** | ✅ gated, 10% |
| `jSHeapUsedSize` | ~3% | ✅ gated, 100% |
| `scenarioMs` | up to 75% | ✅ gated, 100% |
| `longTaskMs`, `longTaskCount`, frame timings, CDP duration deltas | 66% – 750% | ⚠️ **advisory only** |

The advisory metrics are still measured and reported — they are simply not
allowed to fail a build. Long-task totals in particular are bimodal at N=1: a
run either trips the browser's 50 ms threshold or it does not, so identical code
can report 0 ms one run and several seconds the next. Gating on that fires on
scheduling luck, and a flaky gate trains people to ignore it. The structural
counts are the reliable signal and are what actually protect against the
regressions this harness exists to catch.

### Regenerating

```bash
make perf-baseline
```

Run this **in the same environment CI uses**, since the baseline records worker
count, browser version and Playwright version. A mismatch is reported in the
report rather than silently producing a bogus regression.

## What gets measured

Beyond scenario wall-clock time:

- **Structural shape** — DOM nodes, listing rows, JS event listeners,
  IntersectionObservers created, `addEventListener` calls. Deterministic, so
  these use a tight 10% tolerance and are the most reliable regression signal.
- **Renderer attribution (chromium)** — CDP `Performance.getMetrics` sampled
  *before and after* each scenario. Cumulative counters (`LayoutDuration`,
  `ScriptDuration`, `RecalcStyleDuration`, `LayoutCount`) are reported as deltas
  over the scenario window; gauges (`Nodes`, `JSEventListeners`, heap) as
  absolutes. A single post-hoc read always returned 0 for the counters.
- **Frame health** — real frame timings, not rAF callback rate. Prefers
  `long-animation-frame` (chromium), falls back to `paint` then rAF, and reports
  p50/p95/p99, dropped frames and effective FPS with the sampling source noted.
- **Web vitals (all browsers)** — FCP, LCP, CLS, TTFB, DOM content loaded,
  navigation and resource timing, plus interaction (INP-style) latencies for
  `select`/`scroll`. Unsupported metrics are listed in `unsupported[]`.
- **Chromium trace analysis** — `browser.startTracing` output aggregated into
  hottest functions by self time, forced synchronous layouts (layout thrashing),
  and GC cost. Distinct from Playwright Tracing, which works on all engines and
  is enabled with `PERF_PLAYWRIGHT_TRACE=1`.

Long tasks are attributed to the scenario window they occurred in, so a `select`
run no longer reports the load phase's long tasks.

## Adding a metric

1. Add an entry to `METRIC_REGISTRY` in `perf-metrics.ts` with its direction,
   tolerance class, noise floor and scenarios.
2. Map it in `extractBaselineMetrics` (`perf-extract.ts`) if it is not a
   top-level scalar.

The baseline writer, comparison engine and report all read the registry, so the
metric then flows everywhere automatically.

## Reproducibility

- Mock data is **seeded** (`mock.go` derives a stable seed from the requested
  shape; `/api/mock-data` accepts an optional `seed`), so JSON payload size and
  DOM text are byte-identical between runs.
- Each repeat runs in a **fresh page**. Re-mounting the listing in the same page
  accumulated state: JS event listeners and the CDP heap/nodes gauges are
  page-lifetime measurements, so a second iteration reported exactly double the
  first (`jSEventListeners` spread was 100% before this fix, 0% after).
- Local runs use N=3 with a median (override with `PERF_REPEATS`).
- `report.json` records git SHA, dirty flag, browser version, Playwright version,
  CPU count, worker count, scales and image tag.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `PERF_SCALES` | `100,1000,10000` | Scales to sweep |
| `PERF_BROWSERS` | all (CI: chromium) | Comma-separated browser subset |
| `PERF_REPEATS` | `3` local / `1` CI | Iterations per scenario (median) |
| `PERF_WORKERS` | min(scales, 3) | Playwright workers |
| `PERF_SELECT_COUNT` | per-scale | Items selected in the select scenario |
| `PERF_UPDATE_BASELINE` | `0` | Set to `1` to rewrite the baseline |
| `PERF_ENFORCE_BASELINE` | CI only | Set to `1` to fail the run on regression |
| `PERF_FULL_TRACE` | `0` | Trace every scenario, not just `load` |
| `PERF_PLAYWRIGHT_TRACE` | `0` | Enable Playwright Tracing (all browsers) |
| `PERF_MOCK_SEED` | derived | Override the mock-data seed |
| `PERF_IMAGE_TAG` | — | Stamped into the baseline fingerprint |

## Failure modes this guards against

- **No result files** — teardown fails loudly instead of writing an empty report.
- **Partial run** — a missing `(browser, scale, scenario)` tuple fails the run.
- **Noisy metrics** — metrics declared `gating: false` are reported but never
  fail the build, so the gate stays trustworthy.
- **Incomparable environment** — a worker-count or scale-set mismatch skips
  gating and explains why.
