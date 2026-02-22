
import { test, expect } from "../test-setup";

test("navigate with hash in file name", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="folder#hash"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="folder#hash"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - folder#hash");
  await page.locator('a[aria-label="file#.sh"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file#.sh"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - file#.sh");
  await expect(page.locator('.topTitle')).toHaveText('file#.sh');
  checkForErrors()
})

test("breadcrumbs navigation checks", async({ page, checkForErrors, context }) => {
  await page.goto("/files/exclude/myfolder");
  await page.waitForSelector('#breadcrumbs');
  let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);
  let breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-myfolder"]')
  await expect(breadCrumbLink).toHaveText("myfolder");

  await page.goto("/files/exclude/myfolder/testdata");
  await page.waitForSelector('#breadcrumbs');
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(2);
  breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-testdata"]')
  await expect(breadCrumbLink).toHaveText("testdata");

  // TODO: fix this test.. router issue for /files path
  //await page.goto("/files/files");
  //await page.waitForSelector('#breadcrumbs');
  //spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  //expect(spanChildrenCount).toBe(1);
  //breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-files"]')
  //await expect(breadCrumbLink).toHaveText("files");
  checkForErrors();
});

test("navigate from search item", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search-bar-input').click()
  await page.locator('#search-input').fill('for testing');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="for testing.md"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - for testing.md");
  await expect(page.locator('.topTitle')).toHaveText('for testing.md');
  checkForErrors()
});
