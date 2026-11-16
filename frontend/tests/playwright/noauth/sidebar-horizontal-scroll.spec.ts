import type { Locator, Page } from "@playwright/test";
import { expect, test } from "../test-setup.ts";

/*
 * Regression test for issue #3351.
 *
 * Defect: when the folder tree in the sidebar overflows horizontally, the
 * source path selector (rendered inside the same scroll container,
 * `.sidebar-links-content`) moves with the tree instead of staying fixed.
 *
 * The no-auth configuration already provides two enabled sources (`exclude`
 * and `include`), so the path selector renders. The nested fixture is created
 * through the application's own API before each test and removed afterwards.
 */

// Enabled source from the no-auth configuration (see _docker/src/noauth/backend/config.yaml).
const SOURCE = "exclude";

// Fixture location inside the source. The exact name is only used by this test.
const FIXTURE_ROOT_NAME = "sidebar-scroll-3351";
const FIXTURE_ROOT = `/${FIXTURE_ROOT_NAME}/`;
const LEVEL_NAMES = [
  "level-1-sidebar-width-overflow-probe-directory",
  "level-2-nested-directory-with-an-extremely-long-name",
  "level-3-deep-directory-name-that-keeps-going-for-width",
  "level-4-directory-used-to-widen-the-tree-past-sidebar",
  "level-5-directory-with-a-very-long-name-to-force-scroll",
  "level-6-directory-name-long-enough-to-overflow-content",
  "level-7-leaf-directory-with-the-longest-name-in-tree-x",
];
const DEEPEST_LEVEL_NAME = LEVEL_NAMES[LEVEL_NAMES.length - 1];
const DEEP_PATH = `${FIXTURE_ROOT}${LEVEL_NAMES.join("/")}/`;

// API paths are absolute, so they resolve under the configured baseURL (the app is served under /files/).
const RESOURCES_API = "/files/api/resources";

const MIN_OVERFLOW_PX = 100;
const MIN_TREE_SCROLL_PX = 50;
const STATIONARY_TOLERANCE_PX = 1;
const MIN_VERTICAL_SCROLL_PX = 50;

/** Removes the fixture directory tree. A missing fixture (404) is not an error. */
async function deleteFixture(page: Page): Promise<void> {
  const response = await page.request.delete(RESOURCES_API, {
    params: { path: FIXTURE_ROOT, source: SOURCE },
  });
  if (response.status() !== 404 && !response.ok()) {
    throw new Error(`could not delete fixture ${FIXTURE_ROOT}: HTTP ${response.status()}`);
  }
}

/** Creates the nested directories one level at a time, parents first. */
async function createFixture(page: Page): Promise<void> {
  let path = "/";
  for (const name of [FIXTURE_ROOT_NAME, ...LEVEL_NAMES]) {
    path = `${path}${name}/`;
    const response = await page.request.post(RESOURCES_API, {
      params: { path, source: SOURCE, isDir: "true" },
    });
    if (!response.ok()) {
      throw new Error(`could not create fixture directory ${path}: HTTP ${response.status()}`);
    }
  }
}

async function requireBox(locator: Locator, name: string) {
  const box = await locator.boundingBox();
  if (!box) {
    throw new Error(`${name} has no bounding box (not rendered or not visible)`);
  }
  return box;
}

/**
 * Waits until the sidebar's scroll position and a tree row's position stop changing.
 * The sidebar scrolls on its own after the folder tree first renders (it brings the current
 * folder into view), so baselines must be taken after that movement ends.
 */
async function waitForSidebarToSettle(content: Locator, row: Locator) {
  let previous: string | null = null;
  await expect
    .poll(
      async () => {
        const box = await requireBox(row, "deepest tree row");
        const scrollLeft = await content.evaluate((el) => el.scrollLeft);
        const current = `${scrollLeft}|${box.x}`;
        const settled = current === previous;
        previous = current;
        return settled;
      },
      { message: "sidebar scroll position and tree row should settle before measuring" },
    )
    .toBe(true);
}

// Viewport used to reproduce the overflow (sidebar width 288px at this size).
test.use({ viewport: { width: 1400, height: 900 } });

test.beforeEach(async ({ page }) => {
  // Remove any fixture left behind by an interrupted run, then create a fresh one.
  await deleteFixture(page);
  await createFixture(page);
});

test.afterEach(async ({ page }) => {
  await deleteFixture(page);
});

test("sidebar path selector stays stationary while the folder tree scrolls horizontally", async ({ page, checkForErrors }) => {
  const sidebarContent = page.locator(".sidebar-links-content");
  const pathSelector = page.locator(".sidebar-source-switcher");
  const homeButton = page.getByLabel("Navigate Home");
  const fileTree = page.locator(".file-tree-container");
  const deepestRow = page.locator(".tree-node", { hasText: DEEPEST_LEVEL_NAME });
  const sidebar = page.locator(".sidebar-links");
  const modeToggle = page.locator(".sidebar-mode-toggle");
  const editButton = page.locator('.sidebar-links-header i[aria-label="Edit Links"]');
  const levelOneRow = page.locator(".tree-node", { hasText: LEVEL_NAMES[0] });
  const levelTwoRow = page.locator(".tree-node", { hasText: LEVEL_NAMES[1] });

  await page.goto(`/files/${SOURCE}${DEEP_PATH}`);

  // The sidebar opens in links mode; the folder tree renders only in navigation mode.
  await page.locator(".sidebar-mode-toggle label.switch").click();
  await expect(fileTree).toBeVisible();
  await expect(deepestRow).toBeVisible();
  await waitForSidebarToSettle(sidebarContent, deepestRow);

  // Two enabled sources must render the path selector. Fail if it is absent.
  await expect(pathSelector, "path selector must render with two enabled sources").toBeVisible();

  // Overflow precondition: the scroll container that holds the tree must overflow by >= 100px.
  const overflowX = await sidebarContent.evaluate((el) => getComputedStyle(el).overflowX);
  expect(overflowX, ".sidebar-links-content must be a horizontal scroll container").toMatch(/auto|scroll/);
  const overflowPx = await sidebarContent.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflowPx, "horizontal overflow precondition").toBeGreaterThanOrEqual(MIN_OVERFLOW_PX);

  // Deterministic start: measure relative to the current scrollLeft, which may be nonzero.
  const scrollLeftBefore = await sidebarContent.evaluate((el) => el.scrollLeft);
  const remainingScrollPx = await sidebarContent.evaluate((el) => el.scrollWidth - el.clientWidth - el.scrollLeft);
  expect(remainingScrollPx, "container must have room to scroll at least 50px from its starting position")
    .toBeGreaterThanOrEqual(MIN_TREE_SCROLL_PX);

  const homeBefore = await requireBox(homeButton, "home button");
  const pathBefore = await requireBox(pathSelector, "path selector");
  const treeRowBefore = await requireBox(deepestRow, "deepest tree row");
  const modeToggleBefore = await requireBox(modeToggle, "mode toggle");
  const editButtonBefore = await requireBox(editButton, "edit links button");

  // Scroll the actual overflowing container horizontally with a wheel gesture over its content.
  const contentBox = await requireBox(sidebarContent, ".sidebar-links-content");
  await page.mouse.move(contentBox.x + contentBox.width / 2, contentBox.y + contentBox.height / 2);
  await page.mouse.wheel(200, 0);

  await expect
    .poll(() => sidebarContent.evaluate((el) => el.scrollLeft), { message: "container should scroll horizontally" })
    .toBeGreaterThanOrEqual(scrollLeftBefore + MIN_TREE_SCROLL_PX);

  // The folder tree itself must move horizontally.
  const treeRowAfter = await requireBox(deepestRow, "deepest tree row");
  const treeMovedLeftPx = treeRowBefore.x - treeRowAfter.x;
  expect(treeMovedLeftPx, "folder tree rows must move left by at least 50px").toBeGreaterThanOrEqual(MIN_TREE_SCROLL_PX);

  // Preservation check: the home button must stay put while the tree scrolls.
  const homeAfter = await requireBox(homeButton, "home button");
  expect(Math.abs(homeAfter.x - homeBefore.x), "home button must not move").toBeLessThanOrEqual(STATIONARY_TOLERANCE_PX);

  // Regression assertion: the path selector must stay stationary. Fails on the pre-fix code.
  const pathAfter = await requireBox(pathSelector, "path selector");
  expect(
    Math.abs(pathAfter.x - pathBefore.x),
    "path selector moved horizontally with the folder tree (issue #3351)",
  ).toBeLessThanOrEqual(STATIONARY_TOLERANCE_PX);

  // Mode toggle and edit links button must also stay put.
  const modeToggleAfter = await requireBox(modeToggle, "mode toggle");
  expect(Math.abs(modeToggleAfter.x - modeToggleBefore.x), "mode toggle must not move").toBeLessThanOrEqual(
    STATIONARY_TOLERANCE_PX,
  );
  const editButtonAfter = await requireBox(editButton, "edit links button");
  expect(Math.abs(editButtonAfter.x - editButtonBefore.x), "edit links button must not move").toBeLessThanOrEqual(
    STATIONARY_TOLERANCE_PX,
  );

  // Header controls and the path selector must stay inside the sidebar's visible bounds.
  const sidebarAfter = await requireBox(sidebar, ".sidebar-links");
  const controlsAfter = [
    ["home button", homeAfter],
    ["mode toggle", modeToggleAfter],
    ["edit links button", editButtonAfter],
    ["path selector", pathAfter],
  ] as const;
  for (const [name, box] of controlsAfter) {
    expect(box.x, `${name} must not be clipped on the left`).toBeGreaterThanOrEqual(
      sidebarAfter.x - STATIONARY_TOLERANCE_PX,
    );
    expect(box.x + box.width, `${name} must not be clipped on the right`).toBeLessThanOrEqual(
      sidebarAfter.x + sidebarAfter.width + STATIONARY_TOLERANCE_PX,
    );
  }

  // The active folder keeps its highlight after horizontal scrolling.
  await expect(deepestRow, "active folder keeps the current-item class").toHaveClass(/current-item/);

  // Vertical scrolling still works on the tall tree.
  const verticalOverflowPx = await sidebarContent.evaluate((el) => el.scrollHeight - el.clientHeight);
  expect(verticalOverflowPx, "tree must be tall enough to scroll vertically").toBeGreaterThanOrEqual(
    MIN_VERTICAL_SCROLL_PX,
  );
  const scrollTopBefore = await sidebarContent.evaluate((el) => el.scrollTop);
  await page.mouse.wheel(0, 200);
  await expect
    .poll(() => sidebarContent.evaluate((el) => el.scrollTop), { message: "sidebar should scroll vertically" })
    .toBeGreaterThanOrEqual(scrollTopBefore + MIN_VERTICAL_SCROLL_PX);

  // Expand and collapse still work: collapsing a folder removes its child row, expanding restores it.
  const levelOneExpandIcon = levelOneRow.locator(".expand-icon");
  await expect(levelTwoRow, "child row is visible before collapsing").toHaveCount(1);
  await levelOneExpandIcon.click();
  await expect(levelTwoRow, "collapsing a folder hides its children").toHaveCount(0);
  await levelOneExpandIcon.click();
  await expect(levelTwoRow, "expanding a folder shows its children again").toHaveCount(1);

  checkForErrors();
});
