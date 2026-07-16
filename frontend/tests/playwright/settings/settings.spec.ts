import { checkForNotification, expect, selectExpandDropdownOption, test } from '../test-setup'
import type { Page } from '@playwright/test';

async function openSystemAdminSettings(page: Page) {
  await page.locator('i[aria-label="settings"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  const analyticsResponse = page.waitForResponse(
    (response) => response.url().includes("/api/settings/analytics") && response.ok(),
  );
  await page.locator('#systemAdmin-sidebar').click();
  await analyticsResponse;
}

test("adjusting theme colors", async({ page, checkForErrors }) => {
  await page.goto("/files/");
  const originalPrimaryColor = await page.evaluate(() => {
    return getComputedStyle(document.documentElement).getPropertyValue('--primaryColor').trim();
  });
  expect(originalPrimaryColor).toBe('#2196f3');

  // Verify the page title
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('i[aria-label="settings"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator('div[aria-label="themeLanguage"]').click();
  await page.locator('button', { hasText: 'violet' }).click();
  await checkForNotification(page, 'Settings updated!');
  const newPrimaryColor = await page.evaluate(() => {
    return getComputedStyle(document.documentElement).getPropertyValue('--primaryColor').trim();
  });
  expect(newPrimaryColor).toBe('#9b59b6');
  // Check for console errors
  checkForErrors();
});

test("choose custom theme", async({ page, checkForErrors }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await page.locator('i[aria-label="settings"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator('div[aria-label="themeLanguage"]').click();
  // a custom no-rounded.css theme file added to docker that should exist and be selectable
  await selectExpandDropdownOption(page, 'Theme', /^no-rounded/);
  await checkForNotification(page, 'Settings updated!');
  // Check for console errors
  checkForErrors();
});

test("view config", async({ page, checkForErrors }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await openSystemAdminSettings(page);

  const configResponse = page.waitForResponse(
    (response) => response.url().includes("/api/settings/config") && response.ok(),
  );
  await page.getByRole("button", { name: "View configuration" }).click();
  await configResponse;
  await expect(page.locator(".ace_text-layer .ace_line").first()).toContainText("server:");
  checkForErrors();
});

test("view analytics diagnostic", async({ page, checkForErrors }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
  await openSystemAdminSettings(page);

  await expect(page.getByText("Send deployment analytics")).toBeVisible();
  await expect(page.getByRole("button", { name: "View analytics" })).toBeVisible();

  const previewResponsePromise = page.waitForResponse(
    (response) => response.url().includes("/api/settings/analytics/preview") && response.ok(),
  );
  await page.getByRole("button", { name: "View analytics" }).click();
  const previewResponse = await previewResponsePromise;
  const preview = await previewResponse.json();

  expect(preview.schema_version).toBe("1");
  expect(preview.event_type).toBe("deployment_snapshot");
  expect(typeof preview.installation_id).toBe("string");
  expect(preview.installation_id.length).toBeGreaterThan(0);

  await expect(page.locator(".ace_text-layer")).toContainText("deployment_snapshot");
  checkForErrors();
});
