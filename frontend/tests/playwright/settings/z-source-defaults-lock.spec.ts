import {
  expect,
  test,
  expectLockTooltipOnRowHover,
  isExactSettingsApiResponse,
} from "../test-setup";

async function openAccessSettings(page: import("@playwright/test").Page) {
  await page.goto("/settings#profile-main");
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");

  const defaultsResponse = page.waitForResponse(
    (response) => isExactSettingsApiResponse(response, "settings/source", "GET"),
  );
  await page.locator("#access-sidebar").click();

  const permissionsGroup = page
    .locator(".settings-group")
    .filter({ hasText: "Permissions" });
  const content = permissionsGroup.locator(".settings-content");
  if (!(await content.isVisible())) {
    await permissionsGroup.locator(".settings-group-title.button").click();
    await expect(content).toBeVisible();
  }

  await expect(page.locator(".source-file-permissions")).toBeVisible();
  await expect(
    page.locator(".source-file-permissions .item").filter({ hasText: "Edit files" }),
  ).toBeVisible();

  const response = await defaultsResponse;
  const data = await response.json();
  if (!Array.isArray(data.lockedFromConfigPaths)) {
    const apiResponse = await page.request.get("http://127.0.0.1/api/settings/source");
    expect(apiResponse.ok()).toBeTruthy();
    return apiResponse;
  }
  return response;
}

test("config-locked source defaults show lock help and skip patch", async ({ page, checkForErrors }) => {
  test.setTimeout(15000);

  try {
    const defaultsResponse = await openAccessSettings(page);
    const data = await defaultsResponse.json();
    expect(data.lockedFromConfigPaths).toContain("defaultPermissions.modify");

    const modifyRow = page
      .locator(".source-file-permissions .item")
      .filter({ hasText: "Edit files" });
    await expect(modifyRow).toBeVisible();
    await expectLockTooltipOnRowHover(
      page,
      modifyRow,
      "This default is set in the config file and cannot be changed here.",
    );

    let patchCount = 0;
    await page.route("**/api/settings/source", (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/api/settings/source") && route.request().method() === "PATCH") {
        patchCount += 1;
      }
      return route.continue();
    });

    const valueInput = modifyRow.locator(".toggle-row--value input[type='checkbox']");
    const valueSwitch = modifyRow.locator(".toggle-row--value label.switch");
    expect(await valueInput.isChecked()).toBe(false);
    await expect(valueInput).toBeDisabled();
    await valueSwitch.click({ force: true });
    await expect.poll(() => patchCount).toBe(0);
    expect(await valueInput.isChecked()).toBe(false);

    const enforceInput = modifyRow.locator(".toggle-row--enforced input[type='checkbox']");
    const enforceSwitch = modifyRow.locator(".toggle-row--enforced label.switch");
    await expect(enforceInput).toBeEnabled();
    const initialEnforced = await enforceInput.isChecked();
    const enforcePatch = page.waitForResponse(
      (resp) => isExactSettingsApiResponse(resp, "settings/source", "PATCH"),
    );
    await enforceSwitch.click();
    await enforcePatch;
    expect(patchCount).toBe(1);
    expect(await enforceInput.isChecked()).toBe(!initialEnforced);
  } finally {
    checkForErrors();
  }
});
