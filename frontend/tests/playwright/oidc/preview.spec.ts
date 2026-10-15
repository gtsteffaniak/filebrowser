import { expect, test } from "../test-setup.ts";

test("Create first item", async ({ page, checkForErrors, openContextMenu }, testInfo) => {
  test.setTimeout(20_000);
  const folderName = `first-item-creation-${testInfo.retry}`;

  await page.goto("/");
  await openContextMenu();
  await page.locator('button[aria-label="New folder"]').click();
  await page.locator('input[aria-label="New Folder Name"]').fill(folderName);
  await page.locator('button[aria-label="Create"]').click();
  const folderNotification = page
    .locator('.notification-item')
    .filter({ hasText: 'Folder created successfully' });
  await folderNotification.locator('.notification-buttons .button').click();
  await expect(page).toHaveTitle(`Graham's Filebrowser - Files - ${folderName}`);
  // Dismiss the folder notification
  await folderNotification.locator('i.material-symbols', { hasText: /^\s*close\s*$/ }).click();
  await expect(folderNotification).toBeHidden();
  await expect(page.locator('.listing-items .message > span')).toHaveText('Nothing to show here...');
  await page.locator('.listing-items').click({ button: "right" });
  await page.locator('button[aria-label="New file"]').click();
  await page.locator('button[aria-label="Empty file"]').click();
  await page.locator('input[aria-label="FileName Field"]').fill('test.txt');
  await page.locator('button[aria-label="Create"]').click();
  // Wait for notification and click "Go to item" button
  const fileNotification = page
    .locator('.notification-item')
    .filter({ hasText: 'File created successfully' });
  await fileNotification.locator('.notification-buttons .button').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - test.txt");
  await page.locator('button[aria-label="Close"]').click();
  await expect(page.locator('.listing-items .file-items')).toHaveCount(1);
  checkForErrors();
});
