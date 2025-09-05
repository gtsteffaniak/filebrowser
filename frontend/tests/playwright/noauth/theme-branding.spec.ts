import { test, expect } from "../test-setup";

test("sidebar links", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");

  // Verify the page title
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  // Locate the credits container
  const credits = page.locator('.credits'); // Fix the selector to match the HTML structure

  // Assert that the <h3> contains the text 'FileBrowser Quantum'
  await expect(credits.locator("h4")).toHaveText("Graham's Filebrowser");

  // Assert that the <a> contains the text 'A playwright test'
  await expect(credits.locator("span").locator("a")).toHaveText('A playwright test');

  // Assert that the <a> does not contain the text 'Help'
  await expect(credits.locator("span").locator("a")).not.toHaveText('Help');
  // Check for console errors
  checkForErrors();
});

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
  await page.locator('a[aria-label="themeLanguage"]').click();
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
