import { test, expect } from "@playwright/test";

test("file preview", async ({  page, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle('playwright-files - FileBrowser Quantum - Files');
  await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file.tar.gz"]').dblclick();
  await expect(page).toHaveTitle('file.tar.gz - FileBrowser Quantum - Files');
  await page.locator('button[title="Close"]').click();
  await expect(page).toHaveTitle('playwright-files - FileBrowser Quantum - Files');
});