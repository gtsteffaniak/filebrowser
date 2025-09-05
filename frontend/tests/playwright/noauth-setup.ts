import { Browser, firefox, expect, Page } from "@playwright/test";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  await page.goto("http://127.0.0.1/files");
  await page.waitForURL("**/files/", { timeout: 1000 });

  // Create a share of folder
  await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="myfolder"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Share"]').click();
  await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /myfolder');
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

  await page.goto("http://127.0.0.1/files", { timeout: 500 });
  // Create a share of file
  await page.locator('a[aria-label="1file1.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="1file1.txt"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Share"]').click();
  await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /1file1.txt');
  await page.locator('button[aria-label="Share-Confirm"]').click();
  await expect(page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th))")).toHaveCount(1);
  const shareHashFile = await page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th)) td").first().textContent();
  if (!shareHashFile) {
    throw new Error("Failed to retrieve shareHash");
  }
  // Store shareHash in localStorage
  await page.evaluate((hash) => {
    localStorage.setItem('shareHashFile', hash);
  }, shareHashFile);


  await context.storageState({ path: "./noauth.json" });
  await browser.close();
}

export default globalSetup;