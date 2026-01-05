import { test, expect, checkForNotification } from "../test-setup";

test("root share path is valid", async ({ page, checkForErrors, openContextMenu, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await openContextMenu();
  await page.locator('button[aria-label="Share"]').click();
  await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /');
  checkForErrors();
});


test("share file works", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  const shareHashFile = await page.evaluate(() => localStorage.getItem('shareHashFile'));
  if (shareHashFile == "") {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/share/" + shareHashFile);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - 1file1.txt");
  checkForErrors(0,1); // redirect errors are expected
});

test("share download single file", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (!shareHash) {
    throw new Error("Share hash not found in localStorage");
  }

  await page.goto("/share/" + shareHash + "/testdata/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await page.locator('a[aria-label="gray-sample.jpg"]').click({ button: "right" });
  await page.locator('button[aria-label="Download"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Download"]').click();
  await checkForNotification(page, "Downloading...");
  checkForErrors(0,1);
});

test("share private source", async ({ page, checkForErrors, openContextMenu }) => {
  await page.goto("/files/docker");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - backend");
   // Create a share of folder
   await openContextMenu();
   await page.locator('button[aria-label="Share"]').click();
   await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /');
   await page.locator('button[aria-label="Share-Confirm"]').click();
  await expect(page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th))")).toHaveCount(0);
  await checkForNotification(page, "403: the target source is private, sharing is not permitted");
  checkForErrors(1,1); // 1 error is expected for the private source
});


test("share file creation actions", async ({ page, checkForErrors, openContextMenu }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  const rootShareHash = await page.evaluate(() => localStorage.getItem('rootShareHash'));
  if (rootShareHash == "") {
    throw new Error("Share hash not found in localStorage");
  }
  await page.goto("public/share/" + rootShareHash);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - playwright-files");
  await page.waitForTimeout(1000);
  await openContextMenu();
  await page.locator('button[aria-label="New file"]').click();
  await page.locator('input[aria-label="FileName Field"]').waitFor({ state: 'visible' });
  await page.locator('input[aria-label="FileName Field"]').fill('dfsaf.txt');
  await page.locator('button[aria-label="Create"]').click();
  // Note: Share links don't show the "go to item" button, so no need to click notification
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - playwright-files");
  await page.locator('a[aria-label="dfsaf.txt"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - dfsaf.txt");
  await page.locator(".ace_content").click();
  await page.keyboard.type("test content");
  await page.locator(".overflow-menu-button").click();
  await page.locator('button[aria-label="Save"]').click();
  await checkForNotification(page, "dfsaf.txt saved successfully.");
  checkForErrors();
});