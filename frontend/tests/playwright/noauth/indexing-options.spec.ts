import { test, expect } from "../test-setup";

test("navigate folder -- item should not be visible", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // excluded folder should not be visible in the file list
    await expect(page.locator('a[aria-label="excluded"]')).toHaveCount(0);
    await page.goto("/files/excluded");
    const msg = "500: path not accessible: directory or item excluded from indexing"
    await expect(page.locator('#popup-notification-content')).toHaveText(msg);
    checkForErrors(1,1); // expect error not indexed
});

test("navigate folder -- item should be visible", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // excludedButVisible folder should show up in list
    await expect(page.locator('a[aria-label="excludedButVisible"]')).toHaveCount(1);
    await page.goto("/files/excludedButVisible");
    await expect(page.locator('a[aria-label="shouldshow.txt"]')).toHaveCount(1);
    checkForErrors(); // expect error not indexed
});