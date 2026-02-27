import { test, expect } from "../test-setup";

test("admin user jwt works", async({ page, checkForErrors, context }) => {
    test.skip(test.info().project.name !== "admin-user", "Only run on admin-user project");
    
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - testadmin");
    checkForErrors();
});

test("regular user jwt works", async({ page, checkForErrors, context }) => {
    test.skip(test.info().project.name !== "regular-user", "Only run on regular-user project");
    
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Files - testuser");
    checkForErrors();
});

test("wrong key doesn't work", async({ page, checkForErrors, context }) => {
    test.skip(test.info().project.name !== "wrong-key", "Only run on wrong-key project");
    
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Login");
    checkForErrors(0,1); // expect one api error
});

test("no key doesn't work", async({ page, checkForErrors, context }) => {
    test.skip(test.info().project.name !== "no-user", "Only run on no-user project");
    
    await page.goto("/files/");
    await expect(page).toHaveTitle("Graham's Filebrowser - Login");
    checkForErrors(0,1); // expect one api error
});
