import { expect, test } from "../test-setup";

async function expandUserDefaultsGroup(
  page: import("@playwright/test").Page,
  groupTitle: string,
) {
  const prompt = page.locator('[aria-label="user-defaults-prompt"]');
  const groupRoot = prompt.locator(".settings-group").filter({ hasText: groupTitle });
  const content = groupRoot.locator(".settings-content");
  if (!(await content.isVisible())) {
    await groupRoot.locator(".settings-group-title.button").click();
    await expect(content).toBeVisible();
  }
}

async function openUserDefaultsPrompt(page: import("@playwright/test").Page) {
  await page.goto("/settings");
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator("#users-sidebar").click();

  const defaultsResponse = page.waitForResponse(
    (response) =>
      response.url().includes("/api/settings/user-defaults") &&
      response.request().method() === "GET" &&
      response.ok(),
  );
  await page.getByRole("button", { name: "User defaults" }).click();

  const prompt = page.locator('[aria-label="user-defaults-prompt"]');
  await expect(prompt).toBeVisible();
  await expect(prompt.locator(".loading-hint")).not.toBeVisible();

  return defaultsResponse;
}

test("config-locked user defaults show lock help and skip patch", async ({ page, checkForErrors }) => {
  try {
    const defaultsResponse = await openUserDefaultsPrompt(page);
    const response = await defaultsResponse;
    const data = await response.json();
    expect(data.lockedFromConfigPaths).toContain("listing.showHidden");

    await expandUserDefaultsGroup(page, "Listing options");

    const showHiddenRow = page.locator(".user-defaults-prompt .item").filter({ hasText: "Show hidden files" });
    await expect(showHiddenRow).toBeVisible();
    await showHiddenRow.locator(".toggle-row--value").hover();
    const lockTooltip = page.locator(".floating-tooltip");
    await expect(lockTooltip).toBeVisible();
    await expect(lockTooltip).toHaveText(
      "This default is set in the config file and cannot be changed here.",
    );

    let patchCount = 0;
    await page.route("**/api/settings/user-defaults", (route) => {
      if (route.request().method() === "PATCH") {
        patchCount += 1;
      }
      return route.continue();
    });

    const valueInput = showHiddenRow.locator(".toggle-row--value input[type='checkbox']");
    const valueSwitch = showHiddenRow.locator(".toggle-row--value label.switch");
    expect(await valueInput.isChecked()).toBe(true);
    await expect(valueInput).toBeDisabled();
    await valueSwitch.click({ force: true });
    await expect.poll(() => patchCount).toBe(0);
    expect(await valueInput.isChecked()).toBe(true);

    const enforceInput = showHiddenRow.locator(".toggle-row--enforced input[type='checkbox']");
    const enforceSwitch = showHiddenRow.locator(".toggle-row--enforced label.switch");
    await expect(enforceInput).toBeEnabled();
    const initialEnforced = await enforceInput.isChecked();
    const enforcePatch = page.waitForResponse(
      (resp) =>
        resp.url().includes("/api/settings/user-defaults") &&
        resp.request().method() === "PATCH" &&
        resp.ok(),
    );
    await enforceSwitch.click();
    await enforcePatch;
    expect(patchCount).toBe(1);
    expect(await enforceInput.isChecked()).toBe(!initialEnforced);
  } finally {
    checkForErrors();
  }
});
