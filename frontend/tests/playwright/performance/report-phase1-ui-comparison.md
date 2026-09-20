# Phase 1 UI vs pre-change (chromium medians, ms)

| Scale | Scenario | Before (17:08 run) | After (17:17 run) | Δ |
|------:|----------|-------------------:|------------------:|--:|
| 100 | load | 817 | 1274 | +56% |
| 100 | scroll | 454 | 618 | +36% |
| 1000 | load | 2486 | 3863 | +55% |
| 1000 | scroll | 673 | 860 | +28% |
| 10000 | load | 12552 | 27871 | +122% |
| 10000 | scroll | 2869 | 3092 | +8% |
| 10000 | resize | 3906 | 5923 | +52% |
| 10000 | select | 10578 | 12284 | +16% |

## Structural (chromium @10k load)

| Metric | Before | After |
|--------|-------:|------:|
| IntersectionObservers | 20,000 | **1** |
| `updateScrollableContent` in load trace top-10 | ~1889 ms self | **not in top 10** |

## Notes

- Wall-clock medians **worsened** on this pair of runs (high variance: @10k load samples were 27871 / 18429 / 29294).
- Baseline comparison in `report.md` compares **current run vs committed `perf-baseline.json`**, not vs the pre-UI snapshot above.
- Re-run `make perf-baseline-docker` after a stable perf-check if you want CI gates aligned with post-UI timings.
