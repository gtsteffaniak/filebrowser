import { expect, test } from "../test-setup.ts";

test("Create first new file", async ({ page, checkForErrors }, testInfo) => {
  // johndoe is in OIDC adminGroup "Admins", so CreateUser keeps source root (not an
  // empty createUserDir home). playwright-files already has content — do not require
  // the empty-state message. Unique name so retries in the same container stay safe.
  const fileName = `oidc-new-${testInfo.retry}-${Date.now()}.txt`;

  await page.goto("/");
  await expect(page.locator(".listing-items")).toBeVisible();
  await expect(page.locator(".listing-items .message.delayed")).toHaveCount(0);

  await page.locator(".listing-items").click({ button: "right" });
  await page.locator('button[aria-label="New file"]').click();
  await page.locator('button[aria-label="Empty file"]').click();
  await page.locator('input[aria-label="FileName Field"]').fill(fileName);
  await page.locator('button[aria-label="Create"]').click();
  await page.locator(".notification-buttons .button").waitFor({ state: "visible" });
  await page.locator(".notification-buttons .button").click();
  await expect(page).toHaveTitle(`Graham's Filebrowser - Files - ${fileName}`);
  await page.locator('button[aria-label="Close"]').click();
  await expect(page.locator(`.listing-items [aria-label="${fileName}"]`)).toBeVisible();
  checkForErrors();
});
