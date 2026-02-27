import { test, expect } from "../test-setup";

// 3d file thumbnails work
test("3d file preview thumbnails in share", async({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
    if (!shareHash || shareHash == "") {
        throw new Error("Share hash not found in localStorage");
    }
    console.log("shareHash: " + shareHash);
    await page.goto("/share/" + shareHash);
    // log current url
    console.log("current url: " + page.url());
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - myfolder");
    await page.locator('a[aria-label="3dmodels"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="3dmodels"]').dblclick();
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - 3dmodels");
    // check previews work
    await page.locator('a[aria-label="Lowpoly_tree_sample.dae"] .threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="Lowpoly_tree_sample.dae"] .threejs-viewer canvas').waitFor({ state: 'visible' });

    // wait 2 seconds
    await page.waitForTimeout(2000);
    // Check for console errors
    checkForErrors(0,1); // redirect errors are expected
});

// 3d file preview, cycle through all 3d files and confirm no errors
test("3d file preview next/previous", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
    if (shareHash == "") {
        throw new Error("Share hash not found in localStorage");
    }
    
    // Go directly to a 3D model file in the share
    await page.goto("/share/" + shareHash + "/3dmodels/Lowpoly_tree_sample.dae");
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - Lowpoly_tree_sample.dae");
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    await page.locator('button[aria-label="Next"]').click();

    // material file
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - Lowpoly_tree_sample.mtl");
    await page.locator('button[aria-label="Next"]').click();

    await expect(page).toHaveTitle("Graham's Filebrowser - Share - Lowpoly_tree_sample.obj");
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    await page.locator('button[aria-label="Next"]').click();
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - mario_cube_WHOLE_top_foxed.stl");
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    await page.locator('button[aria-label="Next"]').click();
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - Rigged Hand.3ds");
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    checkForErrors(0,1); // redirect errors are expected
});
