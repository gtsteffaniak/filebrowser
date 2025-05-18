import { test, expect } from '../test-setup'

test('create and delete testuser', async ({
  page,
  checkForErrors,
  context
}) => {
  await page.goto('/settings')
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

  // click the edit button for testuser
  const userRow = page.locator('tr.item', { hasText: 'testuser' })
  const editLink = await userRow
    .locator('td[aria-label="Edit User"] a')
    .getAttribute('href')
  await page.goto(editLink!)
  checkForErrors()
})
