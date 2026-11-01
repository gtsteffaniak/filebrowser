import type { Locator, Page } from "@playwright/test";
import { expect, test } from "../test-setup.ts";

const TITLE = "Graham's Filebrowser - Files - ";

const advancedSearchUrl = (term: string) =>
  `/tools/advancedSearch?term=${encodeURIComponent(term)}&scope=${encodeURIComponent("playwright + files:/")}`;

const resultItems = (page: Page) =>
  page.locator(".advanced-search-listing-results .file-items a.listing-item");

const namesOf = (items: Locator) =>
  items.evaluateAll((els) => els.map((el) => el.getAttribute("aria-label") ?? ""));

test("navigate to the adv search tool and search something", async ({ page, checkForErrors }) => {
  await page.goto("/files/");
  await expect(page).toHaveTitle(`${TITLE}playwright-files`);

  await page.getByRole("link", { name: "Tools", exact: true }).click();
  await expect(page).toHaveURL(/\/tools\/?$/);
  await page.getByRole("link", { name: /Advanced Search/ }).click();
  await expect(page).toHaveURL(/\/tools\/advancedSearch/);
  await expect(page.locator(".advanced-search-toolbar")).toBeVisible();

  await page.locator('.advanced-search-toolbar input[type="search"]').fill("for testing");
  await page.locator("button.search-submit").click();

  await expect(resultItems(page)).toHaveCount(1);
  await expect(page.locator('a.listing-item[aria-label="for testing.md"]')).toBeVisible();
  await expect(page).toHaveURL(/term=for(%20|\+)testing/);
  checkForErrors();
});

test("adv search shows a message when nothing matches", async ({ page, checkForErrors }) => {
  await page.goto("/tools/advancedSearch");
  await page.locator('.advanced-search-toolbar input[type="search"]').fill("idk-no-existent-file-sfkodsñ");
  await page.locator("button.search-submit").click();

  await expect(page.locator(".empty-state-message")).toContainText("No matching files or folders.");
  await expect(resultItems(page)).toHaveCount(0);
  checkForErrors();
});

test("next/previous navigate and follow the sorting of the adv search results", async ({ page, checkForErrors }) => {
  await page.goto(advancedSearchUrl("jpg"));
  const items = resultItems(page);
  await expect(items.first()).toBeVisible();
  const before = await namesOf(items);
  // results come from more than one folder
  expect(before).toEqual(expect.arrayContaining(["gray-sample.jpg", "HAND_C.jpg"]));

  await page.locator(".advanced-search-listing-results .listing-item-header .name").click();
  await expect.poll(() => namesOf(items)).not.toEqual(before);
  const names = await namesOf(items);

  await items.nth(1).dblclick();
  await expect(page).toHaveTitle(`${TITLE}${names[1]}`);

  const next = page.locator('button[aria-label="Next"]');
  const previous = page.locator('button[aria-label="Previous"]');
  await next.waitFor({ state: "visible" });
  await next.click();
  await expect(page).toHaveTitle(`${TITLE}${names[2]}`);
  await previous.click();
  await expect(page).toHaveTitle(`${TITLE}${names[1]}`);
  await previous.click();
  await expect(page).toHaveTitle(`${TITLE}${names[0]}`);
  await expect(previous).toBeHidden();
  checkForErrors();
});

test("closing a result we opened returns and scrolls to it", async ({ page, checkForErrors }) => {
  await page.goto(advancedSearchUrl("jpg"));
  const items = resultItems(page);
  await expect(items.first()).toBeVisible();
  const before = await namesOf(items);
  await page.locator(".advanced-search-listing-results .listing-item-header .name").click();
  await expect.poll(() => namesOf(items)).not.toEqual(before);
  const names = await namesOf(items);
  const last = names[names.length - 1];
  const beforeLast = names[names.length - 2];

  await items.last().dblclick();
  await expect(page).toHaveTitle(`${TITLE}${last}`);
  await page.locator('button[aria-label="Previous"]').click();
  await expect(page).toHaveTitle(`${TITLE}${beforeLast}`);

  await page.locator('button[aria-label="Close"]').click();
  await expect(page).toHaveURL(/\/tools\/advancedSearch/);
  await expect.poll(() => namesOf(items)).toEqual(names);

  const returned = page.locator(".advanced-search-listing-results").locator(`a.listing-item[aria-label="${beforeLast}"]`);
  await expect(returned).toHaveAttribute("aria-selected", "true");
  await expect(returned).toBeInViewport();
  checkForErrors();
});
