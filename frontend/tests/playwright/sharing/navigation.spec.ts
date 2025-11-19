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

    checkForErrors();
});

test("breadcrumbs navigation checks", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

    const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
    if (shareHash == "") {
      throw new Error("Share hash not found in localStorage");
    }

    await page.goto("/share/" + shareHash);
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - myfolder");
    await page.dblclick('a[aria-label="testdata"]');
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
    await page.waitForSelector('#breadcrumbs');
    // Ensure no <span> children exist directly under #breadcrumbs (ie no breadcrumbs paths)
    let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
    expect(spanChildrenCount).toBe(1);

    checkForErrors(0,2); // redirect errors are expected and 404 for blank preview
  });
