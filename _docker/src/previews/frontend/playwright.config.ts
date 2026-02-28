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
  timeout: 10000,
  testDir: "./tests/playwright/previews",
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: false,
  /* Retry on CI only */
  retries: 2,
  workers: 10,
  reporter: "line",
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    launchOptions: {
      args: [
        '--disable-gpu',
        '--use-gl=swiftshader',  // CPU-based WebGL renderer built into Chromium
        '--enable-webgl',
        '--disable-dev-shm-usage',  // Overcome limited resource problems in Docker
        '--no-sandbox',  // Required for Docker
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
