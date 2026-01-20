import { test, expect, checkForNotification } from "../test-setup";

test("navigate folder -- item should not be visible", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // excluded folder should not be visible in the file list
    await expect(page.locator('a[aria-label="excluded"]')).toHaveCount(0);
    await page.goto("/files/exclude/excluded");
    const msg = "500: directory or item excluded from indexing"
    await checkForNotification(page, msg);
    checkForErrors(1,1); // expect error not indexed
});

test("navigate folder -- item should be visible", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // excludedButVisible folder should show up in list
    await expect(page.locator('a[aria-label="excludedButVisible"]')).toHaveCount(1);
    await page.goto("/files/exclude/excludedButVisible");
    await expect(page.locator('a[aria-label="shouldshow.txt"]')).toHaveCount(1);
    checkForErrors();
});

test("navigate folder -- item should not be visible because of specific exlcude rule", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // excludedButVisible folder should show up in list
    await expect(page.locator('a[aria-label="excludedButVisible"]')).toHaveCount(1);
    await page.goto("/files/exclude/excludedButVisible");
    await expect(page.locator('a[aria-label="startsWith-hide-me.txt"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="dontshow.txt"]')).toHaveCount(0);
    checkForErrors();
});


test("navigate subfolderExclusions -- item should be visible", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/include");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files2");
    // excludedButVisible folder should show up in list
    await expect(page.locator('a[aria-label="subfolderExclusions"]')).toHaveCount(1);
    await page.goto("/files/include/subfolderExclusions");
    await expect(page.locator('a[aria-label="shouldshow"]')).toHaveCount(1);
    checkForErrors();
});

test("navigate subfolderExclusions -- subfolder items rules should be applied", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/include/subfolderExclusions");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - subfolderExclusions");

    // should show
    await expect(page.locator('a[aria-label="shouldshow"]')).toHaveCount(1);
    await expect(page.locator('a[aria-label="folderShouldShow"]')).toHaveCount(1);
    await expect(page.locator('a[aria-label="folderItem"]')).toHaveCount(1);

    // should not show
    await expect(page.locator('a[aria-label="endsWithFolder"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="startsWithFolder"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="startsWithTest.txt"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="exclusionlist.sh"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label=".hiddenDir"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label=".hidden"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="fileName"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="folderName"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="fileNames"]')).toHaveCount(0);
    await expect(page.locator('a[aria-label="folderNames"]')).toHaveCount(0);

    checkForErrors();
});

test("navigate subfolderExclusions -- nested subfolder items rules should inherit from parent", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/include/subfolderExclusions/.hiddenDir/nested.txt");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files");

    await expect(page.locator('.error-message .message > span')).toHaveText('Something really went wrong.');

    checkForErrors(1,1); // expect error not indexed
});

test("root indexing info is correct", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // should mostly match system (du -sh frontend/tests/playwright-files) besides excluded values
    await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
    
    // Check folder sizes
    await expect(page.locator('a[aria-label="myfolder"]').locator('.size')).toHaveText("3.0 MB");
    await expect(page.locator('a[aria-label="folder#hash"]').locator('.size')).toHaveText("4.0 KB");
    await expect(page.locator('a[aria-label="files"]').locator('.size')).toHaveText("8.0 KB");
    await expect(page.locator('a[aria-label="share"]').locator('.size')).toHaveText("4.0 KB");
    await expect(page.locator('a[aria-label="text-files"]').locator('.size')).toHaveText("8.0 KB");
    await expect(page.locator('a[aria-label="subfolderExclusions"]').locator('.size')).toHaveText("16.0 KB"); // 16 not 24 due to excluded items
    await expect(page.locator('a[aria-label="excludedButVisible"]').locator('.size')).toHaveText("4.0 KB");
    
    // Check file sizes
    await expect(page.locator('a[aria-label="file.tar.gz"]').locator('.size')).toHaveText("4.0 KB");
    await expect(page.locator('a[aria-label="copyme.txt"]').locator('.size')).toHaveText("4.0 KB");
    await expect(page.locator('a[aria-label="utf8-truncated.txt"]').locator('.size')).toHaveText("12.0 KB");
    
    // Check zero-size files
    await expect(page.locator('a[aria-label="1file1.txt"]').locator('.size')).toHaveText("0.0 bytes");

    checkForErrors();
});

test("root indexing info is correct (logical size)", async ({ page, checkForErrors, context }) => {
    await page.goto("/files/include");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files2");
    // should mostly match system (du -sh frontend/tests/playwright-files) besides excluded values
    await page.locator('a[aria-label="myfolder"]').waitFor({ state: 'visible' });
    
    // Check folder sizes
    await expect(page.locator('a[aria-label="folder#hash"]').locator('.size')).toHaveText("0.0 bytes");
    await expect(page.locator('a[aria-label="files"]').locator('.size')).toHaveText("17.0 bytes");
    await expect(page.locator('a[aria-label="subfolderExclusions"]').locator('.size')).toHaveText("0.0 bytes");
    
    await page.goto("/files/include/files");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - files");

    // Check folder sizes
    await expect(page.locator('a[aria-label="nested"]').locator('.size')).toHaveText("1.0 bytes");
    // Check file sizes
    await expect(page.locator('a[aria-label="for testing.md"]').locator('.size')).toHaveText("16.0 bytes");

    checkForErrors();
});