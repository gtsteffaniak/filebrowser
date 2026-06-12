import type { Browser, Page } from "@playwright/test";
import { expect, firefox } from "@playwright/test";
import { createShareAndGetHash, openContextMenuHelper } from "./test-setup";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  // Set basic auth credentials for protected /subpath route
  await page.setExtraHTTPHeaders({
    Authorization: "Basic ZGVtby0xMjcuMC4wLjE6U2VjdXJlUGFzczEyMyE=",
  });

  await page.goto("http://127.0.0.1/subpath/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - demo-127.0.0.1");

  const shareHash = await createShareAndGetHash(page, "Path: /", async () => {
    await openContextMenuHelper(page);
    await page.locator('button[aria-label="Share"]').click();
  });
  await page.evaluate((hash) => {
    localStorage.setItem("shareHash", hash);
  }, shareHash);

  await context.storageState({ path: "./loginAuth.json" });
  await browser.close();
}

export default globalSetup;
