import type { Browser, Page } from "@playwright/test";
import { firefox } from "@playwright/test";
import { loginPlaywrightAdmin } from "./playwright-auth";

// Perform authentication and store auth state
async function localSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  await page.goto("http://127.0.0.1/login");
  await loginPlaywrightAdmin(page);
  await page.waitForURL("**/files/", { timeout: 1000 });
  await context.storageState({ path: "loginAuth.json" });
  await browser.close();
}

export default localSetup;
