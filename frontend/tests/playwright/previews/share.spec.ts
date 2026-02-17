import { test, expect } from "../test-setup";

test("share file works", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
    if (shareHash == "") {
        throw new Error("Share hash not found in localStorage");
    }

    await page.goto("/share/" + shareHash);
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - myfolder");
    checkForErrors(0,1); // redirect errors are expected
});

// 3d file preview, cycle through all 3d files and confirm no errors
test("3d file preview next/previous", async ({ page, checkForErrors, context }) => {
    const shareHash = await page.evaluate(() => localStorage.getItem('shareHash'));
    if (shareHash == "") {
        throw new Error("Share hash not found in localStorage");
    }

    await page.goto("/share/" + shareHash);
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - myfolder");
    // click first item in folder
    await page.locator('a[aria-label="Lowpoly_tree_sample.dae"]').click();
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - Lowpoly_tree_sample.dae");

    // check previews work
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    await page.locator('button[aria-label="Next"]').click();
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - Lowpoly_tree_sample.obj");
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    await page.locator('button[aria-label="Next"]').click();
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - mario_cube_WHOLE_top_foxed.stl");
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    await page.locator('button[aria-label="Next"]').click();
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - Rigged Hand.3ds");
    await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
    await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' });
    checkForErrors(2,2); // lets fix this later
});
