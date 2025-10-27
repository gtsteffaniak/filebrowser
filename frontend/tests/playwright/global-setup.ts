import { Browser, firefox, expect, Page } from "@playwright/test";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  await page.goto("http://127.0.0.1/login");
  await page.getByPlaceholder("Username").fill("admin");
  await page.getByPlaceholder("Password").fill("admin");
  await page.getByRole("button", { name: "Login" }).click();
  await page.waitForURL("**/files/", { timeout: 1000 });

  const cookies = await context.cookies();
  expect(cookies.find((c) => c.name === "filebrowser_quantum_jwt")?.value).toBeDefined();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  await page.waitForURL("**/files/playwright%20+%20files", { timeout: 1000 });

  // Create a share of folder
  await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="myfolder"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Share"]').click();
  await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /myfolder/');
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

  await page.goto("http://127.0.0.1/files/playwright%20%2B%20files", { timeout: 500 });
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

  // Create a share of folder "/share"
  await page.goto("http://127.0.0.1/files/playwright%20%2B%20files", { timeout: 500 });
  await page.locator('a[aria-label="share"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="share"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Share"]').click();
  await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /share/');
  // Toggle "Allow creating and uploading files and folders" setting
  await page.locator('input[aria-label="allow creating and uploading files and folders toggle"]').waitFor({ state: 'attached' });
  await page.locator('input[aria-label="allow creating and uploading files and folders toggle"] + .slider').click();

  // Toggle "Allow creating and uploading files and folders" setting
  await page.locator('input[aria-label="allow editing files toggle"]').waitFor({ state: 'attached' });
  await page.locator('input[aria-label="allow editing files toggle"] + .slider').click();

  await page.locator('button[aria-label="Share-Confirm"]').click();
  await expect(page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th))")).toHaveCount(1);
  const shareHashShare = await page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th)) td").first().textContent();
  if (!shareHashShare) {
    throw new Error("Failed to retrieve shareHash");
  }
  // Store shareHash in localStorage
  await page.evaluate((hash) => {
    localStorage.setItem('shareHashShare', hash);
  }, shareHashShare);

  await context.storageState({ path: "./loginAuth.json" });
  await browser.close();
}

export default globalSetup;