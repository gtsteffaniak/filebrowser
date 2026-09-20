# Listing performance report

Generated: 2026-09-20T17:08:55.912Z
Environment: chromium 153.0.8010.12, playwright 1.63.0, 16 CPUs, workers=6, scales=[100, 1000, 10000]

## Headline findings

- **critical** Long tasks in scenario regressed +999% (baseline 0 → 8, limit +100%)
- **high** resize at 10000 is 3.04x slower on webkit (11880 ms) than chromium (3906 ms)
- **high** chromium: one IntersectionObserver per listing row (20000 observers for 20000 rows) — scales linearly with row count
- **high** firefox: one IntersectionObserver per listing row (20000 observers for 20000 rows) — scales linearly with row count
- **high** webkit: one IntersectionObserver per listing row (20000 observers for 20000 rows) — scales linearly with row count
- **medium** load at 10000 is 1.91x slower on firefox (23934 ms) than chromium (12552 ms)
- **medium** scroll at 10000 is 1.78x slower on firefox (4906 ms) than webkit (2751 ms)
- **info** chromium@100:load: Hottest function: `(program)` (1123 ms self)
- **info** chromium@1000:load: Hottest function: `(program)` (3874 ms self)
- **info** chromium@1000:load: Garbage collection consumed 112.73 ms
- **info** chromium@10000:load: Hottest function: `(program)` (28438 ms self)
- **info** chromium@10000:load: Garbage collection consumed 921.52 ms

## Baseline comparison

216 metrics compared · 1 regressions · 4 improvements · 0 missing

| Metric | Baseline | Current | Δ% | Limit |
| --- | ---: | ---: | ---: | ---: |
| Long tasks in scenario (longTaskCount) | 0 | 8 | +999% | +100% |

## Scenario timings by scale

| Browser | Scale | Load | Scroll | Resize | Select |
| --- | ---: | ---: | ---: | ---: | ---: |
| chromium | 100 | 817 ms | 454 ms | 334 ms | 1202 ms |
| chromium | 1000 | 2486 ms | 673 ms | 589 ms | 2773 ms |
| chromium | 10000 | 12552 ms | 2869 ms | 3906 ms | 10578 ms |
| firefox | 100 | 1229 ms | 756 ms | 500 ms | 2106 ms |
| firefox | 1000 | 5776 ms | 1416 ms | 1828 ms | 3356 ms |
| firefox | 10000 | 23934 ms | 4906 ms | 8741 ms | 8501 ms |
| webkit | 100 | 2499 ms | 586 ms | 1969 ms | 10891 ms |
| webkit | 1000 | 5312 ms | 579 ms | 2892 ms | 8874 ms |
| webkit | 10000 | 19179 ms | 2751 ms | 11880 ms | 12396 ms |

## Trace analysis (chromium)

### chromium@100:load

Total trace 1943 ms · 7499 events · 25 hot functions

| Function | Self ms | Total ms | Category |
| --- | ---: | ---: | --- |
| `(program)` | 1123 | 1123 | other |
| `(idle)` | 305 | 305 | other |
| `updateScrollableContent` | 189 | 189 | app |
| `Cn` | 110 | 110 | app |
| `(garbage collector)` | 98 | 98 | other |
| `(anonymous)` | 57 | 57 | app |
| `_b` | 47 | 47 | app |
| `(anonymous)` | 38 | 38 | app |
| `get` | 38 | 38 | app |
| `Gc` | 38 | 38 | app |

### chromium@1000:load

Total trace 6011 ms · 18598 events · 25 hot functions

| Function | Self ms | Total ms | Category |
| --- | ---: | ---: | --- |
| `(program)` | 3874 | 3874 | other |
| `(garbage collector)` | 546 | 546 | other |
| `Cn` | 523 | 523 | app |
| `(anonymous)` | 444 | 444 | app |
| `updateScrollableContent` | 377 | 377 | app |
| `(idle)` | 351 | 351 | other |
| `_b` | 260 | 260 | app |
| `xt` | 202 | 202 | app |
| `get` | 147 | 147 | app |
| `insertBefore` | 138 | 138 | other |

### chromium@10000:load

Total trace 36555 ms · 121127 events · 25 hot functions

| Function | Self ms | Total ms | Category |
| --- | ---: | ---: | --- |
| `(program)` | 28438 | 28438 | other |
| `(garbage collector)` | 4480 | 4480 | other |
| `(anonymous)` | 4184 | 4184 | app |
| `Cn` | 4084 | 4084 | app |
| `updateScrollableContent` | 1889 | 1889 | app |
| `_b` | 1652 | 1652 | app |
| `xt` | 1614 | 1614 | app |
| `addEventListener` | 1292 | 1292 | other |
| `insertBefore` | 1241 | 1241 | other |
| `get` | 1066 | 1066 | app |

