import { test, expect } from "../test-setup";
import { Page, Locator } from "@playwright/test";

/** Users tab uses SettingsTable (no `tr.item`); scoped to `.settings-table` body rows only. */
function userRowInSettingsUsersTable(page: Page, usernameText: string): Locator {
    return page.locator("table.settings-table tbody tr").filter({ hasText: usernameText });
}

/** Edit opener: `role="button"` div with `$t('general.edit')` (typically "Edit"). */
function editUserTrigger(row: Locator): Locator {
    return row.getByRole("button", { name: /Edit/i });
}

/**
 * User POST/PUT/DELETE for sensitive actions return 401 until X-Password is supplied; the UI opens password-prompt on top.
 * Password must match tests/playwright/global-setup.ts (same as two factor auth check).
 */
async function confirmActorPasswordPrompt(page: Page) {
    const passwordModal = page.locator(
        'div[aria-label="password-prompt"]:not(.prompt-behind)'
    );
    await expect(passwordModal).toBeVisible();
    await passwordModal.locator("input").fill("admin");
    await passwordModal.locator('button[aria-label="Confirm"]').click();
    await expect(passwordModal).not.toBeVisible();
}

test("create, check settings, and delete user (retry-safe name)", async ({
    page,
    checkForErrors,
}, testInfo) => {
    // Unique username per run/retry so retries don't conflict with leftover users (same idea as proxy/preview.spec.ts).
    const username = `testuser2-${testInfo.retry + 1}`;
    await page.goto('/settings')
    await expect(page).toHaveTitle("Graham's Filebrowser - Settings")
    await page.locator('#users-sidebar').click();
    await page.locator('button[aria-label="Add New User"]').click()
    await page.locator('#username').fill(username)
    await page.locator('input[aria-label="Password1"]').fill('testpassword')
    await page.locator('input[aria-label="Password2"]').fill('testpass')
    // check that the invalid-field class is added properly
    await expect(page.locator('input[aria-label="Password2"]')).toHaveClass(
        'input form-form form-invalid'
    )
    await page.locator('input[aria-label="Password2"]').fill('testpassword')

    // usersApi.create tries POST without X-Password first; server returns 401, then password-prompt + retry (201).
    const createResponse = page.waitForResponse(
        (resp) =>
            resp.url().includes('/api/users') &&
            resp.request().method() === 'POST' &&
            resp.status() === 201
    );
    await page.locator('button[aria-label="Save"]').click();
    await confirmActorPasswordPrompt(page);
    await createResponse;

    // We should be back on the settings page
    await expect(userRowInSettingsUsersTable(page, username)).toBeVisible();

    // Now, click the edit button for the new user
    await editUserTrigger(userRowInSettingsUsersTable(page, username)).click();

    // Now on the edit page, toggle the settings
    await expect(page.locator('div[aria-label="user-edit-prompt"]')).toBeVisible();
    const modal = page.locator('div[aria-label="user-edit-prompt"]');

    const settingsToToggle = [
        "Administrator",
        "Prevent the user from changing the password",
        "Edit files",
        "Share files",
        "Create and manage long-live API tokens",
        "Enable real-time connections and updates",
    ];

    for (const settingName of settingsToToggle) {
        const toggleContainer = modal.locator(".toggle-container", { hasText: settingName });
        const toggleSwitch = toggleContainer.locator("label.switch");
        await toggleSwitch.click();
    }

    // Save the updated settings (admin actor password required for sensitive user fields)
    await modal.locator('button[aria-label="Save"]').click();
    await confirmActorPasswordPrompt(page);
    await expect(modal).not.toBeVisible();

    // Re-open the modal to check the settings
    await editUserTrigger(userRowInSettingsUsersTable(page, username)).click();
    await expect(page.locator('div[aria-label="user-edit-prompt"]')).toBeVisible();

    for (const settingName of settingsToToggle) {
        const checkbox = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).toBeChecked();
    }

    // Delete the user
    await modal.locator('button[aria-label="Delete User"]').click();
    const genericModal = page.locator('div[aria-label="generic-prompt"]');
    await expect(genericModal).toBeVisible();
    await genericModal.locator('button[aria-label="Delete"]').click();
    await confirmActorPasswordPrompt(page);

    // After deletion, we should be back on the settings page.
    await expect(userRowInSettingsUsersTable(page, username)).not.toBeVisible();
    // usersApi.create/update/remove try without X-Password first; server returns 401, then the UI retries (201/204/200).
    checkForErrors(0, 3);
})

test("two factor auth check", async ({ page, checkForErrors }) => {
    // go to settings
    await page.goto("/settings");
    await expect(page).toHaveURL(/\/settings/);
    await page.locator('#users-sidebar').click();
    // click the edit button for testuser
    const userRow = userRowInSettingsUsersTable(page, "admin");
    await editUserTrigger(userRow).click();
    await expect(page.locator('div[aria-label="user-edit-prompt"]')).toBeVisible();

    const modal = page.locator('div[aria-label="user-edit-prompt"]');

    // Toggle the two factor authentication switch
    const twoFactorCheckbox = modal.locator('.toggle-container:has-text("Two-Factor Authentication") input[type="checkbox"]');
    const twoFactorToggle = modal.locator('.toggle-container:has-text("Two-Factor Authentication") label.switch');
    // Toggle by clicking the label (checkbox is hidden).
    await twoFactorToggle.click();
    // Verify it changed state
    await expect(twoFactorCheckbox).toBeChecked();
    await modal.locator('button[aria-label="Generate Code"]').click();

    const passwordModal = page.locator(
        'div[aria-label="password-prompt"]:not(.prompt-behind)'
    );
    // Must match the admin password used in tests/playwright/global-setup.ts (not testuser passwords).
    await passwordModal.locator('input').fill('admin');
    await passwordModal.locator('button[aria-label="Confirm"]').click();

    const totpModal = page.locator('div[aria-label="totp-prompt"]');
    // check for the otp url
    await expect(totpModal.locator('p[aria-label="otp-url"]')).toBeVisible();

    // check that the otp-url is not empty
    const otpUrl = await totpModal.locator('p[aria-label="otp-url"]').textContent();
    expect(otpUrl).not.toBe("");
    checkForErrors();
});

test.describe("User Settings Persistence", () => {
    const username = "testuser1";
    test.beforeEach(async ({ page }) => {
        await page.goto("/settings");
        await page.locator('#users-sidebar').click();
    });

    async function checkTogglePersistence(page: Page, settingName: string) {
        const userRow = userRowInSettingsUsersTable(page, username);
        const modal = page.locator('div[aria-label="user-edit-prompt"]');

        // --- Open modal and check initial state (should be OFF) ---
        // Debug: Check if the user row exists
        await expect(userRow).toBeVisible({ timeout: 5000 });
        // Debug: Take a screenshot before clicking
        await page.screenshot({ path: `debug-before-click-${settingName.replace(/\s+/g, '-')}.png` });
        // Open edit modal (settings table edit control — role button, aria from general.edit).
        await editUserTrigger(userRow).click();

        await expect(modal).toBeVisible();
        const checkbox = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked();

        // --- Toggle ON and save ---
        const toggleSwitch = modal.locator(".toggle-container", { hasText: settingName }).locator("label.switch");
        await toggleSwitch.click();
        await modal.locator('button[aria-label="Save"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(modal).not.toBeVisible();

        // --- Re-open and check persisted state (should be ON) ---
        await editUserTrigger(userRow).click();
        await expect(modal).toBeVisible();
        const checkboxOn = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkboxOn).toBeChecked();

        // --- Toggle OFF to restore state and save ---
        const toggleSwitchOn = modal.locator(".toggle-container", { hasText: settingName }).locator("label.switch");
        await toggleSwitchOn.click();
        await modal.locator('button[aria-label="Save"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(modal).not.toBeVisible();

        // --- Re-open and check state is restored (should be OFF) ---
        await editUserTrigger(userRow).click();
        await expect(modal).toBeVisible();
        const checkboxOff = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkboxOff).not.toBeChecked();

        await modal.locator('button[aria-label="Cancel"]').click();
    }

    test('should persist "Prevent the user from changing the password" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Prevent the user from changing the password");
    });

    test('should persist "Administrator" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Administrator");
    });

    test('should persist "Edit files" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Edit files");
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
        await editUserTrigger(userRow).click();
        const modal = page.locator('div[aria-label="user-edit-prompt"]');
        await expect(modal).toBeVisible();

        const loginMethodSelector = modal.locator("#loginMethod");
        await expect(loginMethodSelector).toHaveValue("password");
        checkForErrors();
    });
});