import type { Page } from "@playwright/test";
import { expect, test } from "../test-setup";
import {
    SETTINGS_TEST_SOURCE,
    confirmActorPasswordPrompt,
    expandUserEditSourceScope,
    openUserEdit,
    userEditSourcePermissionCheckbox,
    userEditSourcePermissionToggle,
    userRowInSettingsUsersTable,
} from "./user-edit-helpers";

test("create, check settings, and delete user (retry-safe name)", async ({
    page,
    checkForErrors,
}, testInfo) => {
    test.setTimeout(30000);

    const username = `testuser2-${testInfo.retry + 1}`;
    await page.goto("/settings");
    await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
    await page.locator("#users-sidebar").click();
    await page.locator('button[aria-label="New user"]').click();
    await page.locator("#username").fill(username);
    await page.locator('input[aria-label="Password1"]').fill("testpassword");
    await page.locator('input[aria-label="Password2"]').fill("testpass");
    await expect(page.locator('input[aria-label="Password2"]')).toHaveClass(
        "input form-form form-invalid",
    );
    await page.locator('input[aria-label="Password2"]').fill("testpassword");

    const createResponse = page.waitForResponse(
        (resp) =>
            resp.url().includes("/api/users") &&
            resp.request().method() === "POST" &&
            resp.status() === 201,
    );
    await page.locator('button[aria-label="Save"]').click();
    await confirmActorPasswordPrompt(page);
    await createResponse;

    await expect(userRowInSettingsUsersTable(page, username)).toBeVisible();

    const modal = await openUserEdit(
        page,
        userRowInSettingsUsersTable(page, username),
        { username },
    );

    const settingsToToggle = [
        "Administrator",
        "Prevent the user from changing the password",
        "Share files",
        "Create and manage long-live API tokens",
        "Enable real-time connections and updates",
    ];

    for (const settingName of settingsToToggle) {
        const toggleContainer = modal.locator(".toggle-container", { hasText: settingName });
        await toggleContainer.locator("label.switch").click();
    }

    await modal.locator('button[aria-label="Save"]').click();
    await confirmActorPasswordPrompt(page);
    await expect(modal).not.toBeVisible();

    await openUserEdit(page, userRowInSettingsUsersTable(page, username), { username });

    for (const settingName of settingsToToggle) {
        const checkbox = modal.locator(
            `.toggle-container:has-text("${settingName}") input[type="checkbox"]`,
        );
        await expect(checkbox).toBeChecked();
    }

    await modal.locator('button[aria-label="Delete User"]').click();
    const genericModal = page.locator('div[aria-label="generic-prompt"]');
    await expect(genericModal).toBeVisible();
    await genericModal.locator('button[aria-label="Delete"]').click();
    await confirmActorPasswordPrompt(page);

    await expect(userRowInSettingsUsersTable(page, username)).not.toBeVisible();
    checkForErrors(0, 3);
});

test("two factor auth check", async ({ page, checkForErrors }) => {
    test.setTimeout(30000);

    await page.goto("/settings");
    await expect(page).toHaveURL(/\/settings/);
    await page.locator("#users-sidebar").click();

    const modal = await openUserEdit(
        page,
        userRowInSettingsUsersTable(page, "admin"),
        { username: "admin" },
    );

    const twoFactorCheckbox = modal.locator(
        '.toggle-container:has-text("Two-Factor Authentication") input[type="checkbox"]',
    );
    const twoFactorToggle = modal.locator(
        '.toggle-container:has-text("Two-Factor Authentication") label.switch',
    );
    await twoFactorToggle.click();
    await expect(twoFactorCheckbox).toBeChecked();
    await modal.locator('button[aria-label="Generate Code"]').click();

    const passwordModal = page.locator(
        'div[aria-label="password-prompt"]:not(.prompt-behind)',
    );
    await passwordModal.locator("input").fill("admin");
    await passwordModal.locator('button[aria-label="Confirm"]').click();

    const totpModal = page.locator('div[aria-label="totp-prompt"]');
    await expect(totpModal.locator('p[aria-label="otp-url"]')).toBeVisible();
    const otpUrl = await totpModal.locator('p[aria-label="otp-url"]').textContent();
    expect(otpUrl).not.toBe("");
    checkForErrors();
});

test.describe("User Settings Persistence", () => {
    test.describe.configure({ timeout: 30000 });

    const username = "testuser1";
    test.beforeEach(async ({ page }) => {
        await page.goto("/settings");
        await page.locator("#users-sidebar").click();
    });

    async function checkTogglePersistence(page: Page, settingName: string) {
        const userRow = userRowInSettingsUsersTable(page, username);
        await expect(userRow).toBeVisible({ timeout: 5000 });

        const modal = await openUserEdit(page, userRow, { username });
        const checkbox = modal.locator(
            `.toggle-container:has-text("${settingName}") input[type="checkbox"]`,
        );
        const initialChecked = await checkbox.isChecked();

        const toggleSwitch = modal
            .locator(".toggle-container", { hasText: settingName })
            .locator("label.switch");
        await toggleSwitch.click();
        await expect(checkbox).toBeChecked({ checked: !initialChecked });
        await modal.locator('button[aria-label="Save"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(modal).not.toBeVisible();

        await openUserEdit(page, userRow, { username });
        const checkboxToggled = modal.locator(
            `.toggle-container:has-text("${settingName}") input[type="checkbox"]`,
        );
        await expect(checkboxToggled).toBeChecked({ checked: !initialChecked });

        const toggleSwitchBack = modal
            .locator(".toggle-container", { hasText: settingName })
            .locator("label.switch");
        await toggleSwitchBack.click();
        await expect(checkboxToggled).toBeChecked({ checked: initialChecked });
        await modal.locator('button[aria-label="Save"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(modal).not.toBeVisible();

        await openUserEdit(page, userRow, { username });
        const checkboxRestored = modal.locator(
            `.toggle-container:has-text("${settingName}") input[type="checkbox"]`,
        );
        await expect(checkboxRestored).toBeChecked({ checked: initialChecked });
        await modal.locator('button[aria-label="Cancel"]').click();
    }

    test('should persist "Prevent the user from changing the password" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Prevent the user from changing the password");
    });

    test('should persist "Administrator" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Administrator");
    });

    test('should persist per-source "Edit files" setting', async ({ page }) => {
        const userRow = userRowInSettingsUsersTable(page, username);
        const modal = await openUserEdit(page, userRow, { username });
        await expandUserEditSourceScope(modal, SETTINGS_TEST_SOURCE);

        const editFilesToggle = userEditSourcePermissionToggle(
            modal,
            SETTINGS_TEST_SOURCE,
            "Edit files",
        );
        const editFilesCheckbox = userEditSourcePermissionCheckbox(
            modal,
            SETTINGS_TEST_SOURCE,
            "Edit files",
        );

        const wasChecked = await editFilesCheckbox.isChecked();
        await editFilesToggle.click();
        await expect(editFilesCheckbox).toBeChecked({ checked: !wasChecked });

        await modal.locator('button[aria-label="Save"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(modal).not.toBeVisible();

        await openUserEdit(page, userRow, { username });
        await expandUserEditSourceScope(modal, SETTINGS_TEST_SOURCE);
        await expect(editFilesCheckbox).toBeChecked({ checked: !wasChecked });

        await editFilesToggle.click();
        await modal.locator('button[aria-label="Save"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(modal).not.toBeVisible();
    });

    test('should persist "Share files" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Share files");
    });

    test('should persist "Create and manage long-live API tokens" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Create and manage long-live API tokens");
    });

    test('should persist "Enable real-time connections and updates" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Enable real-time connections and updates");
    });

    test('should persist "allowed login method" setting', async ({ page, checkForErrors }) => {
        const userRow = userRowInSettingsUsersTable(page, username);
        const modal = await openUserEdit(page, userRow, { username });
        await expect(
            modal.locator("#loginMethod .expand-dropdown-trigger-label"),
        ).toHaveText("Password");
        checkForErrors();
    });
});
