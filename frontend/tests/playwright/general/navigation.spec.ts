import { expect, test } from "../test-setup.ts";

test("navigate with hash in file name", async({ page, checkForErrors }) => {
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

test("breadcrumbs navigation checks", async({ page, checkForErrors }) => {
  await page.goto("/files/playwright%20+%20files/myfolder");
  await page.locator('#breadcrumbs').waitFor();
  let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);
  let breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-myfolder"]')
  await expect(breadCrumbLink).toHaveText("myfolder");

  await page.goto("/files/playwright%20+%20files/myfolder/testdata");
  await page.locator('#breadcrumbs').waitFor();
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(2);
  breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-testdata"]')
  await expect(breadCrumbLink).toHaveText("testdata");

  await page.goto("/files/playwright%20+%20files/files");
  await page.locator('#breadcrumbs').waitFor();
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);
  breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-files"]')
  await expect(breadCrumbLink).toHaveText("files");
  checkForErrors();
});

test("navigate from search item", async({ page, checkForErrors }) => {
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

test("use quick jump", async({ page, checkForErrors }) => {
  await page.goto("/files/playwright%20%2B%20files/myfolder/testdata/gray-sample.jpg");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.jpg");

  // drag next button to the left to open quick jump list
  const nextButton = page.locator('button[aria-label="Next"]');
  await nextButton.waitFor({ state: "visible" });
  const box = await nextButton.boundingBox();
  if (!box) throw new Error("Could not get bounding box of Next button");
  const startX = box.x + box.width / 2;
  const startY = box.y + box.height / 2;
  await page.mouse.move(startX, startY);
  await page.mouse.down();
  await page.mouse.move(startX - 200, startY);
  await page.mouse.up();

  const quickJumpWindow = page.locator('div.floating-window[aria-label="file-list-prompt"]');
  await expect(quickJumpWindow).toBeVisible();
  await page.locator('div[aria-label="20130612_142406.jpg"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - 20130612_142406.jpg");
  await expect(page.locator('.topTitle')).toHaveText('20130612_142406.jpg');
  await expect(quickJumpWindow).toBeHidden();
  checkForErrors();
})

const TEST_PATH = "/files/playwright%20%2B%20files/myfolder/testdata";

test("next/previous follow the name order of the folder", async({ page, checkForErrors }) => {
  const next = page.locator('button[aria-label="Next"]');
  const previous = page.locator('button[aria-label="Previous"]');

  await page.goto(`${TEST_PATH}/20130612_142406.jpg`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - 20130612_142406.jpg");
  await next.waitFor({ state: "visible" });
  await expect(previous).toBeHidden();

  await next.click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.jpg");
  await next.click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.webp");

  await previous.click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.jpg");
  await previous.click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - 20130612_142406.jpg");
  await expect(previous).toBeHidden();
  checkForErrors();
});

test("next/previous follow the listing sort order", async({ page, checkForErrors }) => {
  await page.goto(`${TEST_PATH}/`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - testdata");
  const items = page.locator('.listing-items a.listing-item');
  await expect(items.first()).toHaveAttribute("aria-label", "20130612_142406.jpg");

  // reverse name sorting
  await page.locator('.listing-item-header .name').click();
  await expect(items.first()).toHaveAttribute("aria-label", "sample-640x480.mp4");
  const names = await items.evaluateAll((els) => els.map((el) => el.getAttribute("aria-label")));
  expect(names).toEqual([
    "sample-640x480.mp4",
    "IMG_2578.JPG",
    "gray-sample.webp",
    "gray-sample.jpg",
    "20130612_142406.jpg",
  ]);

  // open the second item
  await page.locator('a[aria-label="IMG_2578.JPG"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - IMG_2578.JPG");
  await page.locator('button[aria-label="Next"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.webp");
  await page.locator('button[aria-label="Previous"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - IMG_2578.JPG");
  checkForErrors();
});

test("listing highlights the item we navigated with next/previous after closing", async({ page, checkForErrors }) => {
  await page.goto(`${TEST_PATH}/`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - testdata");
  await page.locator('a[aria-label="gray-sample.jpg"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.jpg");

  await page.locator('button[aria-label="Next"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.webp");

  await page.locator('button[aria-label="Close"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - testdata");
  // the item we ended on, not the one we opened, is selected
  await expect(page.locator('a[aria-label="gray-sample.webp"]')).toHaveAttribute("aria-selected", "true");
  await expect(page.locator('a[aria-label="gray-sample.jpg"]')).not.toHaveAttribute("aria-selected", "true");
  checkForErrors();
});
