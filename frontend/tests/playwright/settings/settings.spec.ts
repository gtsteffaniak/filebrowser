import { test, expect, checkForNotification } from '../test-setup'

test("adjusting theme colors", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  const originalPrimaryColor = await page.evaluate(() => {
    return getComputedStyle(document.documentElement).getPropertyValue('--primaryColor').trim();
  });
  await expect(originalPrimaryColor).toBe('#2196f3');

  // Verify the page title
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('i[aria-label="settings"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator('div[aria-label="themeLanguage"]').click();
  await page.locator('button', { hasText: 'violet' }).click();
  await checkForNotification(page, 'Settings updated!');
  const newPrimaryColor = await page.evaluate(() => {
    return getComputedStyle(document.documentElement).getPropertyValue('--primaryColor').trim();
  });
  await expect(newPrimaryColor).toBe('#9b59b6');
  // Check for console errors
  checkForErrors();
});

test("choose custom theme", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('i[aria-label="settings"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator('div[aria-label="themeLanguage"]').click();
  // a custom no-rounded.css theme file added to docker that should exist and be selectable
  await page.locator('select[aria-label="themeOptions"]').selectOption('no-rounded');
  await checkForNotification(page, 'Settings updated!');
  // Check for console errors
  checkForErrors();
});

test("view config", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('i[aria-label="settings"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator('#systemAdmin-sidebar').click();
  await page.locator('button[aria-label="loadConfig"]').click();
  await expect(page.locator('.ace_text-layer .ace_line').first()).toContainText('server:');
  // Check for console errors
  checkForErrors();
});
