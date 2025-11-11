import { test, expect } from "../test-setup";

test("share folder breadcrumbs navigation checks", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/playwright%20+%20files/share");
    await page.waitForSelector('#breadcrumbs');
    let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
    spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
    expect(spanChildrenCount).toBe(1);
    let breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-share"]')
    await expect(breadCrumbLink).toHaveText("share");

    // click breadcrumb link
    await breadCrumbLink.click()
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - share");
    await page.waitForSelector('#breadcrumbs');
    spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
    expect(spanChildrenCount).toBe(1);
    breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-share"]')
    await expect(breadCrumbLink).toHaveText("share");

    checkForErrors(0,1); // 404 image preview for blank file
});
