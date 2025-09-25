import { test, expect } from "../test-setup";

test("no config shows files", async ({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - backend");
  // expect some items
  await expect(page.locator('div[aria-label="File Items"]')).toBeVisible();
  await expect(page.locator('div[aria-label="Folder Items"]')).toBeVisible();
  checkForErrors();
});