import type { Locator, Page } from "@playwright/test";
import { expect, test } from "../test-setup";

const LISTING_TITLE = "Graham's Filebrowser - Files - playwright-files";
const DOUBLE_CLICK_WINDOW_MS = 600;

const FILE_A = "copyme.txt";
const FILE_B = "file.tar.gz";
const FOLDER_C = "myfolder";
const FILE_D = "1file1.txt";

async function openListing(page: Page) {
  await page.goto("/files/");
  await expect(page).toHaveTitle(LISTING_TITLE);
  await listingItem(page, FILE_A).waitFor({ state: "visible" });
  await listingItem(page, FILE_B).waitFor({ state: "visible" });
  await listingItem(page, FOLDER_C).waitFor({ state: "visible" });
  await listingItem(page, FILE_D).waitFor({ state: "visible" });
}

function listingItem(page: Page, name: string) {
  return page.locator(`.listing-items .listing-item[aria-label="${name}"]`);
}

function nestedLabel(page: Page, name: string) {
  return listingItem(page, name).locator(".name span");
}

function selectedItems(page: Page) {
  return page.locator('.listing-items .listing-item[aria-selected="true"]');
}

function selectedCount(page: Page) {
  return page.locator("#status-bar .status-info .button");
}

async function expectSelected(page: Page, names: string[]) {
  await expect(selectedItems(page)).toHaveCount(names.length);
  if (names.length > 0) {
    await expect(selectedCount(page)).toHaveText(String(names.length));
  }
  for (const name of names) {
    await expect(listingItem(page, name)).toHaveAttribute("aria-selected", "true");
  }
  await expect(page).toHaveTitle(LISTING_TITLE);
  await expect(page).toHaveURL(/\/files\//);
}

async function dispatchBubblingClick(
  target: Locator,
  modifiers: { ctrlKey?: boolean; metaKey?: boolean; shiftKey?: boolean } = {},
) {
  await target.evaluate((el, mods) => {
    el.dispatchEvent(
      new MouseEvent("click", {
        bubbles: true,
        cancelable: true,
        button: 0,
        view: window,
        ctrlKey: Boolean(mods.ctrlKey),
        metaKey: Boolean(mods.metaKey),
        shiftKey: Boolean(mods.shiftKey),
      }),
    );
  }, modifiers);
}

async function waitForDoubleClickWindow(page: Page) {
  await page.waitForTimeout(DOUBLE_CLICK_WINDOW_MS);
}

for (const modifier of [
  { name: "ctrlKey", init: { ctrlKey: true } },
  { name: "metaKey", init: { metaKey: true } },
] as const) {
  test(`preserves ${modifier.name} selection when a nested label click bubbles without keydown`, async ({
    page,
    checkForErrors,
  }) => {
    await openListing(page);

    await nestedLabel(page, FILE_A).click();
    await expectSelected(page, [FILE_A]);

    await dispatchBubblingClick(nestedLabel(page, FILE_B), modifier.init);
    await expectSelected(page, [FILE_A, FILE_B]);

    await dispatchBubblingClick(nestedLabel(page, FOLDER_C), modifier.init);
    await expectSelected(page, [FILE_A, FILE_B, FOLDER_C]);

    checkForErrors();
  });
}

test("native Control hold selects distinct files and folders from nested and root targets", async ({
  page,
  checkForErrors,
}) => {
  await openListing(page);
  await page.keyboard.down("Control");
  try {
    await nestedLabel(page, FILE_A).click({ noWaitAfter: true });
    await expect(listingItem(page, FILE_A)).toHaveAttribute("aria-selected", "true");
    await expect(listingItem(page, FOLDER_C)).toHaveAttribute("aria-selected", "false");
    await expect(listingItem(page, FILE_D)).toHaveAttribute("aria-selected", "false");
    await expectSelected(page, [FILE_A]);

    await listingItem(page, FOLDER_C).click({ noWaitAfter: true });
    await expect(listingItem(page, FILE_A)).toHaveAttribute("aria-selected", "true");
    await expect(listingItem(page, FOLDER_C)).toHaveAttribute("aria-selected", "true");
    await expect(listingItem(page, FILE_D)).toHaveAttribute("aria-selected", "false");
    await expectSelected(page, [FILE_A, FOLDER_C]);

    await nestedLabel(page, FILE_D).click({ noWaitAfter: true });
    await expect(listingItem(page, FILE_A)).toHaveAttribute("aria-selected", "true");
    await expect(listingItem(page, FOLDER_C)).toHaveAttribute("aria-selected", "true");
    await expect(listingItem(page, FILE_D)).toHaveAttribute("aria-selected", "true");
    await expectSelected(page, [FILE_A, FOLDER_C, FILE_D]);
  } finally {
    await page.keyboard.up("Control");
  }
  checkForErrors();
});

test("holding Control before the listing mounts still allows additive selection", async ({
  page,
  checkForErrors,
}) => {
  await page.goto("/settings");
  await expect(page).toHaveTitle("Graham's Filebrowser - Settings");

  await page.keyboard.down("Control");
  try {
    await page.goto("/files/");
    await expect(page).toHaveTitle(LISTING_TITLE);
    await listingItem(page, FILE_A).waitFor({ state: "visible" });
    await listingItem(page, FILE_B).waitFor({ state: "visible" });

    await nestedLabel(page, FILE_A).click({ noWaitAfter: true });
    await expect(listingItem(page, FILE_A)).toHaveAttribute("aria-selected", "true");
    await expect(listingItem(page, FILE_B)).toHaveAttribute("aria-selected", "false");

    await nestedLabel(page, FILE_B).click({ noWaitAfter: true });
    await expectSelected(page, [FILE_A, FILE_B]);
  } finally {
    await page.keyboard.up("Control");
  }
  checkForErrors();
});

test("Ctrl-click deselects only the clicked item after the double-click window", async ({
  page,
  checkForErrors,
}) => {
  await openListing(page);

  await nestedLabel(page, FILE_A).click();
  await dispatchBubblingClick(nestedLabel(page, FILE_B), { ctrlKey: true });
  await dispatchBubblingClick(nestedLabel(page, FOLDER_C), { ctrlKey: true });
  await expectSelected(page, [FILE_A, FILE_B, FOLDER_C]);

  await waitForDoubleClickWindow(page);
  await dispatchBubblingClick(nestedLabel(page, FILE_B), { ctrlKey: true });
  await expect(listingItem(page, FILE_B)).toHaveAttribute("aria-selected", "false");
  await expectSelected(page, [FILE_A, FOLDER_C]);

  checkForErrors();
});

test("unmodified click replaces the selection and a background click clears it", async ({
  page,
  checkForErrors,
}) => {
  await openListing(page);

  await nestedLabel(page, FILE_A).click();
  await expectSelected(page, [FILE_A]);

  await nestedLabel(page, FILE_B).click();
  await expect(listingItem(page, FILE_A)).toHaveAttribute("aria-selected", "false");
  await expectSelected(page, [FILE_B]);

  await page.locator(".listing-items h2").first().click();
  await expect(selectedItems(page)).toHaveCount(0);
  await expect(page).toHaveTitle(LISTING_TITLE);
  await expect(page).toHaveURL(/\/files\//);

  checkForErrors();
});

test("releasing Control allows an ordinary background click to clear selection", async ({
  page,
  checkForErrors,
}) => {
  await openListing(page);
  await page.keyboard.down("Control");
  try {
    await nestedLabel(page, FILE_A).click({ noWaitAfter: true });
    await listingItem(page, FOLDER_C).click({ noWaitAfter: true });
    await expectSelected(page, [FILE_A, FOLDER_C]);
  } finally {
    await page.keyboard.up("Control");
  }

  await page.locator(".listing-items h2").first().click();
  await expect(selectedItems(page)).toHaveCount(0);
  await expect(page).toHaveTitle(LISTING_TITLE);

  checkForErrors();
});

test("Shift-click selects a range of files", async ({ page, checkForErrors }) => {
  await openListing(page);

  const fileItems = page.locator('div[aria-label="File Items"] .listing-item');
  await expect(fileItems.first()).toBeVisible();
  expect(await fileItems.count()).toBeGreaterThanOrEqual(3);

  const firstName = await fileItems.nth(0).getAttribute("aria-label");
  const secondName = await fileItems.nth(1).getAttribute("aria-label");
  const thirdName = await fileItems.nth(2).getAttribute("aria-label");
  expect(firstName).toBeTruthy();
  expect(secondName).toBeTruthy();
  expect(thirdName).toBeTruthy();
  if (!firstName || !secondName || !thirdName) {
    throw new Error("Expected at least three named file items");
  }

  await fileItems.nth(0).click();
  await expect(listingItem(page, firstName)).toHaveAttribute("aria-selected", "true");

  await fileItems.nth(2).click({ modifiers: ["Shift"] });
  await expectSelected(page, [firstName, secondName, thirdName]);

  checkForErrors();
});

test("Ctrl+A selects all listing items", async ({ page, checkForErrors }) => {
  await openListing(page);

  await nestedLabel(page, FILE_A).click();
  await expectSelected(page, [FILE_A]);

  await page.keyboard.press("Control+a");
  const allItems = page.locator(".listing-items .listing-item");
  await expect(selectedItems(page)).toHaveCount(await allItems.count());
  await expect(listingItem(page, FILE_A)).toHaveAttribute("aria-selected", "true");
  await expect(listingItem(page, FILE_B)).toHaveAttribute("aria-selected", "true");
  await expect(listingItem(page, FOLDER_C)).toHaveAttribute("aria-selected", "true");
  await expect(page).toHaveTitle(LISTING_TITLE);

  checkForErrors();
});
