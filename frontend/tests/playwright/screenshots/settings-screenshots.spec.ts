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
  await page.waitForTimeout(100);
  await page.screenshot({ path: `./generated/settings/profile-listing-options-${theme}.jpg`, quality: jpgQuality });
  if (theme === 'light') {
    return;
  }
  await page.locator('div[aria-label="thumbnailOptions"]').click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-thumbnail-options-${theme}.jpg`, quality: jpgQuality });
  await page.locator('div[aria-label="sidebarOptions"]').click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-sidebar-options-${theme}.jpg`, quality: jpgQuality });
  await page.locator('div[aria-label="searchOptions"]').click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-search-options-${theme}.jpg`, quality: jpgQuality });
  await page.locator('div[aria-label="fileViewerOptions"]').click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-file-viewer-options-${theme}.jpg`, quality: jpgQuality });
  await page.locator('div[aria-label="themeLanguage"]').click();
  await page.waitForTimeout(300);
});
