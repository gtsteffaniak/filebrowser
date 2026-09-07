import type { Locator, Page, Response } from "@playwright/test";
import { expect } from "../test-setup";

/** Account defaults block in the user edit modal (global permissions + lockPassword). */
export function userEditAccountSection(modal: Locator): Locator {
    return modal.locator(".settings-group").filter({ hasText: "Account defaults" });
}

export function accountPermissionCheckbox(modal: Locator, label: string) {
    const section = userEditAccountSection(modal);
    return {
        input: section.locator(".toggle-container", { hasText: label }).locator('input[type="checkbox"]'),
        switch: section.locator(".toggle-container", { hasText: label }).locator("label.switch"),
    };
}

function userEditUserLoadResponse(page: Page, username: string): Promise<Response> {
    const encoded = encodeURIComponent(username);
    return page.waitForResponse(
        (response) => {
            const url = response.url();
            return (
                (url.includes(`/api/users?username=${encoded}`) ||
                    url.includes(`/public/api/users?username=${encoded}`)) &&
                response.request().method() === "GET" &&
                response.ok()
            );
        },
    );
}

function userEditSourcesLoadResponse(page: Page): Promise<Response> {
    return page.waitForResponse(
        (response) =>
            response.url().includes("/api/settings/sources") && response.ok(),
    );
}

/** Start network waiters before opening edit; call after click + modal visible. */
export function startUserEditLoadWaiters(page: Page, username: string) {
    return {
        userLoad: userEditUserLoadResponse(page, username),
        sourcesLoad: userEditSourcesLoadResponse(page),
    };
}

/** Waits for user GET, sources catalogue, and account section to finish initializing. */
export async function waitForUserEditReady(
    modal: Locator,
    waiters: { userLoad: Promise<Response>; sourcesLoad: Promise<Response> },
): Promise<void> {
    await Promise.all([waiters.userLoad, waiters.sourcesLoad]);
    await expect(
        accountPermissionCheckbox(modal, "Administrator").input,
    ).toBeVisible();
}
