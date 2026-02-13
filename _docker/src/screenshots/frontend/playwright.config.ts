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
  globalSetup: "./tests/playwright/screenshots-setup.ts",
  timeout: 5000,
  testDir: "./tests/playwright/screenshots",
  /* Run tests in files in parallel */
  fullyParallel: false,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: false,
  /* Retry on CI only */
  retries: 2,
  /* Opt out of parallel tests on CI. */
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: "line",
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    actionTimeout: 5000,
    storageState: "loginAuth.json",
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: "http://localhost:8080",

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",

    /* Set default locale to English (US) */
    locale: "en-US",
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: "dark-screenshots",
      use: {
        ...devices["Desktop Firefox"],
        theme: 'dark',
      },
      testMatch: /.*screenshots.spec.ts/,
      retries: 0,
    },
    {
      name: "light-screenshots",
      use: {
        ...devices["Desktop Firefox"],
        theme: 'light',
      },
      testMatch: /.*screenshots.spec.ts/,
      retries: 0,
    },
  ],
});
