import { defineConfig, devices } from "@playwright/test";

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// require('dotenv').config();

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  globalSetup: "./tests/playwright/previews-setup",
  timeout: 45_000,
  testDir: "./tests/playwright/previews",
  fullyParallel: false,
  forbidOnly: false,
  retries: 2,
  workers: 1,
  reporter: "line",
  use: {
    launchOptions: {
      args: [
        "--autoplay-policy=no-user-gesture-required",
        "--disable-gpu",
        "--use-gl=swiftshader",
        "--enable-webgl",
        "--disable-dev-shm-usage",
        "--no-sandbox",
      ],
    },
    storageState: "previews.json",
    actionTimeout: 5000,
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: "http://127.0.0.1",

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",

    /* Set default locale to English (US) */
    locale: "en-US",
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
