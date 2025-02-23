import { test, expect } from "@playwright/test";

test("blob file preview", async ({  page, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file.tar.gz"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - file.tar.gz");
  await page.locator('button[title="Close"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files  - playwright-files");
});

test("text file editor", async ({ page, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="copyme.txt"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - copyme.txt");
  const firstLineText = await page.locator('.ace_text-layer .ace_line').first().textContent();
  expect(firstLineText).toBe('test file for playwright');
  await page.locator('button[title="Close"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
});

test("navigate folders", async ({  page, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="myfolder"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - myfolder");
  await page.locator('a[aria-label="testdata"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="testdata"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - testdata");
  await page.locator('a[aria-label="gray-sample.jpg"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="gray-sample.jpg"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.jpg");
});

test("navigating images", async ({  page, context }) => {
  await page.goto("/files/myfolder/testdata/20130612_142406.jpg");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - 20130612_142406.jpg");
  await page.locator('button[aria-label="Previous"]').waitFor({ state: 'hidden' });
  await page.mouse.move(100, 100);
  await page.locator('button[aria-label="Next"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="Next"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - gray-sample.jpg");
  await page.locator('button[aria-label="Previous"]').waitFor({ state: 'hidden' });
  await page.locator('button[aria-label="Next"]').waitFor({ state: 'hidden' });
  await page.mouse.move(100, 100);
  await page.locator('button[aria-label="Next"]').waitFor({ state: 'visible' });
  //await page.locator('button[aria-label="Next"]').click();
  // went to next image
  //await expect(page).toHaveTitle("Graham's Filebrowser - Files - IMG_2578.JPG");
});