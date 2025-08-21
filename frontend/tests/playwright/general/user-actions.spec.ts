import { test, expect } from "../test-setup";
import { Page } from "@playwright/test";

test('create, check settings, and delete testuser2', async ({
    page,
    checkForErrors,
}) => {
    await page.goto('/settings')
    await expect(page).toHaveTitle("Graham's Filebrowser - Settings")
    await page.locator('#users-sidebar').click();
    await page.locator('button[aria-label="Add New User"]').click()
    await page.locator('#username').fill('testuser2')
    await page.locator('input[aria-label="Password1"]').fill('testpassword')
    await page.locator('input[aria-label="Password2"]').fill('testpass')
    // check that the invalid-field class is added properly
    await expect(page.locator('input[aria-label="Password2"]')).toHaveClass(
        'input form-form form-invalid'
    )
    await page.locator('input[aria-label="Password2"]').fill('testpassword')

    // Just create the user first
    await page.locator('input[aria-label="Save User"]').click();
    await page.waitForResponse((resp) => resp.url().includes('/api/users') && resp.status() === 201)

    // We should be back on the settings page
    await expect(page.locator('tr.item', { hasText: 'testuser2' })).toBeVisible();

    // Now, click the edit button for testuser2
    await page.locator('tr.item', { hasText: 'testuser2' }).getByLabel('Edit User').click();

    // Now on the edit page, toggle the settings
    await expect(page.locator('.card.floating')).toBeVisible();
    const modal = page.locator('.card.floating');

    const settingsToToggle = [
        "Administrator",
        "Prevent the user from changing the password",
        "Edit files",
        "Share files",
        "Create and manage long-live API keys",
        "Enable real-time connections and updates",
    ];

    for (const settingName of settingsToToggle) {
        const toggleContainer = modal.locator(".toggle-container", { hasText: settingName });
        const toggleSwitch = toggleContainer.locator("label.switch");
        await toggleSwitch.click();
    }

    // Save the updated settings
    await modal.locator('input[aria-label="Save User"]').click();
    await expect(modal).not.toBeVisible();

    // Re-open the modal to check the settings
    await page.locator('tr.item', { hasText: 'testuser2' }).getByLabel('Edit User').click();
    await expect(page.locator('.card.floating')).toBeVisible();

    for (const settingName of settingsToToggle) {
        const checkbox = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).toBeChecked();
    }

    // Delete the user
    await modal.locator('button[aria-label="Delete User"]').click();
    await page.locator('button[aria-label="Confirm Delete"]').click();

    // After deletion, we should be back on the settings page.
    await expect(page.locator('tr.item', { hasText: 'testuser2' })).not.toBeVisible();
    checkForErrors()
})

test("two factor auth check", async ({ page, checkForErrors, context }) => {
    // go to settings
    await page.goto("/settings");
    await expect(page).toHaveURL(/\/settings/);
    await page.locator('#users-sidebar').click();
    // click the edit button for testuser
    const userRow = page.locator('tr.item', { hasText: 'admin' })
    await userRow.getByLabel('Edit User').click();
    await expect(page.locator('.card.floating')).toBeVisible();

    const modal = page.locator('.card.floating');

    // Toggle the two factor authentication switch
    const twoFactorCheckbox = modal.locator('.toggle-container:has-text("Two-Factor Authentication") input[type="checkbox"]');
    const twoFactorToggle = modal.locator('.toggle-container:has-text("Two-Factor Authentication") label.switch');
    // Check if it's currently enabled
    const isChecked = await twoFactorCheckbox.isChecked();
    // Toggle it by clicking the label (since checkbox is hidden)
    await twoFactorToggle.click();
    // Verify it changed state
    await expect(twoFactorCheckbox).toBeChecked();
    await modal.locator('button[aria-label="Generate Code"]').click();
    // check for the otp url
    await expect(modal.locator('p[aria-label="otp-url"]')).toBeVisible();

    // check that the otp-url is not empty
    const otpUrl = await modal.locator('p[aria-label="otp-url"]').textContent();
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
        const userRow = page.locator("tr.item", { hasText: username });
        const modal = page.locator('.card.floating');

        // --- Open modal and check initial state (should be OFF) ---
        // Now, click the edit button for testuser1
        await userRow.getByLabel('Edit User').click()

        await expect(modal).toBeVisible();
        const checkbox = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked();

        // --- Toggle ON and save ---
        const toggleSwitch = modal.locator(".toggle-container", { hasText: settingName }).locator("label.switch");
        await toggleSwitch.click();
        await modal.locator('input[aria-label="Save User"]').click();
        await expect(modal).not.toBeVisible();

        // --- Re-open and check persisted state (should be ON) ---
        await userRow.getByLabel('Edit User').click();
        await expect(modal).toBeVisible();
        const checkboxOn = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkboxOn).toBeChecked();

        // --- Toggle OFF to restore state and save ---
        const toggleSwitchOn = modal.locator(".toggle-container", { hasText: settingName }).locator("label.switch");
        await toggleSwitchOn.click();
        await modal.locator('input[aria-label="Save User"]').click();
        await expect(modal).not.toBeVisible();

        // --- Re-open and check state is restored (should be OFF) ---
        await userRow.getByLabel('Edit User').click();
        await expect(modal).toBeVisible();
        const checkboxOff = modal.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkboxOff).not.toBeChecked();

        // --- Close modal to finish ---
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

    test('should persist "Create and manage long-live API keys" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Create and manage long-live API keys");
    });

    test('should persist "Enable real-time connections and updates" setting', async ({ page }) => {
        await checkTogglePersistence(page, "Enable real-time connections and updates");
    });

    test('should persist "allowed login method" setting', async ({ page }) => {
        const userRow = page.locator("tr.item", { hasText: username });
        await userRow.getByLabel('Edit User').click();
        const modal = page.locator('.card.floating');
        await expect(modal).toBeVisible();

        const loginMethodSelector = modal.locator("#loginMethod");
        await expect(loginMethodSelector).toHaveValue("password");

        await loginMethodSelector.selectOption({ label: "Proxy" });
        await modal.locator('input[aria-label="Save User"]').click();
        await expect(modal).not.toBeVisible();

        await userRow.getByLabel('Edit User').click();
        await expect(modal).toBeVisible();

        await expect(modal.locator("#loginMethod")).toHaveValue("proxy");

        // Revert change
        const loginMethodSelector2 = modal.locator("#loginMethod");
        await loginMethodSelector2.selectOption({ label: "Password" });
        await modal.locator('input[aria-label="Save User"]').click();
        await expect(modal).not.toBeVisible();
    });
});