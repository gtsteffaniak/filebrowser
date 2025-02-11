
import { test, expect } from "@playwright/test";

test("navigate with hash in file name", async ({ page, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    await page.locator('a[aria-label="folder#hash"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="folder#hash"]').dblclick();
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - folder#hash");
    await page.locator('a[aria-label="file#.sh"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="file#.sh"]').dblclick();
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - file#.sh");
    await expect(page.locator('.topTitle')).toHaveText('file#.sh');
  })