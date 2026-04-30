import { test, expect, checkForNotification } from '../test-setup'

test("access rules - deny folder does not show in folder listing", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/access");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  // expect the excluded folder to not be visible
  await expect(page.locator('a[aria-label="excluded"]')).toBeHidden();
  checkForErrors();
});

test("access rules - main denyByDefault root exception works", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/access");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await expect(page.locator('a[aria-label="text-files"]')).toBeVisible();
  checkForErrors();
});

test("access rules - deny folder has access denied message", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/access/excluded");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files");
  const msg = "403: access denied"
  await checkForNotification(page, msg);
  // expect the excluded folder to have an access denied message
  checkForErrors(1,1);
});

test("navigate from search item", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search-bar-input').click()
  await page.locator('select[aria-label="search sources dropdown"]').selectOption('access');
  await page.locator('#search-input').fill('showme.txt');
  await expect(page.locator('.searchPrompt p')).toHaveText('No results found.');
  checkForErrors()
});

test("share access controls exist", async ({ page, checkForErrors, context }) => {
  // localStorage is not available on about:blank; Firefox throws "The operation is insecure."
  await page.goto("/files/");
  // Leaving /files/ while fetches are still in flight aborts them; Firefox logs NetworkError to the console.
  await page.waitForLoadState("networkidle");
  const rootShareHash = await page.evaluate(() => localStorage.getItem("rootShareHash"));
  if (!rootShareHash) {
    throw new Error("Share hash not found in localStorage");
  }
  await page.goto("/public/share/" + rootShareHash);
  // Document title stays at router default until Files.vue loads share metadata (see router beforeResolve vs Files.vue).
  await expect(page.locator('a[aria-label="excludedButVisible"]')).toBeVisible();
  await expect(page.locator('div[aria-label="excluded"]')).toBeHidden();
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - playwright-files");

  checkForErrors();
});