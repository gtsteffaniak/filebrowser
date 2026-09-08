import { expect, test } from "../test-setup";

// Optional fixture names allow the same scenario against a standalone preview.
const source = process.env.FOLDER_DESCRIPTION_SOURCE || "exclude";
const folder = process.env.FOLDER_DESCRIPTION_FOLDER || "myfolder";
const plainFolder = process.env.FOLDER_DESCRIPTION_PLAIN_FOLDER || "folder#hash";
const blankFolder = process.env.FOLDER_DESCRIPTION_BLANK_FOLDER || "files";
const description = process.env.FOLDER_DESCRIPTION_TEXT || "Project working files";

test("folder descriptions render as text and preserve navigation", async ({ page, checkForErrors }, testInfo) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`/files/${encodeURIComponent(source)}/`);
  const listing = page.locator(".listing-items");
  await expect(listing).toBeVisible();
  for (let attempt = 0; attempt < 5; attempt++) {
    const classes = await listing.getAttribute("class");
    if (/\b(list|compact)\b/.test(classes || "")) break;
    await page.getByRole("button", { name: "Switch view", exact: true }).click();
    await expect(listing).not.toHaveClass(classes || "");
  }
  const row = page.getByRole("button", { name: folder, exact: true });
  const cell = row.locator(".folder-description");
  await expect(cell).toHaveText(description);
  await expect(page.getByRole("button", { name: blankFolder, exact: true }).locator(".folder-description")).toBeEmpty();
  const plain = page.getByRole("button", { name: plainFolder, exact: true }).locator(".folder-description");
  await expect(plain).toHaveText("<img src=x onerror=alert(1)> & plain text");
  await expect(plain.locator("img")).toHaveCount(0);
  await page.mouse.move(0, 0);
  const header = await page.locator(".listing-item-header .folder-description").boundingBox();
  const value = await cell.boundingBox();
  expect(header).not.toBeNull(); expect(value).not.toBeNull();
  expect(Math.abs(header!.x - value!.x)).toBeLessThan(3);
  await page.screenshot({ path: testInfo.outputPath("folder-descriptions.png") });
  await cell.click(); await expect(row).toHaveAttribute("aria-selected", "true");
  await cell.dblclick(); await expect(page).toHaveURL(new RegExp(`/${encodeURIComponent(folder)}/?$`));
  await expect(page.locator(".listing-item .folder-description").filter({ hasText: description })).toHaveCount(0);
  checkForErrors();
});
