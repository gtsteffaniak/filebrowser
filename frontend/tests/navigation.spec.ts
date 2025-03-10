
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

test("breadcrumbs navigation checks", async ({ page }) => {
  await page.goto("/files/");
  await expect(page.locator('a[aria-label="Home"]')).toHaveAttribute("href", "/files/playwright-files");

  // Ensure no <span> children exist directly under .breadcrumbs (ie no breadcrumbs paths)
  let spanChildrenCount = await page.locator('.breadcrumbs > span').count();
  expect(spanChildrenCount).toBe(0);

  await page.goto("/files/playwright-files/myfolder");
  spanChildrenCount = await page.locator('.breadcrumbs > span').count();
  expect(spanChildrenCount).toBe(1);
  let breadCrumbLink = page.locator('span[aria-label="breadcrumb-link-myfolder"] a')
  await expect(breadCrumbLink).toHaveText("myfolder");

  await page.goto("/files/playwright-files/myfolder/testdata");
  spanChildrenCount = await page.locator('.breadcrumbs > span').count();
  expect(spanChildrenCount).toBe(2);
  breadCrumbLink = page.locator('span[aria-label="breadcrumb-link-testdata"] a')
  await expect(breadCrumbLink).toHaveText("testdata");

  await page.goto("/files/playwright-files/files");
  spanChildrenCount = await page.locator('.breadcrumbs > span').count();
  expect(spanChildrenCount).toBe(1);
  breadCrumbLink = page.locator('span[aria-label="breadcrumb-link-files"] a')
  await expect(breadCrumbLink).toHaveText("files");

});
