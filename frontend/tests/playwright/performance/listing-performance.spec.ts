import { test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import path from "node:path";
import { installProbes, repeatCount } from "./perf-helpers";
import { frontendRoot } from "./perf-paths";
import {
  parsePerfScales,
  runLoadScenario,
  runResizeScenario,
  runScrollScenario,
  runSelectScenario,
} from "./perf-scenarios";

/** Scales run in parallel; scenarios within a scale share one listing mount (serial). */
test.describe.configure({ mode: "parallel" });

const scales = parsePerfScales();
const repeats = repeatCount();

type PerfSession = {
  context: BrowserContext | null;
  page: Page | null;
};

async function openPerfSession(browser: Browser): Promise<PerfSession> {
  const context = await browser.newContext({
    storageState: path.join(frontendRoot(import.meta.url), "loginAuth.json"),
  });
  const page = await context.newPage();
  await installProbes(page);
  return { context, page };
}

function timeoutForScale(scale: number): number {
  if (scale >= 10000) return 480_000;
  if (scale >= 1000) return 180_000;
  return 90_000;
}

for (const scale of scales) {
  test.describe(`listing @ ${scale}+${scale}`, () => {
    test.describe.configure({ mode: "serial" });

    const session: PerfSession = { context: null, page: null };

    async function pageForIteration(browser: Browser, iteration: number): Promise<Page> {
      if (iteration > 0) {
        await session.context?.close();
        session.context = null;
        session.page = null;
      }
      if (!session.page) {
        const opened = await openPerfSession(browser);
        session.context = opened.context;
        session.page = opened.page;
      }
      return session.page;
    }

    test.afterAll(async () => {
      await session.context?.close();
      session.context = null;
      session.page = null;
    });

    for (let iteration = 0; iteration < repeats; iteration++) {
      const rep =
        repeats > 1 ? ` (repeat ${iteration + 1}/${repeats})` : "";

      test(`load${rep}`, async ({ browser }, testInfo) => {
        testInfo.setTimeout(timeoutForScale(scale));
        const page = await pageForIteration(browser, iteration);
        await runLoadScenario(page, browser, testInfo, scale, iteration);
      });

      test(`scroll${rep}`, async ({ browser }, testInfo) => {
        testInfo.setTimeout(timeoutForScale(scale));
        if (!session.page) {
          throw new Error("[perf] scroll ran before load — serial order broken");
        }
        await runScrollScenario(session.page, browser, testInfo, scale, iteration, {
          reuseListing: true,
        });
      });

      test(`resize${rep}`, async ({ browser }, testInfo) => {
        testInfo.setTimeout(timeoutForScale(scale));
        if (!session.page) {
          throw new Error("[perf] resize ran before load — serial order broken");
        }
        await runResizeScenario(session.page, browser, testInfo, scale, iteration, {
          reuseListing: true,
        });
      });

      test(`select${rep}`, async ({ browser }, testInfo) => {
        testInfo.setTimeout(timeoutForScale(scale));
        if (!session.page) {
          throw new Error("[perf] select ran before load — serial order broken");
        }
        await runSelectScenario(session.page, browser, testInfo, scale, iteration, {
          reuseListing: true,
        });
      });
    }
  });
}
