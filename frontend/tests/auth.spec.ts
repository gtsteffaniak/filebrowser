import { test, expect } from "@playwright/test";

test("redirect to login", async ({ page, context }) => {
  await context.clearCookies();

  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);

  await page.goto("/files/");
  await expect(page).toHaveURL(/\/login\?redirect=\/files\//);
});

test("logout", async ({ page, context }) => {
  await page.goto('/');
  await expect(page.locator("div.wrong")).toBeHidden();
  await page.waitForURL("**/files/", { timeout: 100 });
  await expect(page).toHaveTitle('playwright-files - FileBrowser Quantum - Files');
  let cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "auth")?.value).toBeDefined();
  await page.locator('div.inner-card.logout-button').click();
  await page.waitForURL("**/login", { timeout: 100 });
  await expect(page).toHaveTitle('FileBrowser Quantum - Login');
  cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "auth")?.value).toBeUndefined();
});