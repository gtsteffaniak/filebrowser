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
  globalSetup: "./tests/playwright/jwt-setup.ts",
  timeout: 5000,
  testDir: "./tests/playwright/jwt",
  /* Run tests in files in parallel */
  fullyParallel: false,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: false,
  /* Retry on CI only */
  retries: 2,
  /* Opt out of parallel tests on CI. */
  workers: 1, // required for now! todo parallel some tests
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: "line",
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    actionTimeout: 5000,
    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",
    /* Set default locale to English (US) */
    locale: "en-US",
  },

  /* Configure projects for different JWT test scenarios */
  projects: [
    {
      name: "admin-user",
      use: { 
        ...devices["Desktop Firefox"],
        baseURL: "http://127.0.0.1:8081", // Admin user JWT
      },
    },
    {
      name: "regular-user",
      use: { 
        ...devices["Desktop Firefox"],
        baseURL: "http://127.0.0.1:8082", // Regular user JWT
      },
    },
    {
      name: "wrong-key",
      use: { 
        ...devices["Desktop Firefox"],
        baseURL: "http://127.0.0.1:8083", // Wrong secret key
      },
    },
    {
      name: "no-user",
      use: { 
        ...devices["Desktop Firefox"],
        baseURL: "http://127.0.0.1:8084", // No JWT token
      },
    },
  ],
});
