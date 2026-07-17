import type { Locator, Page } from "@playwright/test";
import { expect, test } from "../test-setup";

/**
 * Snapshot of the settings Playwright docker fixture after BoltDB → SQLite migration.
 * Tweak these objects if the intended post-migration state changes.
 */

const EXPECTED_ADMIN_SIDEBAR_LINKS = [
    "playwright + files",
    "docker",
    "access",
    "Tools",
] as const;

const EXPECTED_USERS = [
    "admin",
    "basic",
    "graham",
    "testuser1",
] as const;

type SourceFilePerms = {
    view: boolean;
    download: boolean;
    modify: boolean;
    create: boolean;
    delete: boolean;
};

type UserExpectation = {
    username: string;
    global: {
        administrator: boolean;
        shareFiles: boolean;
        apiTokens: boolean;
        realtime: boolean;
        lockPassword: boolean;
    };
    loginMethod: string;
    sources: Record<string, SourceFilePerms>;
};

const ALL_SOURCES = ["playwright + files", "docker", "access"] as const;

const DEFAULT_SOURCE_PERMS: SourceFilePerms = {
    view: true,
    download: true,
    modify: false,
    create: false,
    delete: false,
};

const GRAHAM_SOURCE_PERMS: SourceFilePerms = {
    view: true,
    download: true,
    modify: true,
    create: true,
    delete: false,
};

function sourcePermissionsForAllSources(
    overrides?: Partial<SourceFilePerms>,
): Record<string, SourceFilePerms> {
    const perms = { ...DEFAULT_SOURCE_PERMS, ...overrides };
    return Object.fromEntries(ALL_SOURCES.map((name) => [name, { ...perms }]));
}

function sourcePermissionsForSources(
    sourceNames: readonly string[],
    perms: SourceFilePerms,
): Record<string, SourceFilePerms> {
    return Object.fromEntries(sourceNames.map((name) => [name, { ...perms }]));
}

/** Migrated users + testuser1 (added in Dockerfile.playwright-settings). */
const EXPECTED_USER_DETAILS: UserExpectation[] = [
    {
        username: "admin",
        global: {
            administrator: true,
            shareFiles: true,
            apiTokens: true,
            realtime: false,
            lockPassword: false,
        },
        loginMethod: "Password",
        sources: sourcePermissionsForAllSources({
            view: true,
            download: true,
            modify: true,
            create: true,
            delete: true,
        }),
    },
    {
        username: "basic",
        global: {
            administrator: false,
            shareFiles: true,
            apiTokens: false,
            realtime: false,
            lockPassword: false,
        },
        loginMethod: "Password",
        sources: sourcePermissionsForAllSources({
            view: true,
            download: false,
            modify: false,
            create: false,
            delete: false,
        }),
    },
    {
        username: "graham",
        global: {
            administrator: false,
            shareFiles: false,
            apiTokens: false,
            realtime: false,
            lockPassword: false,
        },
        loginMethod: "Password",
        sources: sourcePermissionsForSources(
            ["playwright + files", "docker"],
            GRAHAM_SOURCE_PERMS,
        ),
    },
    {
        username: "testuser1",
        global: {
            administrator: false,
            shareFiles: false,
            apiTokens: false,
            realtime: false,
            lockPassword: false,
        },
        loginMethod: "Password",
        sources: sourcePermissionsForAllSources(),
    },
];

type ApiTokenExpectation = {
    name: string;
    minimal: boolean;
    permissions?: Partial<Record<string, boolean>>;
};

/** Named API tokens from database.db.old (admin user). */
const EXPECTED_API_TOKENS: ApiTokenExpectation[] = [
    {
        name: "customized",
        minimal: false,
        permissions: {
            admin: true,
            share: false,
            api: false,
            realtime: false,
        },
    },
    {
        name: "full",
        minimal: true,
    },
];

type AccessRuleExpectation = {
    path: string;
    denyTotal: number;
    allowTotal: number;
};

/** Access rules per source after migration. */
const EXPECTED_ACCESS_RULES: Array<[string, AccessRuleExpectation[]]> = [
    [
        "playwright + files",
        [{ path: "/text-files/bash.sh/", denyTotal: 1, allowTotal: 0 }],
    ],
    ["docker", []],
    [
        "access",
        [{ path: "/", denyTotal: 1, allowTotal: 0 }],
    ],
];

type ShareExpectation = {
    hash: string;
    path: string;
    username: string;
    allowModify: boolean;
    allowCreate: boolean;
    allowDelete: boolean;
};

/** Shares migrated from database.db.old (admin user). */
const EXPECTED_SHARES: ShareExpectation[] = [
    {
        hash: "lMhwHkF-hqCN92-QIJJZow",
        path: "/myfolder/",
        username: "admin",
        allowModify: true,
        allowCreate: true,
        allowDelete: true,
    },
    {
        hash: "dGhQi4AcMhva2Ne-7x7fvw",
        path: "/test & test.txt/",
        username: "admin",
        allowModify: false,
        allowCreate: false,
        allowDelete: false,
    },
];

function userRowInSettingsUsersTable(page: Page, usernameText: string): Locator {
    return page.locator("table.settings-table tbody tr").filter({
        has: page.locator("td").first().filter({ hasText: new RegExp(`^${usernameText}$`) }),
    });
}

function shareRowInSettingsSharesTable(page: Page, hash: string): Locator {
    return sharesTable(page).locator("tbody tr").filter({
        has: page.locator("td").first().filter({ hasText: new RegExp(`^${hash}$`) }),
        hasNot: page.locator(".settings-table__empty-cell, .settings-table__loading-cell"),
    });
}

function editUserTrigger(row: Locator): Locator {
    return row.getByRole("button", { name: /Edit/i });
}

function userEditScopeBlock(modal: Locator, sourceName: string): Locator {
    const escaped = sourceName.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
    return modal.locator(
        `.scope-block:has([aria-label="user-edit-scope-path-${escaped}"])`,
    );
}

async function expandUserEditSourceScope(modal: Locator, sourceName: string) {
    const block = userEditScopeBlock(modal, sourceName);
    await block.locator(".settings-group-title").click();
    await expect(block.locator(".source-file-permissions")).toBeVisible();
}

function globalPermissionCheckbox(modal: Locator, label: string | RegExp): Locator {
    return modal.locator(".toggle-container", { hasText: label }).locator('input[type="checkbox"]');
}

function sourcePermissionCheckbox(
    modal: Locator,
    sourceName: string,
    permissionLabel: string,
): Locator {
    return userEditScopeBlock(modal, sourceName)
        .locator(".source-file-permissions .toggle-container", { hasText: permissionLabel })
        .locator('input[type="checkbox"]');
}

function sharePermissionCheckbox(modal: Locator, ariaLabel: string): Locator {
    return modal.locator(`input[type="checkbox"][aria-label="${ariaLabel}"]`);
}

async function openSettingsSection(page: Page, sidebarId: string) {
    await page.goto("/settings");
    await expect(page).toHaveTitle("Graham's Filebrowser - Settings");
    await page.locator(`#${sidebarId}`).click();
}

async function expectCheckboxState(checkbox: Locator, checked: boolean) {
    await expect(checkbox).toBeChecked({ checked });
}

function accessRulesTable(page: Page) {
    return page.getByRole("table", { name: "Access Management" });
}

function sharesTable(page: Page) {
    return page.getByRole("table", { name: "Share management" });
}

async function readAccessRuleRows(page: Page): Promise<AccessRuleExpectation[]> {
    const rows = accessRulesTable(page).locator(
        "tbody tr:not(.settings-table__loading-row):not(:has(.settings-table__empty-cell))",
    );
    const count = await rows.count();
    const result: AccessRuleExpectation[] = [];
    for (let i = 0; i < count; i++) {
        const row = rows.nth(i);
        result.push({
            path: (await row.locator("td").nth(0).innerText()).trim(),
            denyTotal: Number((await row.locator("td").nth(1).innerText()).trim()),
            allowTotal: Number((await row.locator("td").nth(2).innerText()).trim()),
        });
    }
    return result;
}

async function selectAccessSource(page: Page, sourceName: string) {
    const accessCard = page.locator(".card-title").filter({
        has: page.locator("h2", { hasText: /Access Management/i }),
    });
    const sourceButton = accessCard.locator('button[aria-label="Source"]');
    const currentSource = (
        await sourceButton.locator(".expand-dropdown-trigger-label").innerText()
    ).trim();
    if (currentSource === sourceName) {
        await expect(accessRulesTable(page)).not.toHaveAttribute("aria-busy", "true");
        return;
    }

    await expect(accessRulesTable(page)).not.toHaveAttribute("aria-busy", "true");
    await sourceButton.scrollIntoViewIfNeeded();
    await sourceButton.click();
    await expect(sourceButton).toHaveAttribute("aria-expanded", "true", { timeout: 15_000 });

    const listbox = page.getByRole("listbox", { name: "Source" });
    const option = listbox.getByRole("option", { name: sourceName, exact: true });
    await expect(option).toBeVisible({ timeout: 15_000 });

    const rulesLoaded = page.waitForResponse(
        (response) => {
            if (response.request().method() !== "GET" || !response.ok()) {
                return false;
            }
            try {
                return new URL(response.url()).searchParams.get("source") === sourceName;
            } catch {
                return false;
            }
        },
        { timeout: 15_000 },
    );

    await Promise.all([rulesLoaded, option.click()]);
    await expect(accessRulesTable(page)).not.toHaveAttribute("aria-busy", "true", {
        timeout: 15_000,
    });
}

function apiTokenRow(page: Page, tokenName: string): Locator {
    return page.locator("table.settings-table tbody tr").filter({
        has: page.locator("td").first().filter({ hasText: new RegExp(`^${tokenName}$`) }),
        hasNot: page.locator(".settings-table__empty-cell, .settings-table__loading-cell"),
    });
}

test.describe("Migration fixture verification", () => {
    test.describe.configure({ timeout: 60_000 });

    test("admin sidebar links exist in the correct order", async ({ page, checkForErrors }) => {
        await page.goto("/settings");
        await expect(page).toHaveTitle("Graham's Filebrowser - Settings");

        await page.goto("/files/");
        await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

        const sidebarAnchors = page.locator(".sidebar-links .inner-card > a");
        await expect(sidebarAnchors).toHaveCount(EXPECTED_ADMIN_SIDEBAR_LINKS.length);
        const sidebarLabels = await sidebarAnchors.evaluateAll((anchors) =>
            anchors.map((anchor) => anchor.getAttribute("aria-label")),
        );
        expect(sidebarLabels).toEqual([...EXPECTED_ADMIN_SIDEBAR_LINKS]);

        await page.getByLabel("Edit Links").click();
        const prompt = page.locator('[aria-label="SidebarLinks-prompt"]');
        await expect(prompt).toBeVisible();

        const customizeNames = await prompt.locator(".link-item .link-name").allTextContents();
        expect(customizeNames).toEqual([...EXPECTED_ADMIN_SIDEBAR_LINKS]);

        await prompt.getByRole("button", { name: "Close" }).click();
        checkForErrors();
    });

    test("all four migrated users exist with expected permissions", async ({
        page,
        checkForErrors,
    }) => {
        await openSettingsSection(page, "users-sidebar");
        const rows = page.locator("table.settings-table tbody tr");
        await expect(rows).toHaveCount(EXPECTED_USERS.length);

        for (const username of EXPECTED_USERS) {
            await expect(userRowInSettingsUsersTable(page, username)).toHaveCount(1);
        }

        for (const expected of EXPECTED_USER_DETAILS) {
            const userLoad = page.waitForResponse(
                (response) =>
                    response.url().includes(`/public/api/users?username=${expected.username}`) &&
                    response.ok(),
            );
            await editUserTrigger(userRowInSettingsUsersTable(page, expected.username)).click();
            const modal = page.locator('div[aria-label="user-edit-prompt"]');
            await expect(modal).toBeVisible();
            await userLoad;

            await expectCheckboxState(
                globalPermissionCheckbox(modal, "Administrator"),
                expected.global.administrator,
            );
            await expectCheckboxState(
                globalPermissionCheckbox(modal, "Share files"),
                expected.global.shareFiles,
            );
            await expectCheckboxState(
                globalPermissionCheckbox(modal, /Create and manage long-live API tokens/i),
                expected.global.apiTokens,
            );
            await expectCheckboxState(
                globalPermissionCheckbox(modal, "Enable real-time connections and updates"),
                expected.global.realtime,
            );
            await expectCheckboxState(
                globalPermissionCheckbox(modal, "Prevent the user from changing the password"),
                expected.global.lockPassword,
            );

            await expect(modal.locator("#loginMethod .expand-dropdown-trigger-label")).toHaveText(
                expected.loginMethod,
            );

            for (const sourceName of Object.keys(expected.sources)) {
                const sourcePerms = expected.sources[sourceName];
                await expandUserEditSourceScope(modal, sourceName);

                await expectCheckboxState(
                    sourcePermissionCheckbox(modal, sourceName, "View and list files"),
                    sourcePerms.view,
                );
                await expectCheckboxState(
                    sourcePermissionCheckbox(modal, sourceName, "Download files"),
                    sourcePerms.download,
                );
                await expectCheckboxState(
                    sourcePermissionCheckbox(modal, sourceName, "Edit files"),
                    sourcePerms.modify,
                );
                await expectCheckboxState(
                    sourcePermissionCheckbox(modal, sourceName, "Create files"),
                    sourcePerms.create,
                );
                await expectCheckboxState(
                    sourcePermissionCheckbox(modal, sourceName, "Delete files"),
                    sourcePerms.delete,
                );
            }

            await modal.locator('button[aria-label="Cancel"]').click();
            await expect(modal).not.toBeVisible();
        }

        checkForErrors();
    });

    test("migrated API tokens exist with expected properties", async ({
        page,
        checkForErrors,
    }) => {
        await page.goto("/settings");
        await expect(page).toHaveTitle("Graham's Filebrowser - Settings");

        const tokenListResponse = page.waitForResponse(
            (response) =>
                response.url().includes("/api/auth/token/list") &&
                response.status() === 200,
        );
        await page.locator("#api-sidebar").click();
        await expect(page.locator(".card-title h2")).toHaveText(/API Tokens/i);
        const listResp = await tokenListResponse;
        const apiTokens = (await listResp.json()) as Array<{
            name: string;
            Permissions?: Record<string, boolean>;
        }>;
        expect(apiTokens.map((token) => token.name).sort()).toEqual(
            EXPECTED_API_TOKENS.map((token) => token.name).sort(),
        );

        const tokenRows = page.locator("table.settings-table tbody tr").filter({
            hasNot: page.locator(".settings-table__empty, .settings-table__loading-row"),
        });
        await expect(tokenRows).toHaveCount(EXPECTED_API_TOKENS.length);

        for (const expected of EXPECTED_API_TOKENS) {
            const apiToken = apiTokens.find((token) => token.name === expected.name);
            expect(apiToken).toBeDefined();

            if (expected.minimal) {
                expect(
                    !apiToken?.Permissions ||
                        Object.values(apiToken.Permissions).every((value) => !value),
                ).toBe(true);
            } else if (expected.permissions) {
                for (const [permission, enabled] of Object.entries(expected.permissions)) {
                    expect(apiToken?.Permissions?.[permission]).toBe(enabled);
                }
            }

            await expect(apiTokenRow(page, expected.name)).toHaveCount(1);
            const row = apiTokenRow(page, expected.name);
            await row.getByRole("button", { name: /Actions/i }).click();

            const prompt = page.locator('[aria-label="ActionApi-prompt"]');
            await expect(prompt).toBeVisible();
            await expect(prompt.locator(".api-key-name")).toHaveText(expected.name);

            if (expected.minimal) {
                await expect(prompt.locator(".minimal-info")).toBeVisible();
            } else if (expected.permissions) {
                await expect(prompt.locator(".permissions-grid")).toBeVisible();
            }

            await prompt.getByRole("button", { name: /Close/i }).click();
            await expect(prompt).not.toBeVisible();
        }

        checkForErrors();
    });

    test("migrated shares exist with expected properties", async ({ page, checkForErrors }) => {
        await page.goto("/settings");
        await expect(page).toHaveTitle("Graham's Filebrowser - Settings");

        const shareListResponse = page.waitForResponse(
            (response) =>
                response.url().includes("/api/share/list") && response.status() === 200,
        );
        await page.locator("#shares-sidebar").click();
        await expect(page.locator(".card-title h2")).toHaveText(/Share management/i);
        const listResp = await shareListResponse;
        const shares = (await listResp.json()) as Array<{ hash: string }>;
        for (const expected of EXPECTED_SHARES) {
            expect(shares.some((share) => share.hash === expected.hash)).toBe(true);
        }

        await expect(sharesTable(page)).not.toHaveAttribute("aria-busy", "true");

        for (const expected of EXPECTED_SHARES) {
            const row = shareRowInSettingsSharesTable(page, expected.hash);
            await expect(row).toHaveCount(1);
            await expect(row.locator("td").nth(1)).toHaveText(expected.path);
            await expect(row.locator("td").nth(2)).toHaveText("Permanent");
            await expect(row.locator("td").nth(4)).toHaveText(expected.username);

            await row.getByRole("button", { name: /Edit/i }).click();
            const prompt = page.locator('[aria-label="share-prompt"]');
            await expect(prompt).toBeVisible();

            await expectCheckboxState(
                sharePermissionCheckbox(prompt, "allow editing files toggle"),
                expected.allowModify,
            );
            await expectCheckboxState(
                sharePermissionCheckbox(
                    prompt,
                    "allow creating and uploading files and folders toggle",
                ),
                expected.allowCreate,
            );
            await expectCheckboxState(
                sharePermissionCheckbox(prompt, "allow deleting files toggle"),
                expected.allowDelete,
            );

            await prompt.getByRole("button", { name: "Close" }).click();
            await expect(prompt).not.toBeVisible();
        }

        checkForErrors();
    });

    test("access rules exist for each source", async ({ page, checkForErrors }) => {
        await openSettingsSection(page, "access-sidebar");
        await expect(page.locator(".card-title h2")).toHaveText(/Access Management/i);
        await expect(accessRulesTable(page)).not.toHaveAttribute("aria-busy", "true");

        for (const [sourceName, rules] of EXPECTED_ACCESS_RULES) {
            await selectAccessSource(page, sourceName);

            const table = accessRulesTable(page);
            await expect(table).toBeVisible();

            if (rules.length === 0) {
                await expect
                    .poll(async () => readAccessRuleRows(page))
                    .toEqual([]);
                continue;
            }

            await expect
                .poll(async () => readAccessRuleRows(page))
                .toEqual(rules);
        }

        checkForErrors();
    });
});
