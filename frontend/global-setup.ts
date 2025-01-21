import { Browser, firefox, expect, Page } from "@playwright/test";


async function globalSetup() {
    const browser: Browser = await firefox.launch();
    const context = await browser.newContext();
    const page: Page = await context.newPage();
    await page.goto("http://127.0.0.1/login");
    await page.getByPlaceholder("Username").fill("admin");
    await page.getByPlaceholder("Password").fill("admin");
    await page.getByRole("button", { name: "Login" }).click();
    await page.waitForURL("**/files/", { timeout: 100 });
    let cookies = await context.cookies();
    expect(cookies.find((c) => c.name == "auth")?.value).toBeDefined();
    await expect(page).toHaveTitle('playwright-files - FileBrowser Quantum - Files');
    await page.context().storageState({ path: "./loginAuth.json" });
    await browser.close();
}

export default globalSetup