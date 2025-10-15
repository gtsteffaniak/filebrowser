
import { test, expect } from "../test-setup";

test.use({viewport: { width: 750, height: 750 }}); // mobile viewport
test("share download multiple files", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/exclude/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));

  await page.goto("/files/share/" + shareHash+"/testdata/");
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
  await expect(page.locator('.selected-count-header')).toHaveText('3');

  await page.locator('button[aria-label="Download"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Download"]').click();
  const popup2 = page.locator('#popup-notification-content');
  await popup2.waitFor({ state: 'visible' });
  await expect(popup2).toHaveText("Downloading...");
  checkForErrors(0,1);
});