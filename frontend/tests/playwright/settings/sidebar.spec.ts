import { test, expect } from "../test-setup";

test("check default sidebar links are added to sidebar", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // sidebar should have two items
    await expect(page.locator('.sidebar-links .inner-card').locator('a')).toHaveCount(3);

    // check items exist
    await page.locator('a[aria-label="playwright + files"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="docker"]').waitFor({ state: 'visible' });
    checkForErrors();
});
