import { Browser, firefox, expect, Page,  } from "@playwright/test";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  // Set basic auth credentials for protected /subpath route
  await page.setExtraHTTPHeaders({
    'Authorization': `Basic ZGVtby0xMjcuMC4wLjE6U2VjdXJlUGFzczEyMyE=`
  });

  await page.goto("http://127.0.0.1/subpath/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - demo-127.0.0.1");

  // Create a share of folder
  await page.locator('button[aria-label="File-Actions"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="File-Actions"]').click();
  await page.locator('button[aria-label="Share"]').click();
  await page.locator('button[aria-label="Share-Confirm"]').click();
  await expect(page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th))")).toHaveCount(1);
  const shareHash = await page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th)) td").first().textContent();
  if (!shareHash) {
    throw new Error("Failed to retrieve shareHash");
  }
  // Store shareHash in localStorage
  await page.evaluate((hash) => {
    localStorage.setItem('shareHash', hash);
  }, shareHash);

  await context.storageState({ path: "./loginAuth.json" });
  await browser.close();
}

export default globalSetup;