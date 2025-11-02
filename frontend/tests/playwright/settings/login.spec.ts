import { test, expect } from "@playwright/test";

test("redirect to login", async ({ page, context }) => {
  await context.clearCookies();
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
});

test("signup failure -- password mismatch shows tooltip", async ({ page, context }) => {
  await context.clearCookies();
  await page.goto("/login");
  
  // Toggle to signup mode
  await page.locator('p[aria-label="sign up toggle"]').click();
  
  // Fill in form with mismatched passwords
  await page.getByPlaceholder("Username").fill("testuser");
  await page.getByPlaceholder("Password").first().fill("password123");
  await page.getByPlaceholder(/confirm/i).fill("password456");
  
  // Submit
  await page.locator('input[type="submit"]').click();
  
  // Error card should appear
  await expect(page.locator('.wrong-login')).toBeVisible();
  
  // Hover over info icon to show tooltip
  await page.locator('.tooltip-info-icon').hover();
  
  // Tooltip should appear with error details
  await expect(page.locator('.floating-tooltip')).toBeVisible();
});
  