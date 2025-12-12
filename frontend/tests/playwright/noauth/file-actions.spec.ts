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
  await expect(page.locator('#result-list ul li.search-entry')).toHaveCount(1);
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
  await expect(page.locator('#result-list ul li.search-entry')).toHaveCount(1);
  await page.locator('li[aria-label="file.tar.gz"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - file.tar.gz");
  await expect(page.locator('#previewer')).toContainText('Preview is not available for this file.');
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
  await expect(page.locator('li[aria-selected="true"]')).toHaveCount(0);
  await page.locator('li[aria-label="myfolder"]').dblclick();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /myfolder/');
  await page.locator('button[aria-label="Copy"]').click();
  await checkForNotification(page, "Files copied successfully!");
  await page.goto("/files/files/exclude/myfolder/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - myfolder");
  // verify exists and copy again
  await page.locator('a[aria-label="copyme.txt"]').waitFor({ state: 'visible' });

  // create new directory
  // Ensure #listingView is visible
  await page.locator('#listingView').click({ button: "right" });
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
  await page.locator('li[aria-label="newfolder"]').dblclick();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /myfolder/newfolder/');
  await page.locator('button[aria-label="Copy"]').click();
  await checkForNotification(page, "Files copied successfully!");
  await page.goto("/files/files/exclude/myfolder/newfolder/");
  await expect(page).toHaveTitle(/.* - newfolder/);
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
  await expect( page.locator('.card-content')).toHaveText('Are you sure you want to delete this file/folder?/deleteme.txt');
  await expect(page.locator('div[aria-label="delete-path"]')).toHaveText('/deleteme.txt');
  await page.locator('button[aria-label="Confirm-Delete"]').click();
  await checkForNotification(page, "Deleted successfully!");

  // verify its no longer in index via search
  await page.locator('#search').click()
  await page.locator('#main-input').fill('deleteme.txt');
  await expect(page.locator('#result-list ul li.search-entry')).toHaveCount(0);
  checkForErrors();
})

test("delete nested file prompt", async({ page, checkForErrors, context }) => {
  await page.goto("/files/files/exclude/folder%23hash/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - folder#hash");
  await page.locator('a[aria-label="file#.sh"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file#.sh"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Delete"]').click();
  await expect(page.locator('.card-content')).toHaveText('Are you sure you want to delete this file/folder?/folder#hash/file#.sh');
  await expect(page.locator('div[aria-label="delete-path"]')).toHaveText('/folder#hash/file#.sh');
  checkForErrors();
})

test("rename file", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="renameme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="renameme.txt"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Rename"]').click();
  await page.locator('input[aria-label="New Name"]').waitFor({ state: 'visible' });
  await page.locator('input[aria-label="New Name"]').fill('renamed.txt');
  await page.locator('button[aria-label="Submit"]').click();
  await checkForNotification(page, "Item renamed successfully!");

  // verify its no longer in index via search
  await page.locator('#search').click()
  await page.locator('#main-input').fill('renameme.txt');
  await expect(page.locator('#result-list ul li.search-entry')).toHaveCount(0);
  checkForErrors();
})

test("create a file with the same name as a directory", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await openContextMenu();
  await page.locator('button[aria-label="New file"]').click();
  await page.locator('input[aria-label="FileName Field"]').waitFor({ state: 'visible' });
  await page.locator('input[aria-label="FileName Field"]').fill('mytest');
  await page.locator('button[aria-label="Create"]').click();
  await page.locator('a[aria-label="mytest"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="mytest"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1');
  await page.locator('button[aria-label="Delete"]').click();
  await expect(page.locator('.card-content')).toHaveText('Are you sure you want to delete this file/folder?/mytest');
  await expect(page.locator('div[aria-label="delete-path"]')).toHaveText('/mytest');
  await page.locator('button[aria-label="Confirm-Delete"]').click();
  await checkForNotification(page, "Deleted successfully!");
  await openContextMenu();
  await page.locator('button[aria-label="New folder"]').click();
  await page.locator('input[aria-label="FileName Field"]').waitFor({ state: 'visible' });
  await page.locator('input[aria-label="FileName Field"]').fill('mytest');
  await page.locator('button[aria-label="Create"]').click();
  await page.locator('a[aria-label="mytest"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="mytest"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - mytest");
  checkForErrors();
})