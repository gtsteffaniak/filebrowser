import { expect, test } from "../test-setup";

test("check default sidebar links are added to sidebar", async ({ page, checkForErrors }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    // sidebar should have 4 items
    await expect(page.locator('.sidebar-links .inner-card').locator('a')).toHaveCount(4);

    // check items exist
    await page.locator('a[aria-label="playwright + files"]').waitFor({ state: 'visible' });
    await page.locator('a[aria-label="docker"]').waitFor({ state: 'visible' });
    checkForErrors();
});

test("check sidebar source links are formatted correctly", async ({ page, checkForErrors }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

    // Prefer accessible name (aria-label + visible text); scopes all assertions to this link.
    const pwFiles = page.getByRole("link", { name: "playwright + files", exact: true });

    await expect(pwFiles).toBeVisible();
    await expect(pwFiles).toHaveAttribute(
        "href",
        /playwright%20%2B%20files/,
    );
    await expect(pwFiles).toHaveClass(/source-button/);
    await expect(pwFiles).toHaveClass(/sidebar-link-button/);

    // Indexing / realtime status (Links.vue): svg.realtime-pulse.ready when source is ready
    await expect(pwFiles.locator("svg.realtime-pulse.ready")).toBeVisible();

    // ProgressBar.vue renders vue-simple-progress under .usage-info
    const progress = pwFiles.locator(".usage-info .vue-simple-progress");
    await expect(progress).toBeVisible();
    // At 0% fill the inner bar has no layout box; Playwright reports it hidden (esp. Firefox).
    await expect(progress.locator(".vue-simple-progress-bar").first()).toBeAttached();
    await expect(progress).toContainText(/\d/);
    await expect(progress).toContainText(/MB|GB|%/);

    checkForErrors();
});

test("check sidebar Tools link is formatted correctly", async ({ page, checkForErrors }) => {
    await page.goto("/files/");
    const tools = page.getByRole("link", { name: "Tools", exact: true });

    await expect(tools).toBeVisible();
    await expect(tools).toHaveAttribute("href", /\/tools\/?$/);
    await expect(tools).toHaveClass(/action/);
    await expect(tools).toHaveClass(/button/);
    await expect(tools).toHaveClass(/sidebar-link-button/);
    await expect(tools).not.toHaveClass("active");

    const container = tools.locator(".link-container");
    await expect(container).toBeVisible();
    await expect(container.locator("i.material-symbols.link-icon")).toHaveText("build");
    await expect(container.locator("span")).toHaveText("Tools");

    // Router path must match link.target "/tools" exactly (no trailing slash) or `active` is not applied.
    await page.goto("/tools");
    await expect(tools).toHaveClass(/active/);

    checkForErrors();
});

test("Edit Links opens customize prompt; Show tools in sidebar is on", async ({ page, checkForErrors }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

    await page.getByLabel("Edit Links").click();

    const prompt = page.locator('[aria-label="SidebarLinks-prompt"]');
    await expect(prompt).toBeVisible();
    await expect(prompt.locator(".prompt-title")).toHaveText("Customize Sidebar Links");
    await expect(
        prompt.getByText("Customize the links shown in your sidebar. Drag and drop to reorder."),
    ).toBeVisible();

    const showToolsToggle = prompt.locator(".toggle-container").filter({
        hasText: "Show tools in sidebar",
    });
    await expect(showToolsToggle.locator('input[type="checkbox"]')).toBeChecked();

    await prompt.getByRole("button", { name: "Close" }).click();
    await expect(prompt).not.toBeVisible();

    checkForErrors();
});

test("docker source link goes to /files/docker; home returns to playwright + files", async ({ page, checkForErrors }) => {
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

    const sidebarAnchors = page.locator(".sidebar-links .inner-card > a");
    await expect(sidebarAnchors.nth(1)).toHaveAttribute("aria-label", "docker");

    const dockerLink = page.getByRole("link", { name: "docker", exact: true });
    await dockerLink.click();
    await expect(page).toHaveURL(/\/files\/docker\/?$/);

    await page.getByLabel("Navigate Home").click();

    await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");
    await expect(
        page.getByRole("link", { name: "playwright + files", exact: true }),
    ).toHaveClass(/active/);

    checkForErrors();
});
