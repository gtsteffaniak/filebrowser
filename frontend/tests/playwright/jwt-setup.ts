import { Browser, firefox, expect, Page,  } from "@playwright/test";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  await page.goto("http://127.0.0.1:8084/"); // no user
  await expect(page).toHaveTitle("Graham's Filebrowser - Login");
}

export default globalSetup;