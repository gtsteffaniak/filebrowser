import { test } from "@playwright/test";
import { installProbes, repeatCount } from "./perf-helpers";
import { parsePerfScales, runAllScenariosForScale } from "./perf-scenarios";

/**
 * Each scale is independent, so scales run in parallel. `scale` is passed
 * explicitly through every call — it is not held in module state, which removes
 * the race where a sibling worker could change the expected row count.
 */
test.describe.configure({ mode: "parallel" });

const scales = parsePerfScales();
const repeats = repeatCount();

for (const scale of scales) {
  test(`listing perf @ ${scale}+${scale}`, async ({ page, browser }, testInfo) => {
    // Budget scales with the work: repeats multiply the per-scale cost.
    const base = scale >= 10000 ? 480_000 : scale >= 1000 ? 180_000 : 90_000;
    testInfo.setTimeout(base * repeats);

    await installProbes(page);

    // Each repeat after the first gets a fresh page, so listener counts and CDP
    // gauges (both page-lifetime measurements) are not inflated by prior
    // iterations.
    const freshPage = async () => {
      const context = page.context();
      const next = await context.newPage();
      return next;
    };

    await runAllScenariosForScale(page, browser, testInfo, scale, freshPage);
  });
}
