import { chromium, expect } from "@playwright/test";
import { loginPlaywrightAdmin } from "../playwright-auth";
import { warmPerfConfig } from "./perf-config";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { perfResultsDir } from "./perf-helpers";

/**
 * Launch chromium with an actionable error.
 *
 * The raw Playwright failure ("Executable doesn't exist at /root/.cache/...")
 * does not say which environment is misconfigured, which is confusing when the
 * paths in the message belong to a container rather than the current host.
 * We surface the runtime context and the exact remedy instead.
 */
async function launchChromium() {
  try {
    return await chromium.launch();
  } catch (err) {
    const message = String((err as Error)?.message ?? err);
    const missingBrowser = /Executable doesn't exist|Looks like Playwright was just installed/i.test(
      message,
    );
    if (!missingBrowser) throw err;

    const inContainer = process.env.CI === "true" || process.env.PERF_IN_CONTAINER === "1";
    const remedy = inContainer
      ? "Playwright and browser binaries are out of sync in the perf image. Rebuild:\n" +
        "  make test-playwright-performance\n" +
        "or locally: make perf-check (rebuilds playwright-perf-base if needed)."
      : "Install browsers for this Playwright version:\n" +
        "  cd frontend && npx playwright install chromium\n\n" +
        "If you meant to use the containerised run instead, use:\n" +
        "  make perf-check   (builds and runs everything in docker)";

    throw new Error(
      [
        "[perf] Could not launch chromium for the performance harness.",
        `  runtime:      ${inContainer ? "container" : "host"}`,
        `  cwd:          ${process.cwd()}`,
        `  node:         ${process.version}`,
        `  PERF_BASE_URL: ${process.env.PERF_BASE_URL ?? "(default)"}`,
        "",
        remedy,
        "",
        `Original error: ${message.split("\n")[0]}`,
      ].join("\n"),
    );
  }
}

/**
 * Authenticate once and capture the browser version.
 *
 * Browser and Playwright versions are part of the baseline fingerprint: a
 * version bump can legitimately move timings, so the report must record which
 * engine produced the numbers.
 */
async function globalSetup() {
  await warmPerfConfig();
  const baseURL =
    process.env.PERF_BASE_URL ??
    (process.env.CI ? "http://127.0.0.1/" : "http://127.0.0.1:8080/");
  const browser = await launchChromium();
  const context = await browser.newContext();
  const page = await context.newPage();

  // Record versions for the report/baseline fingerprint.
  const browserVersion = browser.version();
  const versionFile = path.join(perfResultsDir(), "..", "perf-browser-version.json");
  // The results dir may not exist yet on a cold run.
  await mkdir(perfResultsDir(), { recursive: true });
  await writeFile(
    versionFile,
    JSON.stringify({ browserVersion, capturedAt: new Date().toISOString() }, null, 2),
  );

  await page.goto(`${baseURL}login`);
  await loginPlaywrightAdmin(page);
  await page.waitForURL("**/files/**", { timeout: 30_000 });

  const cookies = await context.cookies();
  expect(
    cookies.find((c) => c.name === "filebrowser_quantum_jwt")?.value,
  ).toBeDefined();

  await context.storageState({ path: "loginAuth.json" });
  await browser.close();
}

export default globalSetup;
