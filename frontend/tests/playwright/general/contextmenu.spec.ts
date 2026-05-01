import { test, expect } from "../test-setup";


test("info from listing - archive item", async({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="file.tar.gz"]').click( { button: "right" });
    await page.locator('.selected-count-header').waitFor({ state: 'visible' });
    await expect(page.locator('.selected-count-header')).toHaveText('1');
    await expect(page.locator('button[aria-label="Info"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Download"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Share"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Delete"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Rename"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Move file"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Copy file"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Select all"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Create archive"]')).toBeHidden();
    checkForErrors();
});

test("info from listing - regular item", async({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    await page.locator('a[aria-label="1.1MB.bin"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="1.1MB.bin"]').click( { button: "right" });
    await page.locator('.selected-count-header').waitFor({ state: 'visible' });
    await expect(page.locator('.selected-count-header')).toHaveText('1');
    await expect(page.locator('button[aria-label="Info"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Download"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Share"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Delete"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Rename"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Move file"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Copy file"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Select all"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Create archive"]')).toBeHidden();
    checkForErrors();
});

test("context menu is shown on sizeAnalyzer tool", async({ page, checkForErrors, context }) => {
    await page.goto("/tools/sizeViewer");
    await expect(page).toHaveTitle("Graham's Filebrowser - Tools");
    await page.locator('input[aria-label="Larger than size input"]').fill('1');
    await page.locator('button[aria-label="Analyze button"]').click();
    await page.locator('div[aria-label="1.1MB.bin"]').hover();
    await page.waitForTimeout(1500);
    await expect(page.locator('.floating-tooltip')).toBeVisible();
    await expect(page.locator('.floating-tooltip')).toHaveText('1.1MB.bin (1.1 MB)');
    await page.locator('div[aria-label="1.1MB.bin"]').click({ button: "right" });
    await expect(page.locator('.selected-count-header')).toBeHidden();
    await expect(page.locator('button[aria-label="Info"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Open parent folder"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Go to item"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Download"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Share"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Delete"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Rename"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Move file"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Copy file"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Select all"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Create archive"]')).toBeHidden();
    checkForErrors();
});

test("context menu is shown on duplicateFinder tool", async({ page, checkForErrors, context }) => {
    await page.goto("/tools/duplicateFinder");
    await expect(page).toHaveTitle("Graham's Filebrowser - Tools");
    await page.locator('input[aria-label="Minimum size input"]').fill('1');
    await page.locator('button[aria-label="Find duplicates button"]').click();

    // expect 1 group with two items
    await expect(page.locator('div[aria-label="1.1MB.bin"]')).toBeVisible();
    await expect(page.locator('div[aria-label="1.1mb.bin"]')).toBeVisible();
    // right click on 1.1MB.bin
    await page.locator('div[aria-label="1.1MB.bin"]').click({ button: "right" });
    await expect(page.locator('button[aria-label="Info"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Open parent folder"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Go to item"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Download"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Share"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Delete"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Rename"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Move file"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Copy file"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Select all"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Create archive"]')).toBeHidden();
    checkForErrors();
});

test("context menu is shown on quick jump", async({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    await page.locator('a[aria-label="1.1MB.bin"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="1.1MB.bin"]').dblclick();

    await expect(page).toHaveTitle("Graham's Filebrowser - Files - 1.1MB.bin");

    // drag next button to the left to open quick jump list
    const nextButton = page.locator('button[aria-label="Next"]');
    await nextButton.waitFor({ state: "visible" });
    const box = await nextButton.boundingBox();
    expect(box).toBeTruthy();
    const startX = box!.x + box!.width / 2;
    const startY = box!.y + box!.height / 2;
    await page.mouse.move(startX, startY);
    await page.mouse.down();
    await page.mouse.move(startX - 200, startY);
    await page.mouse.up();

    const quickJumpWindow = page.locator('div.floating-window[aria-label="file-list-prompt"]');
    await expect(quickJumpWindow).toBeVisible();
    // expect prompt to be visible
    await expect(page.locator('div.floating-window[aria-label="file-list-prompt"]')).toBeVisible();
    await page.locator('div[aria-label="1.1mb.bin"]').click({ button: "right" });
    await expect(page.locator('button[aria-label="Info"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Open parent folder"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Go to item"]')).toBeVisible();
    await expect(page.locator('button[aria-label="Download"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Share"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Delete"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Rename"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Move file"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Copy file"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Select all"]')).toBeHidden();
    await expect(page.locator('button[aria-label="Create archive"]')).toBeHidden();
    checkForErrors();
});
