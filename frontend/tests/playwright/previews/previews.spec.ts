import { test, expect } from "../test-setup";

// 3d file preview
test("3d file preview thumbnails", async({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="myfolder"]').dblclick();
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - myfolder");
    await page.locator('a[aria-label="3dmodels"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="3dmodels"]').dblclick();
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - 3dmodels");
    // check previews work
    await page.locator('a[aria-label="Lowpoly_tree_sample.dae"] .threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="Lowpoly_tree_sample.dae"] .threejs-viewer canvas').waitFor({ state: 'visible' });
    // Check for console errors
    checkForErrors();
});
  
// 3d file preview
test("3d file preview next/previous", async({ page, checkForErrors, context }) => {
    await page.goto("/files/playwright-files/myfolder/3dmodels/Lowpoly_tree_sample.dae");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - Lowpoly_tree_sample.dae");
    // check previews work
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });

    checkForErrors();
});
