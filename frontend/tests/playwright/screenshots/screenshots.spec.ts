import { test, expect } from "../test-setup";
import { Page } from "@playwright/test";

const jpgQuality = 85;

// this file has playwright tests that create screenshots of the UI
test("setup theme", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    await page.goto("/files/");
    // only toggle if active
    const div = page.locator('div[aria-label="Toggle Theme"]')
    if (await div.evaluate(el => el.classList.contains('active'))) {
      await div.click();
    }
  }
});

// run npx playwright test --ui to run these tests locally in ui mode
test("each view mode", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/");
  await page.waitForTimeout(250);
  await page.screenshot({ path: `./generated/listing/view-mode-normal-${theme}.jpg`, quality: jpgQuality });
  await page.locator('button[aria-label="Switch view"]').click();
  await page.waitForTimeout(250);
  await page.screenshot({ path: `./generated/listing/view-mode-gallery-${theme}.jpg`, quality: jpgQuality });
  await page.locator('button[aria-label="Switch view"]').click();
  await page.waitForTimeout(250);
  await page.screenshot({ path: `./generated/listing/view-mode-list-${theme}.jpg`, quality: jpgQuality });
});

// run npx playwright test --ui to run these tests locally in ui mode
test("context menu", async ({ page, checkForErrors, context, theme }) => {
  await page.goto("/files/");
  await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file.tar.gz"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.waitForTimeout(250);
  await page.locator('#context-menu').screenshot({ path: `./generated/context-menu/${theme}.jpg`, quality: jpgQuality });
  if (theme === 'light') {
    return;
  }
  await page.screenshot({ path: `./generated/listing/right-click-${theme}.jpg`, quality: jpgQuality });
});

test("info from search", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/");
  await page.locator('#search-bar-input').click()
  await page.locator('#search-input').fill('file.tar.gz');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/search/from-listing-${theme}.jpg`, quality: jpgQuality });
  await page.locator('li[aria-label="file.tar.gz"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/search/right-click-${theme}.jpg`, quality: jpgQuality });
  await page.locator('button[aria-label="Info"]').click();
  await expect(page.locator('span[aria-label="info display name"]')).toHaveText('file.tar.gz');
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/search/info-from-search-${theme}.jpg`, quality: jpgQuality });
})

test("no viewer available", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/playwright/file.tar.gz");
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - file.tar.gz");
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/viewer/no-viewer-available-${theme}.jpg`, quality: jpgQuality });
})


test("copy from listing to new folder", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/");
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="copyme.txt"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Copy file"]').click();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /');
  await expect(page.locator('li[aria-selected="true"]')).toHaveCount(0);
  await page.locator('.card-content > .listing-items > div[aria-label="myfolder"]').click();
  await expect(page.locator('.card-content > .listing-items > div[aria-selected="true"]')).toHaveCount(1);
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/prompts/copy-to-new-folder-${theme}.jpg`, quality: jpgQuality });
})

test("breadcrumbs navigation checks", async ({ page, checkForErrors, context, theme }) => {
  await page.goto("/files/playwright/myfolder");
  await page.waitForSelector('#breadcrumbs');
  let spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(1);
  let breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-myfolder"]')
  await expect(breadCrumbLink).toHaveText("myfolder");
  await page.goto("/files/playwright/myfolder/testdata");
  await page.waitForSelector('#breadcrumbs');
  spanChildrenCount = await page.locator('#breadcrumbs > ul > li.item').count();
  expect(spanChildrenCount).toBe(2);
  breadCrumbLink = page.locator('a[aria-label="breadcrumb-link-testdata"]')
  await expect(breadCrumbLink).toHaveText("testdata");
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/listing/breadcrumbs-navigation-${theme}.jpg`, quality: jpgQuality });
})


test("delete file", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/");
  await page.locator('a[aria-label="deleteme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="deleteme.txt"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Delete"]').click();
  await expect(page.locator('.card-message')).toHaveText('Are you sure you want to delete this file/folder?');
  await expect(page.locator('.delete-item-wrapper > .listing-item > .text > .name')).toContainText('/deleteme.txt');
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/prompts/delete-deleteme.txt-${theme}.jpg`, quality: jpgQuality });
})

test("text file editor -- text", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/playwright/copyme.txt");
  await page.locator(".ace_content").click();
  await page.keyboard.type("\nYou can edit this file, it shows styles based on formatting.\n\n Works on all text-based files under 25MB limit.");
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/viewer/editor-copyme.txt-${theme}.jpg`, quality: jpgQuality });
});


test("text file editor -- javascript", async ({ page, checkForErrors, context, theme }) => {
  await page.goto("/files/playwright/text-files/javascript.js");
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/viewer/editor-javascript.js-${theme}.jpg`, quality: jpgQuality });
});

test("text file editor -- bash", async ({ page, checkForErrors, context, theme }) => {
  await page.goto("/files/playwright/text-files/bash.sh");
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/viewer/editor-bash.sh-${theme}.jpg`, quality: jpgQuality });
});

test("3d file preview thumbnails", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/");
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - playwright-files");
  await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="myfolder"]').dblclick();
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - myfolder");
  await page.locator('a[aria-label="3dmodels"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="3dmodels"]').dblclick();
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - 3dmodels");  
  await page.waitForTimeout(2000); // wait for thumbnails to load
  await page.screenshot({ path: `./generated/thumbnails/3d-model-${theme}.jpg`, quality: jpgQuality });
  checkForErrors();
});

// 3d file preview, cycle through all 3d files and confirm no errors
test("3d file preview", async ({ page, checkForErrors, context, theme}) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/");
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - playwright-files");
  await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="myfolder"]').dblclick();
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - myfolder");
  await page.locator('a[aria-label="3dmodels"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="3dmodels"]').dblclick();
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - 3dmodels");  
  await page.locator('a[aria-label="Lowpoly_tree_sample.dae"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="Lowpoly_tree_sample.dae"]').dblclick();
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - Lowpoly_tree_sample.dae");
  // check previews work
  await page.locator('.threejs-viewer .loading-overlay').waitFor({ state: 'visible' });
  await page.locator('.threejs-viewer canvas').waitFor({ state: 'visible' })
  await page.waitForTimeout(500);
  await page.screenshot({ path: `./generated/viewer/3d-model-${theme}.jpg`, quality: jpgQuality });
  checkForErrors();
});
