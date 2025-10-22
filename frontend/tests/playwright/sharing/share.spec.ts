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
  await page.dblclick('a[aria-label="testdata"]');
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await page.waitForSelector('#breadcrumbs');
  // Ensure no <span> children exist directly under #breadcrumbs (ie no breadcrumbs paths)
  let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);

  checkForErrors(0,1); // redirect errors are expected
});

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
  await page.goto("/files/files/");
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
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("Downloading...");
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
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("403: the target source is private, sharing is not permitted");
  checkForErrors(1,1); // 1 error is expected for the private source
});


test("share file creation", async ({ page, checkForErrors, openContextMenu }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  const shareHashShare = await page.evaluate(() => localStorage.getItem('shareHashShare'));
  if (shareHashShare == "") {
    throw new Error("Share hash not found in localStorage");
  }
  await page.goto("public/share/" + shareHashShare);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - share");
  await openContextMenu();
  await page.locator('button[aria-label="New file"]').click();
  await page.locator('input[aria-label="FileName Field"]').waitFor({ state: 'visible' });
  await page.locator('input[aria-label="FileName Field"]').fill('dfsaf.txt');
  await page.locator('button[aria-label="Create"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - les - dfsaf.txt");
  await page.locator(".ace_content").click();
  await page.keyboard.type("test content");
  await page.locator(".overflow-menu-button").click();
  await page.locator('button[aria-label="Save"]').click();
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("dfsaf.txt saved successfully.");
  checkForErrors();
});