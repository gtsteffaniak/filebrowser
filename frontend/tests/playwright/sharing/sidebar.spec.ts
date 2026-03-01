import { test, expect } from "../test-setup";

test("create a new share", async ({ page, checkForErrors, context }) => {
    // create a new share
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // share the text-files folder
    await page.locator('a[aria-label="text-files"]').click();
    await page.locator('a[aria-label="text-files"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="text-files"]').click({ button: "right" });
    await page.locator('.selected-count-header').waitFor({ state: 'visible' });
    await expect(page.locator('.selected-count-header')).toHaveText('1');
    await page.locator('button[aria-label="Share"]').click();
    await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /text-files/');
    await page.locator('button[aria-label="Share-Confirm"]').click();
    await expect(page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th))")).toHaveCount(1);
});

test("check previously created share has correct sidebar links", async ({ page, checkForErrors, context }) => {
    // create a new share
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    await page.locator('a[aria-label="text-files"]').click();
    await page.locator('a[aria-label="text-files"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="text-files"]').click({ button: "right" });
    await page.locator('.selected-count-header').waitFor({ state: 'visible' });
    await expect(page.locator('.selected-count-header')).toHaveText('1');
    await page.locator('button[aria-label="Share"]').click();
    // create a new share
    const shareHash = await page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th)) td").first().textContent();
    if (!shareHash) {
        throw new Error("Failed to retrieve shareHash");
    }
    // navigate to the share sidebar
    await page.goto("/public/share/" + shareHash);
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - text-files");
    // sidebar should have three items (ShareInfo, Download, Edit Share)
    await expect(page.locator('.sidebar-links .inner-card').locator('a')).toHaveCount(3);
    // check items exist
    await page.locator('a[aria-label="Share QR Code and Info"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="Download"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="Edit Share"]').waitFor({ state: 'visible' });
    checkForErrors();
});

test("edit previously created links and ensure they are updated", async ({ page, checkForErrors, context }) => {
    // create a new share
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // share the text-files folder
    await page.locator('a[aria-label="text-files"]').click();
    await page.locator('a[aria-label="text-files"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="text-files"]').click({ button: "right" });
    await page.locator('.selected-count-header').waitFor({ state: 'visible' });
    await expect(page.locator('.selected-count-header')).toHaveText('1');
    await page.locator('button[aria-label="Share"]').click();
    await expect(page.locator('div[aria-label="share-path"]')).toHaveText('Path: /text-files/');
    // edit the link
    await page.locator('button[aria-label="Edit"]').click();
    await page.locator('.customize-sidebar-links-button').click();
    // add a new custom link
    await page.locator('.add-link-button').click()
    // select the custom link from the dropdown
    await page.locator('.add-link-form select[aria-label="Link Type"]').click();
    await page.locator('.add-link-form select[aria-label="Link Type"]').selectOption('custom');
    await page.locator('.add-link-form input[aria-label="Link Name"]').fill('New Custom Link');
    await page.locator('.add-link-form input[aria-label="Link Target"]').fill('https://www.google.com');
    await page.locator('button[aria-label="Add Link"]').click();
    await page.locator('button[aria-label="Save Links"]').click();
    await page.locator('button[aria-label="Share-Confirm"]').click();
    // create a new share
    const shareHash = await page.locator("div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th)) td").first().textContent();
    if (!shareHash) {
        throw new Error("Failed to retrieve shareHash");
    }
    // navigate to the share sidebar
    await page.goto("/public/share/" + shareHash);
    await expect(page).toHaveTitle("Graham's Filebrowser - Share - text-files");
    // sidebar should have 5 items (ShareInfo, Download, New Custom Link, Edit Share, Go to source location)
    await expect(page.locator('.sidebar-links .inner-card').locator('a')).toHaveCount(5);
    // check items exist
    await page.locator('a[aria-label="Share QR Code and Info"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="Download"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="New Custom Link"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="Edit Share"]').waitFor({ state: 'visible' });
    checkForErrors();
});
