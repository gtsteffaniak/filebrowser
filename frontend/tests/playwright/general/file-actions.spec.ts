import { test, expect, checkForNotification } from "../test-setup";

test("info from listing", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file.tar.gz"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Info"]').click();
  await expect(page.locator('span[aria-label="info display name"]')).toHaveText('file.tar.gz');
  checkForErrors();
});

test("info from search", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search').click()
  await page.locator('#main-input').fill('file.tar.gz');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="file.tar.gz"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Info"]').click();
  await expect(page.locator('span[aria-label="info display name"]')).toHaveText('file.tar.gz');
  checkForErrors();
})

test("open from search", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search').click()
  await page.locator('#main-input').fill('file.tar.gz');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="file.tar.gz"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - file.tar.gz");
  await expect(page.locator('#previewer')).toContainText('Preview is not available for this file.');
  checkForErrors();
})

test("open nested file in /files dir from search", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search').click()
  await page.locator('#main-input').fill('binary');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="binary.dat"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - binary.dat");
  await expect(page.locator('#previewer')).toContainText('Preview is not available for this file.');
  checkForErrors();
})

test("open markdown file from search", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('#search').click()
  await page.locator('#main-input').fill('for testing');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="for testing.md"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - for testing.md");
  await expect(page.locator('#markedown-viewer')).toContainText('this is a test');
  checkForErrors();
})

test("2x copy from listing to new folder", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="copyme.txt"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Copy file"]').click();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /');
  await expect(page.locator('div[aria-label="copy-prompt"] .listing-item[aria-selected="true"]')).toHaveCount(0);
  await page.locator('div[aria-label="copy-prompt"] .listing-item[aria-label="myfolder"]').click();
  await expect(page.locator('div[aria-label="copy-prompt"] .listing-item[aria-selected="true"]')).toHaveCount(1);
  await page.locator('button[aria-label="Copy"]').click();
  await checkForNotification(page, "Files copied successfully!");
  await page.goto("/files/playwright%20%2B%20files/myfolder/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - myfolder");
  // verify exists and copy again
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });

  // create new directory
  // Ensure .listing-items is visible
  await page.locator('.listing-items').click({ button: "right" });
  await page.locator('button[aria-label="New folder"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="New folder"]').click();
  await page.locator('input[aria-label="New Folder Name"]').waitFor({ state: 'visible' });
  await page.locator('input[aria-label="New Folder Name"]').fill('newfolder');
  await page.locator('button[aria-label="Create"]').click();
  // Wait for notification and click "Go to item" button
  await page.locator('.notification-buttons .button').waitFor({ state: 'visible' });
  await page.locator('.notification-buttons .button').click();
  await expect(page).toHaveTitle(/.* - newfolder/);
  await page.goBack();
  await expect(page).toHaveTitle(/.* - myfolder/);

  await page.locator('a[aria-label="copyme.txt"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Copy file"]').click();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /myfolder/');
  await page.locator('div[aria-label="copy-prompt"] .listing-item[aria-label="newfolder"]').click();
  await page.locator('button[aria-label="Copy"]').click();
  await checkForNotification(page, "Files copied successfully!");
  await page.goto("/files/playwright%20%2B%20files/myfolder/newfolder/");
  await expect(page).toHaveTitle(/.* - newfolder/);
  checkForErrors();
})

test("copy 'text-files' to 'folder#hash' verify folder size is updated", async({ page, checkForErrors, openContextMenu, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  
  // Find folder#hash and get its size before copy
  await page.locator('a[aria-label="folder#hash"]').waitFor({ state: 'visible' });
  const folderHashLink = page.locator('a[aria-label="folder#hash"]');
  const textFilesLink = page.locator('a[aria-label="text-files"]');
  const textFilesSizeBefore = await textFilesLink.locator('.size').textContent();
  const folderHashSizeBefore = await folderHashLink.locator('.size').textContent();
  
  // Copy myfotext-filesder
  await textFilesLink.click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Copy file"]').click();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /');
  await page.locator('div[aria-label="copy-prompt"] .listing-item[aria-label="folder#hash"]').click();
  await page.locator('button[aria-label="Copy"]').click();
  await checkForNotification(page, "Files copied successfully!");
  await page.locator('.notification-buttons .button').waitFor({ state: 'visible' });
  await page.locator('.notification-buttons .button').click();

  // verify folder size is updated
  const folderSizeAfter = await textFilesLink.locator('.size').textContent();
  expect(folderSizeAfter).not.toBe("0.0 bytes");
  expect(folderSizeAfter).toBe(textFilesSizeBefore);

  // Go back to root and verify the copied folder has a non-zero size
  await page.goto("/files/");
  await page.locator('a[aria-label="folder#hash"]').waitFor({ state: 'visible' });
  const copiedFolderSize = await page.locator('a[aria-label="folder#hash"]').locator('.size').textContent();
  expect(copiedFolderSize).not.toBe("0.0 bytes");
  expect(copiedFolderSize).not.toBe(folderHashSizeBefore);
  checkForErrors();
})

test("delete file", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="deleteme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="deleteme.txt"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Delete"]').click();
  await expect( page.locator('.card-content')).toContainText('/deleteme.txt');
  await page.locator('button[aria-label="Confirm-Delete"]').click();
  await checkForNotification(page, "Deleted successfully!");
  checkForErrors();
})

test("delete nested file prompt", async({ page, checkForErrors, context }) => {
  await page.goto("/files/playwright%20%2B%20files/folder%23hash/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - folder#hash");
  await page.locator('a[aria-label="file#.sh"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file#.sh"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Delete"]').click();
  await expect(page.locator('.card-content')).toContainText('/folder#hash/file#.sh');
  checkForErrors();
})
