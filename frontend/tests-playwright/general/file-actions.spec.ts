import { test, expect } from "../test-setup";


test("info from listing", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="file.tar.gz"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file.tar.gz"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Info"]').click();
  await expect(page.locator('.break-word')).toHaveText('Display Name: file.tar.gz');
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
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Info"]').click();
  await expect(page.locator('.break-word')).toHaveText('Display Name: file.tar.gz');
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
  await page.locator('#main-input').fill('graham');
  await expect(page.locator('#result-list')).toHaveCount(1);
  await page.locator('li[aria-label="graham.xlsx"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - graham.xlsx");
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
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Copy file"]').click();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /playwright-files/');
  await expect(page.locator('li[aria-selected="true"]')).toHaveCount(0);
  await page.locator('li[aria-label="myfolder"]').click();
  await expect(page.locator('li[aria-selected="true"]')).toHaveCount(1);
  await page.locator('button[aria-label="Copy"]').click();
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("Successfully copied file/folder, redirecting...");
  await page.waitForURL('**/myfolder/');
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

  await expect(page).toHaveTitle("Graham's Filebrowser - Files - newfolder");
  await page.goBack();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - myfolder");

  await page.locator('a[aria-label="copyme.txt"]').click( { button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Copy file"]').click();
  await expect(page.locator('div[aria-label="filelist-path"]')).toHaveText('Path: /playwright-files/myfolder/');
  await page.locator('li[aria-label="newfolder"]').click();
  await page.locator('button[aria-label="Copy"]').click();
  const popup2 = page.locator('#popup-notification-content');
  await popup2.waitFor({ state: 'visible' });
  await expect(popup2).toHaveText("Successfully copied file/folder, redirecting...");
  //await page.waitForURL('**/newfolder/');
  //await expect(page).toHaveTitle("Graham's Filebrowser - Files - newfolder");
  checkForErrors();
})

test("delete file", async({ page, checkForErrors, context }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('a[aria-label="deleteme.txt"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="deleteme.txt"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Delete"]').click();
  await expect( page.locator('.card-content')).toHaveText('Are you sure you want to delete this file/folder?/deleteme.txt');
  await expect(page.locator('div[aria-label="delete-path"]')).toHaveText('/deleteme.txt');
  await page.locator('button[aria-label="Confirm-Delete"]').click();
  const popup = page.locator('#popup-notification-content');
  await popup.waitFor({ state: 'visible' });
  await expect(popup).toHaveText("Deleted item successfully! reloading...");
  checkForErrors();

})

test("delete nested file prompt", async({ page, checkForErrors, context }) => {
  await page.goto("/files/playwright-files/folder%23hash/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - folder#hash");
  await page.locator('a[aria-label="file#.sh"]').waitFor({ state: 'visible' });
  await page.locator('a[aria-label="file#.sh"]').click({ button: "right" });
  await page.locator('.selected-count-header').waitFor({ state: 'visible' });
  await expect(page.locator('.selected-count-header')).toHaveText('1 selected');
  await page.locator('button[aria-label="Delete"]').click();
  await expect(page.locator('.card-content')).toHaveText('Are you sure you want to delete this file/folder?/folder#hash/file#.sh');
  await expect(page.locator('div[aria-label="delete-path"]')).toHaveText('/folder#hash/file#.sh');
  checkForErrors();
})
