import { expect, test } from "../test-setup";

function isSourceDefaultsGet(response: import("@playwright/test").Response) {
  const pathname = new URL(response.url()).pathname;
  return (
    pathname.endsWith("/settings/source") &&
    response.request().method() === "GET" &&
    response.ok()
  );
}

async function openAccessSettings(page: import("@playwright/test").Page) {
  const defaultsResponse = page.waitForResponse(isSourceDefaultsGet);
  await page.goto("/settings");
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
  await page.locator("#access-sidebar").click();

  const permissionsGroup = page
    .locator(".settings-group")
    .filter({ hasText: "Permissions" });
  const content = permissionsGroup.locator(".settings-content");
  if (!(await content.isVisible())) {
    await permissionsGroup.locator(".settings-group-title.button").click();
    await expect(content).toBeVisible();
  }
  await expect(permissionsGroup.locator(".loading-hint")).not.toBeVisible();

  return defaultsResponse;
}

test("config-locked source defaults show lock help and skip patch", async ({ page, checkForErrors }) => {
  try {
    const defaultsResponse = await openAccessSettings(page);
    const data = await defaultsResponse.json();
    expect(data.lockedFromConfigPaths).toContain("defaultPermissions.modify");

    const modifyRow = page
      .locator(".source-file-permissions .item")
      .filter({ hasText: "Edit files" });
    await expect(modifyRow).toBeVisible();
    await modifyRow.locator(".toggle-row--value").hover();
    const lockTooltip = page.locator(".floating-tooltip");
    await expect(lockTooltip).toBeVisible();
    await expect(lockTooltip).toHaveText(
      "This default is set in the config file and cannot be changed here.",
    );

    let patchCount = 0;
    await page.route("**/api/settings/source", (route) => {
      if (route.request().method() === "PATCH") {
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
      (resp) =>
        new URL(resp.url()).pathname.endsWith("/settings/source") &&
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
