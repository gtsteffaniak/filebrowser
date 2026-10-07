import { expect, test } from "../test-setup";
import {
    confirmActorPasswordPrompt,
    openUserEdit,
    userRowInSettingsUsersTable,
} from "./user-edit-helpers";

test.describe("Groups management", () => {
    test("create group with member picker, assign via user edit, delete", async ({
        page,
        checkForErrors,
    }, testInfo) => {
        test.setTimeout(45000);
        const groupName = `pw-group-${testInfo.retry + 1}`;
        const username = `pw-user-${testInfo.retry + 1}`;

        // Create a user to pick as a member.
        await page.goto("/settings");
        await page
            .locator(".settings-card-collapsible")
            .filter({ hasText: "User management" })
            .locator(".settings-card-collapsible-header")
            .click();
        await page.locator('button[aria-label="New user"]').click();
        await page.locator("#username").fill(username);
        await page.locator('input[aria-label="Password1"]').fill("testpassword");
        await page.locator('input[aria-label="Password2"]').fill("testpassword");
        await page.locator('button[aria-label="Save"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(userRowInSettingsUsersTable(page, username)).toBeVisible();

        // Create a group and add the user through the member picker.
        await page.goto("/settings#users-groups");
        await page.locator('button[aria-label="New group"]').click();
        const groupPrompt = page.locator('div[aria-label="group-edit-prompt"]');
        await expect(groupPrompt).toBeVisible();
        await groupPrompt.locator("#group-name").fill(groupName);
        // Members are picked inline in the same prompt: partial, case-insensitive search.
        await groupPrompt.locator("input.entity-picker-filter").fill(username.slice(3, 8));
        const memberOption = groupPrompt.getByRole("option", { name: username });
        await memberOption.click();
        await expect(memberOption).toHaveAttribute("aria-selected", "true");
        await groupPrompt.locator('button[aria-label="Save"]').click();
        await expect(groupPrompt).not.toBeVisible();

        // Group shows in the groups table with the member.
        const groupsTable = page.locator('.settings-table-wrapper');
        await expect(groupsTable).toContainText(groupName);
        await expect(groupsTable).toContainText(username);

        // Duplicate group name is blocked inline.
        await page.locator('button[aria-label="New group"]').click();
        await groupPrompt.locator("#group-name").fill(groupName);
        await expect(groupPrompt).toContainText("already exists");
        await groupPrompt.locator('button[aria-label="Cancel"]').click();
        await expect(groupPrompt).not.toBeVisible();

        // Assign the user to the group is already done; remove it via user edit,
        // then re-add through the group picker.
        await page.goto("/settings");
        await page
            .locator(".settings-card-collapsible")
            .filter({ hasText: "User management" })
            .locator(".settings-card-collapsible-header")
            .click();
        const modal = await openUserEdit(page, userRowInSettingsUsersTable(page, username), {
            username,
        });
        const groupsField = modal.locator(".user-edit-hub");
        await groupsField.locator(".entity-picker-button").click();
        const picker = page.locator('div[aria-label="entityPicker-prompt"]');
        await expect(picker).toBeVisible();
        // The picker shows the user context; membership is the checked state in the list.
        await expect(picker).toContainText(username);
        const groupOption = picker.getByRole("option", { name: groupName });
        await expect(groupOption).toHaveAttribute("aria-selected", "true");
        // Uncheck, then re-check the group.
        await groupOption.click();
        await expect(groupOption).toHaveAttribute("aria-selected", "false");
        await groupOption.click();
        await picker.locator('button[aria-label="Save"]').click();
        await expect(picker).not.toBeVisible();
        // Only group membership changed, so the save skips the user PATCH and
        // no actor-password challenge appears.
        await modal.locator('button[aria-label="Save"]').click();
        await expect(modal).not.toBeVisible();

        // Delete the group again from the groups list.
        await page.goto("/settings#users-groups");
        const groupRow = page
            .locator(".settings-table-wrapper tr")
            .filter({ hasText: groupName });
        await expect(groupRow).toContainText(username);
        await groupRow.locator('button[aria-label="Delete"]').click();
        const genericModal = page.locator('div[aria-label="generic-prompt"]');
        await genericModal.locator('button[aria-label="Delete"]').click();
        await expect(page.locator(".settings-table-wrapper")).not.toContainText(groupName);

        // Clean up the user.
        await page.goto("/settings");
        await page
            .locator(".settings-card-collapsible")
            .filter({ hasText: "User management" })
            .locator(".settings-card-collapsible-header")
            .click();
        const modal2 = await openUserEdit(page, userRowInSettingsUsersTable(page, username), {
            username,
        });
        await modal2.locator('button[aria-label="Delete User"]').click();
        await genericModal.locator('button[aria-label="Delete"]').click();
        await confirmActorPasswordPrompt(page);
        await expect(userRowInSettingsUsersTable(page, username)).not.toBeVisible();
        checkForErrors();
    });
});
