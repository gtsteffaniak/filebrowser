import { test, expect } from "@playwright/test";

test("redirect to login from root", async ({ page, context }) => {
  await context.clearCookies();
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);

});

test("redirect to login from files", async ({ page, context }) => {
  await context.clearCookies();
  await page.goto("/files/files/");
  await expect(page).toHaveURL(/\/login\?redirect=\/files\//);
});

test("logout", async ({ page, context }) => {
  await page.goto('/');
  await expect(page.locator("div.wrong")).toBeHidden();
  await page.waitForURL("**/files/playwright-files", { timeout: 100 });
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  let cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "auth")?.value).toBeDefined();
  await page.locator('button[aria-label="logout-button"]').click();
  await page.waitForURL("**/login", { timeout: 100 });
  await expect(page).toHaveTitle("Graham's Filebrowser - Login");
  cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "auth")?.value).toBeUndefined();
});