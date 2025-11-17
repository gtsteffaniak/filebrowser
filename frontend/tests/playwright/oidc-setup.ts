import { Browser, firefox, expect, Page } from "@playwright/test";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  // The final URL we expect to land on after the login dance.
  const finalUrl = "http://127.0.0.1/files/playwright-files";
  const oidcLoginUrl = `http://127.0.0.1/api/auth/oidc/login?redirect=${encodeURIComponent("/files")}`;

  // Go directly to the OIDC login URL. This will start the redirect chain.
  await page.goto(oidcLoginUrl);

  // The mock OIDC provider will auto-login and redirect back.
  // We just need to wait for the final landing page.
  await page.waitForURL(finalUrl, { timeout: 5000 });

  // Final check to ensure we are on the correct page.
  await expect(page).toHaveURL(finalUrl);

  const cookies = await context.cookies();
  expect(cookies.find((c) => c.name === "filebrowser_quantum_jwt")?.value).toBeDefined();

  await context.storageState({ path: "./loginAuth.json" });
  await browser.close();
}

export default globalSetup;