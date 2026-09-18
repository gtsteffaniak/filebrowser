//import { Page } from "@playwright/test";
import { openAdvancedProfileSettings, test } from "../test-setup";

const jpgQuality = 85;

// this file has playwright tests that create screenshots of the UI
test("setup theme", async ({ page, theme }) => {
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
test("profile settings", async ({ page, theme }) => {
  await openAdvancedProfileSettings(page, "listingOptions");
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/profile-listing-options-${theme}.jpg`, quality: jpgQuality });
  if (theme === 'light') {
    return;
  }
  await page.screenshot({ path: `./generated/settings/profile-settings-container-${theme}.jpg`, quality: jpgQuality });

  const sections = [
    { id: "thumbnailOptions", file: "profile-thumbnail-options" },
    { id: "sidebarOptions", file: "profile-sidebar-options" },
    { id: "searchOptions", file: "profile-search-options" },
    { id: "fileViewerOptions", file: "profile-file-viewer-options" },
    { id: "themeLanguage", file: "profile-theme-language-options" },
  ];

  for (const section of sections) {
    await openAdvancedProfileSettings(page, section.id);
    await page.locator(`div[aria-label="${section.id}"]`).evaluate((el) => {
      el.scrollIntoView({ block: "center", behavior: "instant" });
    });
    await page.waitForTimeout(300);
    await page.screenshot({
      path: `./generated/settings/${section.file}-${theme}.jpg`,
      quality: jpgQuality,
    });
  }
});

// run npx playwright test --ui to run these tests locally in ui mode
test("Uploads & Downloads settings", async ({ page, theme }) => {
  if (theme === 'light') {
    return;
  }
  await page.goto("/files/settings#fileLoading-main");
  await page.waitForTimeout(300);
  await page.screenshot({ path: `./generated/settings/uploads-downloads-options-${theme}.jpg`, quality: jpgQuality });

});
