import type { Locator, Page } from "@playwright/test";

/** Default bootstrap admin for Playwright Docker stacks (see auth.adminPassword in _docker). */
export const PLAYWRIGHT_ADMIN_USERNAME = "admin";
export const PLAYWRIGHT_ADMIN_PASSWORD = "playwright-password";

export async function fillPlaywrightAdminLogin(page: Page): Promise<void> {
  await page.getByPlaceholder("Username").fill(PLAYWRIGHT_ADMIN_USERNAME);
  await page.getByPlaceholder("Password").fill(PLAYWRIGHT_ADMIN_PASSWORD);
}

export async function loginPlaywrightAdmin(page: Page): Promise<void> {
  await fillPlaywrightAdminLogin(page);
  await page.getByRole("button", { name: "Login" }).click();
}

export async function fillPlaywrightAdminPasswordPrompt(
  passwordModal: Locator,
): Promise<void> {
  await passwordModal.locator("input").fill(PLAYWRIGHT_ADMIN_PASSWORD);
}
