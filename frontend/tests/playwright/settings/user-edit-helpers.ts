import type { Locator, Page } from "@playwright/test";
import { expect } from "../test-setup";

/** Users tab uses SettingsTable; scoped to `.settings-table` body rows only. */
export function userRowInSettingsUsersTable(page: Page, usernameText: string): Locator {
  return page.locator("table.settings-table tbody tr").filter({ hasText: usernameText });
}

/** Edit opener: `role="button"` with `$t('general.edit')` (typically "Edit"). */
export function editUserTrigger(row: Locator): Locator {
  return row.getByRole("button", { name: /Edit/i });
}

export function userEditModal(page: Page): Locator {
  return page.locator('div[aria-label="user-edit-prompt"]');
}

export function userEditPreferencesModal(page: Page): Locator {
  return page.locator('div[aria-label="user-edit-preferences-prompt"]');
}

export function globalPermissionCheckbox(modal: Locator, label: string): Locator {
  return modal.locator(".toggle-container", { hasText: label }).locator('input[type="checkbox"]');
}

export function globalPermissionToggle(modal: Locator, label: string): Locator {
  return modal.locator(".toggle-container", { hasText: label }).locator("label.switch");
}

/** Primary source in settings Playwright docker config (`_docker/src/settings/backend/config.yaml`). */
export const SETTINGS_TEST_SOURCE = "playwright + files";

/** Scope block for one source in the user edit modal (per-source permissions live here). */
export function userEditScopeBlock(modal: Locator, sourceName: string): Locator {
  const escaped = sourceName.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  return modal.locator(
    `.scope-block:has([aria-label="user-edit-scope-path-${escaped}"])`,
  );
}

export async function expandUserEditSourceScope(modal: Locator, sourceName: string) {
  const block = userEditScopeBlock(modal, sourceName);
  await block.locator(".settings-group-title").click();
  await expect(block.locator(".source-file-permissions")).toBeVisible();
}

export function userEditSourcePermissionCheckbox(
  modal: Locator,
  sourceName: string,
  permissionLabel: string,
): Locator {
  return userEditScopeBlock(modal, sourceName)
    .locator(".source-file-permissions .toggle-container", { hasText: permissionLabel })
    .locator('input[type="checkbox"]');
}

export function userEditSourcePermissionToggle(
  modal: Locator,
  sourceName: string,
  permissionLabel: string,
): Locator {
  return userEditScopeBlock(modal, sourceName)
    .locator(".source-file-permissions .toggle-container", { hasText: permissionLabel })
    .locator("label.switch");
}

function isPublicUserGet(response: import("@playwright/test").Response, username: string): boolean {
  if (!response.ok() || response.request().method() !== "GET") {
    return false;
  }
  const url = new URL(response.url());
  return url.pathname.endsWith("/api/users") && url.searchParams.get("username") === username;
}

/** Wait until the user edit form has loaded user data and is interactive. */
export async function waitForUserEditReady(modal: Locator): Promise<void> {
  await expect(modal.getByRole("button", { name: "Save" })).toBeVisible();
  await expect(modal.getByRole("button", { name: "Cancel" })).toBeVisible();
  await expect(modal.locator("#loginMethod")).toBeVisible({ timeout: 10000 });
  await expect(modal.getByRole("button", { name: "User preferences" })).toBeVisible({
    timeout: 10000,
  });
}

/** Open the nested user preferences editor from the main user edit prompt. */
export async function openUserEditPreferences(page: Page, editModal: Locator): Promise<Locator> {
  await editModal.getByRole("button", { name: "User preferences" }).click();
  const prefsModal = userEditPreferencesModal(page);
  await expect(prefsModal).toBeVisible();
  await expect(globalPermissionCheckbox(prefsModal, "Administrator")).toBeVisible({
    timeout: 10000,
  });
  return prefsModal;
}

/** Close the nested user preferences editor and return to the main user edit prompt. */
export async function closeUserEditPreferences(page: Page): Promise<void> {
  const prefsModal = userEditPreferencesModal(page);
  await prefsModal.getByRole("button", { name: "Close" }).click();
  await expect(prefsModal).not.toBeVisible();
}

/**
 * Open the user edit prompt and wait for the form to be ready.
 * Uses UI readiness instead of /api/settings/sources (already loaded at app start).
 */
export async function openUserEdit(
  page: Page,
  row: Locator,
  options?: { username?: string },
): Promise<Locator> {
  const modal = userEditModal(page);
  const username = options?.username;
  const userResponse = username
    ? page.waitForResponse((response) => isPublicUserGet(response, username))
    : null;

  await editUserTrigger(row).click();
  await expect(modal).toBeVisible();

  if (userResponse) {
    await Promise.race([
      userResponse,
      modal.getByRole("button", { name: "User preferences" }).waitFor({
        state: "visible",
        timeout: 10000,
      }),
    ]).catch(() => {});
  }

  await waitForUserEditReady(modal);
  return modal;
}

/**
 * User POST/PUT/DELETE for sensitive actions return 401 until X-Password is supplied.
 * Password must match tests/playwright/global-setup.ts.
 */
export async function confirmActorPasswordPrompt(page: Page) {
  const passwordModal = page.locator(
    'div[aria-label="password-prompt"]:not(.prompt-behind)',
  );
  await expect(passwordModal).toBeVisible();
  await passwordModal.locator("input").fill("admin");
  await passwordModal.locator('button[aria-label="Confirm"]').click();
  await expect(passwordModal).not.toBeVisible();
}
