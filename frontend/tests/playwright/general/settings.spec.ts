import { test, expect } from '../test-setup'

test('create and delete testuser', async ({
  page,
  checkForErrors,
  context
}) => {
  await page.goto('/settings#users-main')
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings")
  await page.locator('button[aria-label="Add New User"]').click()
  await page.locator('#username').fill('testuser')
  await page.locator('input[aria-label="Password1"]').fill('testpassword')
  await page.locator('input[aria-label="Password2"]').fill('testpass')
  // check that the invalid-field class is added properly
  await expect(page.locator('input[aria-label="Password2"]')).toHaveClass(
    'input input--block form-form invalid-form'
  )
  await page.locator('input[aria-label="Password2"]').fill('testpassword')
  await page.locator('input[aria-label="Save User"]').click()

  // Wait for the user row to appear
  const userRow = page.locator('tr.item', { hasText: 'testuser' })
  await expect(userRow).toBeVisible()

  // Wait for overlay to disappear (try a more generic selector)
  await page.waitForSelector('[class*=overlay]', { state: 'hidden', timeout: 5000 })

  // Wait for the edit button to be visible and enabled
  const editButton = userRow.locator('td[aria-label="Edit User"] .clickable')
  await expect(editButton).toBeVisible()
  await expect(editButton).toBeEnabled()

  // Now click
  await editButton.click()
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
