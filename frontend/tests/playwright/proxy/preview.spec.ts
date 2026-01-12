import { test, expect } from "../test-setup";

test("Create first new file with basic auth", async ({  page, checkForErrors, context }) => {

  // Set basic auth credentials for protected /subpath route
  await page.setExtraHTTPHeaders({
    'Authorization': `Basic ZGVtby0xMjcuMC4wLjE6U2VjdXJlUGFzczEyMyE=`
  });

  await page.goto("/subpath/");
  await expect(page.locator('#listingView .message > span')).toHaveText('Nothing to show here...');
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

  // clear basic auth credentials from browser headers for public routes
  await page.setExtraHTTPHeaders({});

  // check share hash
  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (shareHash == "") {
    throw new Error("Share hash not found in localStorage");
  }

  // test public share route (no auth required)
  await page.goto("/subpath/public/share/" + shareHash);
  await expect(page.locator('#listingView .file-items')).toHaveCount(1);
  await page.locator('a[aria-label="test.txt"]').dblclick();
  checkForErrors(0,1);
});

test("Verify basic auth is required for protected route", async ({ page }) => {
  // try to access protected route without credentials - should get 401
  const response = await page.goto("/subpath/", { waitUntil: 'networkidle' });
  expect(response?.status()).toBe(401);

  // verify public routes still work without auth
  const publicResponse = await page.goto("/subpath/public/static/index.html", { waitUntil: 'networkidle' });
  expect(publicResponse?.status()).toBe(200);
});
