import { expect, test } from "../test-setup";

async function openUserDefaultsPrompt(page: import("@playwright/test").Page) {
  await page.goto("/files/");
  await page.locator('i[aria-label="settings"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator("#users-sidebar").click();
  await page.getByText("User defaults", { exact: true }).click();
  await expect(page.getByRole("heading", { name: "User defaults" })).toBeVisible();
}

test("config-locked user defaults show lock help and skip patch", async ({ page, checkForErrors }) => {
  const defaultsResponse = page.waitForResponse(
    (response) => response.url().includes("/api/settings/user-defaults") && response.ok(),
  );
  await openUserDefaultsPrompt(page);
  const response = await defaultsResponse;
  const data = await response.json();
  expect(data.lockedFromConfigPaths).toContain("listing.showHidden");

  const showHiddenRow = page.locator(".user-defaults-prompt .item").filter({ hasText: "Show hidden files" });
  await showHiddenRow.hover();
  await expect(page.getByText("This default is set in the config file and cannot be changed here.")).toBeVisible();

  let patchCount = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/settings/user-defaults") && request.method() === "PATCH") {
      patchCount += 1;
    }
  });

  const toggleInput = showHiddenRow.locator("input[type='checkbox']");
  const initialChecked = await toggleInput.isChecked();
  await toggleInput.click({ force: true });
  await page.waitForTimeout(300);
  expect(patchCount).toBe(0);
  expect(await toggleInput.isChecked()).toBe(!initialChecked);

  checkForErrors();
});
