import { defineConfig, devices } from "@playwright/test";

const baseURL =
  process.env.PERF_BASE_URL ??
  (process.env.CI ? "http://127.0.0.1/" : "http://127.0.0.1:8080/");

const scaleCount = (process.env.PERF_SCALES ?? "100,1000,10000")
  .split(",")
  .filter((s) => s.trim()).length;

/**
 * Default worker count.
 *
 * Kept low deliberately: multiple browsers hammering the same single-process
 * backend inflates every timing, and the baseline records the worker count so a
 * mismatch is reported rather than silently corrupting comparisons.
 */
const defaultWorkers = Math.min(Math.max(scaleCount, 1), 3);

const ALL_BROWSERS = [
  { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  { name: "firefox", use: { ...devices["Desktop Firefox"] } },
  { name: "webkit", use: { ...devices["Desktop Safari"] } },
] as const;

/**
 * Browser selection.
 *
 * CI sets `PERF_BROWSERS=chromium` (see Dockerfile.playwright-performance, `ci`
 * target) because only chromium results are baseline-gated. Locally the default
 * is all three, for cross-browser comparison. A comma-separated list narrows
 * the set, which is what the local smoke runs and the `perf-smoke` target use.
 */
const requested = (process.env.PERF_BROWSERS ?? "")
  .split(",")
  .map((b) => b.trim().toLowerCase())
  .filter(Boolean);
const projects =
  requested.length > 0
    ? ALL_BROWSERS.filter((b) => requested.includes(b.name))
    : [...ALL_BROWSERS];

export default defineConfig({
  globalSetup: "./tests/playwright/performance/performance-global-setup.ts",
  globalTeardown: "./tests/playwright/performance/performance-global-teardown.ts",
  outputDir: "test-results/playwright-performance",
  timeout: Number(process.env.PERF_TEST_TIMEOUT_MS ?? "300000"),
  testDir: "./tests/playwright/performance",
  testMatch: /listing-performance\.spec\.ts/,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: Number(process.env.PERF_WORKERS ?? defaultWorkers),
  reporter: [
    ["list"],
    [
      "json",
      { outputFile: "test-results/playwright-performance/playwright-report.json" },
    ],
  ],
  use: {
    actionTimeout: 60_000,
    storageState: "loginAuth.json",
    baseURL,
    // Playwright Tracing works on chromium, firefox and webkit, unlike
    // `browser.startTracing` (a Chromium-only low-level API used inside the
    // scenarios for flame charts). Enabled with PERF_PLAYWRIGHT_TRACE=1.
    trace: process.env.PERF_PLAYWRIGHT_TRACE === "1" ? "on" : "off",
    video: "off",
    locale: "en-US",
  },
  projects,
  webServer: process.env.CI
    ? undefined
    : {
        command:
          "cd ../backend && FILEBROWSER_PLAYWRIGHT_TEST=true ./filebrowser -c config.playwright-performance.yaml",
        url: `${baseURL}api/health`,
        reuseExistingServer: true,
        timeout: 120_000,
      },
});
