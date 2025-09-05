import { test, expect } from '../test-setup'

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
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText('Settings updated!');
  const newPrimaryColor = await page.evaluate(() => {
    return getComputedStyle(document.documentElement).getPropertyValue('--primaryColor').trim();
  });
  await expect(newPrimaryColor).toBe('#9b59b6');
  // Check for console errors
  checkForErrors();
});
