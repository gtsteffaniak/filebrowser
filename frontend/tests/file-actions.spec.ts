import { test, expect } from "@playwright/test";

test("info from listing", async ({  page, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file.tar.gz"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Info"]').click();
  await expect(page.locator('.break-word')).toHaveText('Display Name: file.tar.gz');
});

test("info from search", async ({ page, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search').click()
  await page.locator('#main-input').fill('file.tar.gz');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="file.tar.gz"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Info"]').click();
  await expect(page.locator('.break-word')).toHaveText('Display Name: file.tar.gz');
})

test("copy from listing", async ({ page, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="copyme.txt"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Copy file"]').click();
  //await expect(page.locator('.searchContext')).toHaveText('Path: /');
  await page.locator('li[aria-label="myfolder"]').click();
  await page.locator('button[aria-label="Copy"]').click();
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("Successfully copied file/folder, redirecting...");
  await page.waitForURL('**/myfolder/');
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - myfolder");
  // verify exists and copy again
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="copyme.txt"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Copy file"]').click();
  //await expect(page.locator('.searchContext')).toHaveText('Path: /');
  await page.locator('li[aria-label="testdata"]').click();
  await page.locator('button[aria-label="Copy"]').click();
  const popup2 = page.locator('#popup-notification-content');
  await popup2.waitFor({ state: 'visible' });
  //await expect(popup2).toHaveText("Successfully copied file/folder, redirecting...");
  //await page.waitForURL('**/testdata/');
  //await expect(page).toHaveTitle("Graham's Filebrowser - Files - testdata");
})