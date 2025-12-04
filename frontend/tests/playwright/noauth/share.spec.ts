
import { test, expect, checkForNotification } from "../test-setup";

test("breadcrumbs navigation checks for shares", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/exclude/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (shareHash == "") {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/files/share/" + shareHash);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - myfolder");
  await page.dblclick('a[aria-label="testdata"]');
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await page.waitForSelector('#breadcrumbs');

  // Ensure no <span> children exist directly under #breadcrumbs (ie no breadcrumbs paths)
  let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);

  checkForErrors(0,1); // redirect errors are expected and 404 image preview for blank file
});

test("root share path is valid", async ({ page, checkForErrors, openContextMenu, context }) => {
  await page.goto("/files/exclude/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await openContextMenu();
  await page.locator('button[aria-label="Share"]').click();
  await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /');
  checkForErrors();
});

test("share file works", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/exclude/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  const shareHashFile = await page.evaluate(() => localStorage.getItem('shareHashFile'));
  if (shareHashFile == "") {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/files/share/" + shareHashFile);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - 1file1.txt");
  checkForErrors(0,1); // redirect errors are expected
});

test("share download single file", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/exclude/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (!shareHash) {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/files/share/" + shareHash + "/testdata/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await page.locator('a[aria-label="gray-sample.jpg"]').click({ button: "right" });
  await page.locator('button[aria-label="Download"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Download"]').click();
  await checkForNotification(page, "Downloading...");
  checkForErrors(0,1); // redirect errors are expected
});