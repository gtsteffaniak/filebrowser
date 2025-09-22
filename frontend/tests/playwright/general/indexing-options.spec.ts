import { test, expect } from "../test-setup";

test("indexing disabled still shows files", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/docker");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - backend");
    // expect some items
    await expect(page.locator('div[aria-label="File Items"]')).toBeVisible();
    await expect(page.locator('div[aria-label="Folder Items"]')).toBeVisible();
    checkForErrors();
});