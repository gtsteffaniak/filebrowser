
import { test, expect } from "../test-setup";

test("breadcrumbs navigation checks", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (shareHash == "") {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/share/" + shareHash);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - myfolder");
  await expect(page.locator('a[aria-label="Home"]')).toHaveAttribute("href", "/share/"+shareHash);
  let breadCrumbLink = page.locator('span[aria-label="breadcrumb-link-myfolder"] a')

  await page.dblclick('a[aria-label="testdata"]');
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  // Ensure no <span> children exist directly under .breadcrumbs (ie no breadcrumbs paths)
  let spanChildrenCount = await page.locator('.breadcrumbs > span').count();
  await page.waitForSelector('.breadcrumbs');
  expect(spanChildrenCount).toBe(0);

  checkForErrors();
});