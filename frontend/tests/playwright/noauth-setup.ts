import type { Browser, Page } from "@playwright/test";
import { expect, firefox } from "@playwright/test";
import { createShareAndGetHash } from "./test-setup";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  await page.goto("http://127.0.0.1/files/");
  await page.waitForURL("**/files/", { timeout: 1000 });

  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await createShareAndGetHash(page, "Path: /myfolder/", async () => {
    await page.locator('a[aria-label="myfolder"]').waitFor({ state: "visible" });
    await page.locator('a[aria-label="myfolder"]').click({ button: "right" });
    await page.locator(".selected-count-header").waitFor({ state: "visible" });
    await expect(page.locator(".selected-count-header")).toHaveText("1");
    await page.locator('button[aria-label="Share"]').click();
  });
  await page.evaluate((hash) => {
    localStorage.setItem("shareHash", hash);
  }, shareHash);

  await page.goto("http://127.0.0.1/files/", { timeout: 1000 });

  const shareHashFile = await createShareAndGetHash(page, "Path: /1file1.txt", async () => {
    await page.locator('a[aria-label="1file1.txt"]').waitFor({ state: "visible" });
    await page.locator('a[aria-label="1file1.txt"]').click({ button: "right" });
    await page.locator(".selected-count-header").waitFor({ state: "visible" });
    await expect(page.locator(".selected-count-header")).toHaveText("1");
    await page.locator('button[aria-label="Share"]').click();
  });
  await page.evaluate((hash) => {
    localStorage.setItem("shareHashFile", hash);
  }, shareHashFile);

  await context.storageState({ path: "./noauth.json" });
  await browser.close();
}

export default globalSetup;
