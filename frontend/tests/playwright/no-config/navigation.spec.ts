import { expect, test } from "../test-setup.ts";

test("no config shows files", async ({ page, checkForErrors }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - backend");
  // expect some items
  await expect(page.locator('div[aria-label="File Items"]')).toBeVisible();
  await expect(page.locator('div[aria-label="Folder Items"]')).toBeVisible();
  checkForErrors();
});
