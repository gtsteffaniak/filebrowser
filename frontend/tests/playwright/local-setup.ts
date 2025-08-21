import { Browser, firefox, expect, Page, FullConfig } from "@playwright/test";

// Perform authentication and store auth state
async function localSetup(config: FullConfig) {
  const { baseURL } = config.projects[0].use;
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext({
    baseURL
  });
  const page: Page = await context.newPage();

  await page.goto("/testing/login");
  await page.getByPlaceholder("Username").fill("admin");
  await page.getByPlaceholder("Password").fill("admin");
  await page.getByRole("button", { name: "Login" }).click();
  await page.waitForURL("**/files/", { timeout: 500 });
  await context.storageState({ path: "./loginAuth.json" });
  await browser.close();
}

export default localSetup;