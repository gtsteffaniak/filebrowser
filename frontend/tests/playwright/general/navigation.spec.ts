
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
  await page.goto("/files/playwright%20+%20files/myfolder");
  await page.waitForSelector('#breadcrumbs');
  let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);
  let breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-myfolder"]')
  await expect(breadCrumbLink).toHaveText("myfolder");

  await page.goto("/files/playwright%20+%20files/myfolder/testdata");
  await page.waitForSelector('#breadcrumbs');
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(2);
  breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-testdata"]')
  await expect(breadCrumbLink).toHaveText("testdata");

  await page.goto("/files/playwright%20+%20files/files");
  await page.waitForSelector('#breadcrumbs');
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);
  breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-files"]')
  await expect(breadCrumbLink).toHaveText("files");
  checkForErrors();
});

test("navigate from search item", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search').click()
  await page.locator('#main-input').fill('for testing');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="for testing.md"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - for testing.md");
  await expect(page.locator('.topTitle')).toHaveText('for testing.md');
  checkForErrors()
});

test("use quick jump", async({ page, checkForErrors, context }) => {
  await page.goto("/files/playwright%20%2B%20files/myfolder/testdata/gray-sample.jpg");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.jpg");

  // drag next button to the left to open quick jump list
  const nextButton = page.locator('button[aria-label="Next"]');
  await nextButton.waitFor({ state: "visible" });
  const box = await nextButton.boundingBox();
  expect(box).toBeTruthy();
  const startX = box!.x + box!.width / 2;
  const startY = box!.y + box!.height / 2;
  await page.mouse.move(startX, startY);
  await page.mouse.down();
  await page.mouse.move(startX - 200, startY);
  await page.mouse.up();

  await expect(page.locator('div[aria-label="file-list-prompt"]')).toBeVisible();
  await page.locator('div[aria-label="20130612_142406.jpg"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - 20130612_142406.jpg");
  await expect(page.locator('.topTitle')).toHaveText('20130612_142406.jpg');
  await expect(page.locator('div[aria-label="file-list-prompt"]')).toBeHidden();
  checkForErrors();
})