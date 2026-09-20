# Listing performance report

Generated: 2026-09-20T17:55:56.071Z
Revision: not captured (the harness at this time did not record git metadata)
Environment: chromium 153.0.8010.12, playwright 1.63.0, 16 CPUs, workers=6, scales=[100, 1000, 10000]
Provenance: historical artifact, committed in `af0b59d7`; retained as evidence, superseded by reports that record the measured revision.

## Headline findings

- **critical** Long-task time in scenario regressed +3187.29% (baseline 3801 → 124950, limit +100%)
- **critical** Long-task time in scenario regressed +2412.82% (baseline 1989 → 49980, limit +100%)
- **critical** Script time (CDP) regressed +2324.26% (baseline 136 → 3297, limit +100%)
- **critical** Script time (CDP) regressed +1800% (baseline 3 → 57, limit +100%)
- **critical** Script time (CDP) regressed +1650% (baseline 2 → 35, limit +100%)
- **critical** Long-task time in scenario regressed +999% (baseline 0 → 5714, limit +100%)
- **critical** Long tasks in scenario regressed +999% (baseline 0 → 3, limit +100%)
- **critical** Long-task time in scenario regressed +999% (baseline 0 → 3218, limit +100%)
- **critical** Long tasks in scenario regressed +999% (baseline 0 → 19, limit +100%)
- **critical** Long-task time in scenario regressed +999% (baseline 0 → 5800, limit +100%)
- **high** load at 10000 is 4.52x slower on chromium (129255 ms) than webkit (28626 ms)
- **medium** scroll at 10000 is 2.92x slower on firefox (7246 ms) than webkit (2479 ms)
- **medium** select at 10000 is 2.79x slower on chromium (18567 ms) than firefox (6654 ms)
- **info** chromium@100:load: Hottest function: `(program)` (2129 ms self)
- **info** chromium@100:load: Garbage collection consumed 973.34 ms
- **info** chromium@1000:load: Hottest function: `(program)` (2241 ms self)
- **info** chromium@1000:load: Garbage collection consumed 448.82 ms
- **info** chromium@10000:load: Hottest function: `(program)` (5764 ms self)
- **info** chromium@10000:load: Garbage collection consumed 1197.81 ms

## Baseline comparison

216 metrics compared · 47 regressions · 23 improvements · 0 missing

Environment differences vs baseline:
- image playwright-base-chromium != local

> **Scope note:** this legacy table predates per-row scope capture. Rows are
> aggregated across browsers/scales/scenarios and ordered by Δ%, so the metric
> key in parentheses does not identify the failing test. Reports generated after
> this change include Browser, Scale, and Scenario columns.

| Metric | Baseline | Current | Δ% | Limit |
| --- | ---: | ---: | ---: | ---: |
| Long-task time in scenario (longTaskMs) | 3801 | 124950 | +3187.29% | +100% |
| Long-task time in scenario (longTaskMs) | 1989 | 49980 | +2412.82% | +100% |
| Script time (CDP) (scriptDurationDelta) | 136 | 3297 | +2324.26% | +100% |
| Script time (CDP) (scriptDurationDelta) | 3 | 57 | +1800% | +100% |
| Script time (CDP) (scriptDurationDelta) | 2 | 35 | +1650% | +100% |
| Long-task time in scenario (longTaskMs) | 0 | 5714 | +999% | +100% |
| Long tasks in scenario (longTaskCount) | 0 | 3 | +999% | +100% |
| Long-task time in scenario (longTaskMs) | 0 | 3218 | +999% | +100% |
| Long tasks in scenario (longTaskCount) | 0 | 19 | +999% | +100% |
| Long-task time in scenario (longTaskMs) | 0 | 5800 | +999% | +100% |
| Long tasks in scenario (longTaskCount) | 0 | 20 | +999% | +100% |
| Long tasks in scenario (longTaskCount) | 13 | 140 | +976.92% | +100% |
| Long-task time in scenario (longTaskMs) | 11594 | 119775 | +933.08% | +100% |
| Scenario duration (scenarioMs) | 12552 | 129255 | +929.76% | +100% |
| Long tasks in scenario (longTaskCount) | 12 | 116 | +866.67% | +100% |
| Frame time p95 (frameP95) | 16.7 | 110.9 | +564.07% | +100% |
| Frame time p99 (frameP99) | 16.8 | 110.9 | +560.12% | +100% |
| Long-task time in scenario (longTaskMs) | 506 | 2953 | +483.6% | +100% |
| Script time (CDP) (scriptDurationDelta) | 92 | 452 | +391.3% | +100% |
| Style recalc time (CDP) (recalcStyleDurationDelta) | 98 | 462 | +371.43% | +100% |
| Layout time (CDP) (layoutDurationDelta) | 305 | 1008 | +230.49% | +100% |
| Frame time p95 (frameP95) | 16.8 | 55.1 | +227.98% | +100% |
| Style recalc time (CDP) (recalcStyleDurationDelta) | 259 | 822 | +217.37% | +100% |
| Scenario duration (scenarioMs) | 1202 | 3788 | +215.14% | +100% |
| Style recalc time (CDP) (recalcStyleDurationDelta) | 43 | 129 | +200% | +100% |
| Layout time (CDP) (layoutDurationDelta) | 25 | 73 | +192% | +100% |
| Frame time p95 (frameP95) | 1388.5 | 3746.1 | +169.79% | +100% |
| Frame time p99 (frameP99) | 1388.5 | 3746.1 | +169.79% | +100% |
| Style recalc time (CDP) (recalcStyleDurationDelta) | 1398 | 3671 | +162.59% | +100% |
| Scenario duration (scenarioMs) | 2486 | 6418 | +158.17% | +100% |
| Layout time (CDP) (layoutDurationDelta) | 1128 | 2909 | +157.89% | +100% |
| Scenario duration (scenarioMs) | 3906 | 9800 | +150.9% | +100% |
| Scenario duration (scenarioMs) | 817 | 2034 | +148.96% | +100% |
| Script time (CDP) (scriptDurationDelta) | 45 | 112 | +148.89% | +100% |
| Layout time (CDP) (layoutDurationDelta) | 78 | 190 | +143.59% | +100% |
| Frame time p95 (frameP95) | 65.1 | 158.4 | +143.32% | +100% |
| Frame time p99 (frameP99) | 65.1 | 158.4 | +143.32% | +100% |
| Scenario duration (scenarioMs) | 673 | 1630 | +142.2% | +100% |
| Style recalc time (CDP) (recalcStyleDurationDelta) | 165 | 395 | +139.39% | +100% |
| Style recalc time (CDP) (recalcStyleDurationDelta) | 61 | 140 | +129.51% | +100% |
| Scenario duration (scenarioMs) | 454 | 1018 | +124.23% | +100% |
| Script time (CDP) (scriptDurationDelta) | 48 | 107 | +122.92% | +100% |
| Frame time p95 (frameP95) | 149.1 | 327.2 | +119.45% | +100% |
| Frame time p99 (frameP99) | 149.1 | 327.2 | +119.45% | +100% |
| Interaction duration p95 (interactionP95) | 152 | 312 | +105.26% | +100% |
| Interaction duration p95 (interactionP95) | 152 | 312 | +105.26% | +100% |
| Scenario duration (scenarioMs) | 2869 | 5864 | +104.39% | +100% |

## Scenario timings by scale

| Browser | Scale | Load | Scroll | Resize | Select |
| --- | ---: | ---: | ---: | ---: | ---: |
| chromium | 100 | 2034 ms | 1018 ms | 648 ms | 3788 ms |
| chromium | 1000 | 6418 ms | 1630 ms | 1161 ms | 4227 ms |
| chromium | 10000 | 129255 ms | 5864 ms | 9800 ms | 18567 ms |
| firefox | 100 | 2183 ms | 1431 ms | 679 ms | 4858 ms |
| firefox | 1000 | 39896 ms | 2374 ms | 4042 ms | 4331 ms |
| firefox | 10000 | 32254 ms | 7246 ms | 14217 ms | 6654 ms |
| webkit | 100 | 3283 ms | 1060 ms | 2073 ms | 16297 ms |
| webkit | 1000 | 7729 ms | 1122 ms | 4319 ms | 13559 ms |
| webkit | 10000 | 28626 ms | 2479 ms | 10348 ms | 15102 ms |

## Trace analysis (chromium)

### chromium@100:load

Total trace 38181 ms · 13181 events · 25 hot functions

| Function | Self ms | Total ms | Category |
| --- | ---: | ---: | --- |
| `(program)` | 2129 | 2129 | other |
| `updateScrollableContent` | 655 | 655 | app |
| `(garbage collector)` | 414 | 414 | other |
| `xr` | 290 | 290 | app |
| `(idle)` | 178 | 178 | other |
| `(anonymous)` | 148 | 148 | app |
| `(anonymous)` | 143 | 143 | app |
| `Mb` | 89 | 89 | app |
| `fetch` | 79 | 79 | other |
| `xt` | 74 | 74 | app |

### chromium@1000:load

Total trace 21803 ms · 15091 events · 25 hot functions

| Function | Self ms | Total ms | Category |
| --- | ---: | ---: | --- |
| `(program)` | 2241 | 2241 | other |
| `xr` | 1014 | 1014 | app |
| `(garbage collector)` | 936 | 936 | other |
| `(idle)` | 583 | 583 | other |
| `(anonymous)` | 486 | 486 | app |
| `Mb` | 408 | 408 | app |
| `xt` | 290 | 290 | app |
| `addEventListener` | 239 | 239 | other |
| `n` | 224 | 224 | app |
| `insertBefore` | 220 | 220 | other |

### chromium@10000:load

Total trace 44996 ms · 61948 events · 25 hot functions

| Function | Self ms | Total ms | Category |
| --- | ---: | ---: | --- |
| `(program)` | 5764 | 5764 | other |
| `xr` | 5666 | 5666 | app |
| `(garbage collector)` | 4961 | 4961 | other |
| `(anonymous)` | 4755 | 4755 | app |
| `xt` | 2817 | 2817 | app |
| `Mb` | 2184 | 2184 | app |
| `addEventListener` | 1923 | 1923 | other |
| `insertBefore` | 1656 | 1656 | other |
| `vu` | 1258 | 1258 | app |
| `get` | 1229 | 1229 | app |

