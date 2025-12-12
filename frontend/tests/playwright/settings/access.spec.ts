import { test, expect, checkForNotification } from '../test-setup'

test("access rules - deny folder does not show in folder listing", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/access");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  // expect the excluded folder to not be visible
  await expect(page.locator('div[aria-label="excluded"]')).toBeHidden();
  checkForErrors();
});

test("access rules - deny folder has access denied message", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/access");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  const msg = "403: access denied"
  await checkForNotification(page, msg);
  // expect the excluded folder to have an access denied message
  checkForErrors(1,1); // expect 1 api error
});

test("navigate from search item", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search').click()
  await page.locator('#search-sources-dropdown').click()
  await page.locator('#search-sources-dropdown option[value="access"]').click()
  await page.locator('#main-input').fill('showme.txt');
  await expect(page.locator('#result-list')).toHaveCount(0);
  checkForErrors()
});

test("share access controls exist", async ({ page, checkForErrors, context }) => {
  const rootShareHash = await page.evaluate(() => localStorage.getItem('rootShareHash'));
  if (!rootShareHash) {
    throw new Error("Share hash not found in localStorage");
  }
  await page.goto("/files/share/" + rootShareHash );
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - playwright-files");
  // expect the excluded folder to not be visible
  await expect(page.locator('div[aria-label="excluded"]')).toBeHidden();
  checkForErrors(); 
});