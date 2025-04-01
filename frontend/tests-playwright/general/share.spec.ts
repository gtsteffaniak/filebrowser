
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
  await expect(page.locator('a[aria-label="Home"]')).toHaveAttribute("href", `/share/${shareHash}/`);

  await page.dblclick('a[aria-label="testdata"]');
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  // Ensure no <span> children exist directly under .breadcrumbs (ie no breadcrumbs paths)
  let spanChildrenCount = await page.locator('.breadcrumbs > span').count();
  await page.waitForSelector('.breadcrumbs');
  expect(spanChildrenCount).toBe(1);

  checkForErrors();
});

test("root share path is valid", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('button[aria-label="Show-Share"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Show-Share"]').click();
  await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /');
  checkForErrors();
});

test("share download single file", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (shareHash == "") {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/share/" + shareHash + "/testdata/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await page.locator('a[aria-label="gray-sample.jpg"]').click({ button: "right" });
  await page.locator('button[aria-label="Download"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Download"]').click();
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("Downloading...");
  checkForErrors();
});

test("share download multiple files", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (shareHash == "") {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/share/" + shareHash + "/testdata/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await page.locator('a[aria-label="gray-sample.jpg"]').click({ button: "right" });
  await page.locator('button[aria-label="Select multiple"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Select multiple"]').click();

  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("Multiple Selection Enabled");

  await page.locator('a[aria-label="20130612_142406.jpg"]').click();
  await page.locator('a[aria-label="IMG_2578.JPG"]').click();
  await page.locator('a[aria-label="gray-sample.jpg"]').click({ button: "right" });

  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('3 selected');

  await page.locator('button[aria-label="Download"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Download"]').click();
  const popup2 = page.locator('#popup-notification-content');
  await popup2.waitFor({ state: 'visible' });
  await expect(popup2).toHaveText("Downloading...");
  checkForErrors();
});