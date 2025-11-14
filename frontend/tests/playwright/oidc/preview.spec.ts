import { test, expect } from "../test-setup";

test("Create first new file", async ({  page, checkForErrors, context }) => {
  await page.goto("/");
  await expect(page.locator('#listingView .message > span')).toHaveText('It feels lonely here...');
  await page.locator('#listingView').click({ button: "right" });
  await page.locator('button[aria-label="New file"]').click();
  await page.locator('input[aria-label="FileName Field"]').fill('test.txt');
  await page.locator('button[aria-label="Create"]').click();
  // Wait for notification and click "Go to item" button
  await page.locator('.notification-buttons .button').waitFor({ state: 'visible' });
  await page.locator('.notification-buttons .button').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - test.txt");
  await page.locator('button[aria-label="Close"]').click();
  await expect(page.locator('#listingView .file-items')).toHaveCount(1);
  checkForErrors();
});

