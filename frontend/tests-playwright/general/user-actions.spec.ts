import { test, expect } from "../test-setup";
import { Page } from "@playwright/test";

test('create, check settings, and delete testuser2', async ({
    page,
    checkForErrors,
}) => {
    await page.goto('/settings')
    await expect(page).toHaveTitle("Graham's Filebrowser - Settings")
    await page.locator('button[aria-label="Add New User"]').click()
    await page.locator('#username').fill('testuser2')
    await page.locator('input[aria-label="Password1"]').fill('testpassword')
    await page.locator('input[aria-label="Password2"]').fill('testpass')
    // check that the invalid-field class is added properly
    await expect(page.locator('input[aria-label="Password2"]')).toHaveClass(
        'input input--block form-form invalid-form'
    )
    await page.locator('input[aria-label="Password2"]').fill('testpassword')

    // Just create the user first
    await page.locator('input[aria-label="Save User"]').click();

    // We should be back on the settings page
    await expect(page.locator('tr.item', { hasText: 'testuser2' })).toBeVisible();

    // Now, click the edit button for testuser2
    const userRow = page.locator('tr.item', { hasText: 'testuser2' })
    const editLink = await userRow
        .locator('td[aria-label="Edit User"] a')
        .getAttribute('href')
    await page.goto(editLink!)

    // Now on the edit page, toggle the settings
    const settingsToToggle = [
        "Administrator",
        "Prevent the user from changing the password",
        "Edit files",
        "Share files",
        "Create and manage long-live API keys",
        "Enable real-time connections and updates",
    ];

    for (const settingName of settingsToToggle) {
        const toggleContainer = page.locator(".toggle-container", { hasText: settingName });
        const toggleSwitch = toggleContainer.locator("label.switch");
        await toggleSwitch.click();
    }

    // Save the updated settings
    await page.locator('input[aria-label="Save User"]').click();

    // The app might show a notification and stay on the page. Let's reload to ensure we have the latest state.
    await page.reload();


    for (const settingName of settingsToToggle) {
        const checkbox = page.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).toBeChecked();
    }

    // Delete the user
    await page.locator('button[aria-label="Delete User"]').click();
    await page.locator('button[aria-label="Confirm Delete"]').click();

    // After deletion, we should be back on the settings page. A reload should confirm the user is gone.
    await page.reload();
    await expect(page.locator('tr.item', { hasText: 'testuser2' })).not.toBeVisible();
    checkForErrors()
})

test("two factor auth check", async ({ page, checkForErrors, context }) => {
    // go to settings
    await page.goto("/settings");
    await expect(page).toHaveURL(/\/settings/);
    // click the edit button for testuser
    const userRow = page.locator('tr.item', { hasText: 'admin' })
    const editLink = await userRow
        .locator('td[aria-label="Edit User"] a')
        .getAttribute('href')
    await page.goto(editLink!)

    // Toggle the two factor authentication switch
    const twoFactorCheckbox = page.locator('.toggle-container:has-text("Two-Factor Authentication") input[type="checkbox"]');
    const twoFactorToggle = page.locator('.toggle-container:has-text("Two-Factor Authentication") label.switch');
    // Check if it's currently enabled
    const isChecked = await twoFactorCheckbox.isChecked();
    // Toggle it by clicking the label (since checkbox is hidden)
    await twoFactorToggle.click();
    // Verify it changed state
    await expect(twoFactorCheckbox).toBeChecked();
    await page.locator('button[aria-label="Generate Code"]').click();
    // check for the otp url
    await expect(page.locator('p[aria-label="otp-url"]')).toBeVisible();

    // check that the otp-url is not empty
    const otpUrl = await page.locator('p[aria-label="otp-url"]').textContent();
    expect(otpUrl).not.toBe("");
    checkForErrors();
});

test.describe("User Settings Persistence", () => {
    test.beforeEach(async ({ page }) => {
        await page.goto("/settings");
        const userRow = page.locator("tr.item", { hasText: "testuser1" });
        const editLink = await userRow
            .locator('td[aria-label="Edit User"] a')
            .getAttribute("href");
        await page.goto(editLink!);
    });

    async function toggleSettingAndSave(page: Page, settingName: string) {
        const toggleContainer = page.locator(".toggle-container", { hasText: settingName });
        const toggleSwitch = toggleContainer.locator("label.switch");
        await toggleSwitch.click();
        await page.locator('input[aria-label="Save User"]').click();
        await page.reload();
    }

    test('should persist "Prevent the user from changing the password" setting', async ({ page }) => {
        const settingName = "Prevent the user from changing the password";
        const checkbox = page.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked();
        await toggleSettingAndSave(page, settingName);
        await expect(checkbox).toBeChecked();
    });

    test('should persist "Administrator" setting', async ({ page }) => {
        const settingName = "Administrator";
        const checkbox = page.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked(); // Admin user should have this on
        await toggleSettingAndSave(page, settingName);
        await expect(checkbox).toBeChecked();
    });

    test('should persist "Edit files" setting', async ({ page }) => {
        const settingName = "Edit files";
        const checkbox = page.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked();
        await toggleSettingAndSave(page, settingName);
        await expect(checkbox).toBeChecked();
        await toggleSettingAndSave(page, settingName); // Revert
    });

    test('should persist "Share files" setting', async ({ page }) => {
        const settingName = "Share files";
        const checkbox = page.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked();
        await toggleSettingAndSave(page, settingName);
        await expect(checkbox).toBeChecked();
    });

    test('should persist "Create and manage long-live API keys" setting', async ({ page }) => {
        const settingName = "Create and manage long-live API keys";
        const checkbox = page.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked();
        await toggleSettingAndSave(page, settingName);
        await expect(checkbox).toBeChecked();
    });

    test('should persist "Enable real-time connections and updates" setting', async ({ page }) => {
        const settingName = "Enable real-time connections and updates";
        const checkbox = page.locator(`.toggle-container:has-text("${settingName}") input[type="checkbox"]`);
        await expect(checkbox).not.toBeChecked();
        await toggleSettingAndSave(page, settingName);
        await expect(checkbox).toBeChecked();
    });

    test('should persist "allowed login method" setting', async ({ page }) => {
        const loginMethodSelector = page.locator("#loginMethod");
        await expect(loginMethodSelector).toHaveValue("password");

        await loginMethodSelector.selectOption({ label: "Proxy" });
        await page.locator('input[aria-label="Save User"]').click();
        await page.reload();

        await expect(page.locator("#loginMethod")).toHaveValue("proxy");
    });
});