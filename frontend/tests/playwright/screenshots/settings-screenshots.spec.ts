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
test("profile settings", async ({ page, checkForErrors, context, theme }) => {
  await page.goto("/files/settings/");
  const listingOptionsDiv = page.locator('div[aria-label="listingOptions"]');
  await listingOptionsDiv.click(); // collapse the listing options section

  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-listing-options-${theme}.jpg`, quality: jpgQuality });
  if (theme === 'light') {
    return;
  }
  await listingOptionsDiv.click(); // open the listing options section
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-settings-container-${theme}.jpg`, quality: jpgQuality });
  await listingOptionsDiv.click();

  const thumbnailDiv = page.locator('div[aria-label="thumbnailOptions"]');
  await thumbnailDiv.click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-thumbnail-options-${theme}.jpg`, quality: jpgQuality });
  
  const sidebarDiv = page.locator('div[aria-label="sidebarOptions"]');
  await sidebarDiv.click();
  await sidebarDiv.evaluate(el => el.scrollIntoView({ block: 'center', behavior: 'instant' }));
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-sidebar-options-${theme}.jpg`, quality: jpgQuality });
  
  const searchDiv = page.locator('div[aria-label="searchOptions"]');
  await searchDiv.click();
  await searchDiv.evaluate(el => el.scrollIntoView({ block: 'center', behavior: 'instant' }));
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-search-options-${theme}.jpg`, quality: jpgQuality });
  
  const fileViewerDiv = page.locator('div[aria-label="fileViewerOptions"]');
  await fileViewerDiv.click();
  await fileViewerDiv.evaluate(el => el.scrollIntoView({ block: 'center', behavior: 'instant' }));
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-file-viewer-options-${theme}.jpg`, quality: jpgQuality });
  
  const themeLanguageDiv = page.locator('div[aria-label="themeLanguage"]');
  await themeLanguageDiv.click();
  await themeLanguageDiv.evaluate(el => el.scrollIntoView({ block: 'center', behavior: 'instant' }));
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-theme-language-options-${theme}.jpg`, quality: jpgQuality });
});

// run npx playwright test --ui to run these tests locally in ui mode
test("Uploads & Downloads settings", async ({ page, checkForErrors, context, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/settings#fileLoading-main");
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/uploads-downloads-options-${theme}.jpg`, quality: jpgQuality });

});
