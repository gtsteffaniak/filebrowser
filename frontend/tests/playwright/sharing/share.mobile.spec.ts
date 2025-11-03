import { test, expect, checkForNotification } from "../test-setup";

test.use({viewport: { width: 750, height: 750 }}); // mobile viewport
test("share download multiple files", async ({ page, checkForErrors, context }) => {

  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (shareHash == "") {
    throw new Error("Share hash not found in localStorage");
  }

  // Test explicit redirect behavior
  const responsePromise = page.waitForResponse(response =>
    response.url().includes("/share/" + shareHash + "/testdata/") &&
    response.status() === 301
  );

  await page.goto("/share/" + shareHash + "/testdata/");

  // Wait for and verify the redirect response
  const response = await responsePromise;
  expect(response.status()).toBe(301); // Moved Permanently

  // Verify final URL and title after redirect
  await expect(page).toHaveURL(new RegExp(`/public/share/${shareHash}/testdata/`));
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await page.locator('a[aria-label="gray-sample.jpg"]').click({ button: "right" });
  await page.locator('button[aria-label="Select multiple"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Select multiple"]').click();

  await checkForNotification(page, "Multiple Selection Enabled");

  await page.locator('a[aria-label="20130612_142406.jpg"]').click();
  await page.locator('a[aria-label="IMG_2578.JPG"]').click();
  await page.locator('a[aria-label="gray-sample.jpg"]').click({ button: "right" });

  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('3');

  await page.locator('button[aria-label="Download"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Download"]').click();
  await checkForNotification(page, "Downloading...");
  checkForErrors(0,1); // redirect errors are expected
});