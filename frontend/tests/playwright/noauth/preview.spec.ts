import { test, expect } from "../test-setup";

test("blob file preview", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file.tar.gz"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - file.tar.gz");
  await page.locator('button[title="Close"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  // Check for console errors
  checkForErrors();
});

test("text file editor", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="copyme.txt"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - copyme.txt");
  const firstLineText = await page.locator('.ace_text-layer .ace_line').first().textContent();
  expect(firstLineText).toBe('test file for playwright');
  await page.locator('button[title="Close"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  // Check for console errors
  checkForErrors();
});

test("navigate folders", async({ page, checkForErrors, context }) => {
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
  // Check for console errors
  checkForErrors();
});
