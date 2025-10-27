import { test, expect } from "@playwright/test";

test("redirect to login from root", async ({ page, context }) => {
  await context.clearCookies();
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
});

test("redirect to login from files", async ({ page, context }) => {
  await context.clearCookies();
  await page.goto("/files/");
  await expect(page).toHaveURL(/\/login\?redirect=\/files\//);
});

test("logout", async ({ browser }) => {
  const context = await browser.newContext({
    storageState: undefined,
    baseURL: "http://127.0.0.1/",
  });
  const page = await context.newPage();

  await page.goto("/login");
  await page.getByPlaceholder("Username").fill("admin");
  await page.getByPlaceholder("Password").fill("admin");
  await page.getByRole("button", { name: "Login" }).click();
  await page.waitForURL("**/files/**");

  await expect(page.locator("div.wrong")).toBeHidden();
  await expect(page).toHaveTitle(/Graham's Filebrowser - Files/);
  let cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "filebrowser_quantum_jwt")?.value).toBeDefined();
  await page.locator('button[aria-label="logout-button"]').click();
  await page.waitForURL("**/login", { timeout: 2000 });
  await expect(page).toHaveTitle("Graham's Filebrowser - Login");
  cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "filebrowser_quantum_jwt")?.value).toBeUndefined();

  await context.close();
});