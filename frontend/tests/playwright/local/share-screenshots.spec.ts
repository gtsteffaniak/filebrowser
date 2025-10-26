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
