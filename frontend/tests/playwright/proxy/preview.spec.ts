import { test, expect } from "../test-setup";

test("Create first new file with basic auth", async ({ page, checkForErrors }, testInfo) => {
  // Unique filename per run/retry so retries don't conflict with leftover files
  const fileName = `test-${testInfo.retry + 1}.txt`;

  // Set basic auth credentials for protected /subpath route
  await page.setExtraHTTPHeaders({
    'Authorization': `Basic ZGVtby0xMjcuMC4wLjE6U2VjdXJlUGFzczEyMyE=`
  });

  await page.goto("/subpath/");
  if (testInfo.retry === 0) {
    await expect(page.locator('.listing-items .message > span')).toHaveText('Nothing to show here...');
  }else{
    await expect(await page.locator('.listing-items .file-items')).toHaveCount(testInfo.retry);
  }
  await page.locator('.listing-items').click({ button: "right" });
  await page.locator('button[aria-label="New file"]').click();
  await page.locator('input[aria-label="FileName Field"]').fill(fileName);
  await page.locator('button[aria-label="Create"]').click();
  // Wait for notification and click "Go to item" button
  await page.locator('.notification-buttons .button').waitFor({ state: 'visible' });
  await page.locator('.notification-buttons .button').click();
  await expect(page).toHaveTitle(`Graham's Filebrowser - Files - ${fileName}`);
  await page.locator('button[aria-label="Close"]').click();
  await expect(await page.locator('.listing-items .file-items')).toHaveCount(1 + testInfo.retry);

  // clear basic auth credentials from browser headers for public routes
  await page.setExtraHTTPHeaders({});

  // check share hash
  const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
  if (shareHash === "" || shareHash === null) {
    throw new Error("Share hash not found in localStorage");
  }

  // test public share route (no auth required)
  await page.goto("/subpath/public/share/" + shareHash);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - demo-127.0.0.1");
  await expect(await page.locator('.listing-items .file-items')).toHaveCount(1 + testInfo.retry);
  await page.locator(`a[aria-label="${fileName}"]`).dblclick();
  checkForErrors(0, 1); // expect one redirect error
});

test("Verify basic auth is required for protected route", async ({ page }) => {
  // try to access protected route without credentials - should get 401
  const response = await page.goto("/subpath/", { waitUntil: 'networkidle' });
  expect(response?.status()).toBe(401);

  // verify public routes still work without auth
  const publicResponse = await page.goto("/subpath/public/static/index.html", { waitUntil: 'networkidle' });
  expect(publicResponse?.status()).toBe(200);
});
