import { test, expect } from "../test-setup";

test("navigate folders", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // excluded folder should not be visible in the file list
    await expect(page.locator('a[aria-label="excluded"]')).toHaveCount(0);
    await page.goto("/files/excluded");
    const msg = "500: could not refresh file info: directory or item excluded from indexing"
    await expect(page.locator('#popup-notification-content')).toHaveText(msg);
    checkForErrors(2,1); // expect error not indexed
});