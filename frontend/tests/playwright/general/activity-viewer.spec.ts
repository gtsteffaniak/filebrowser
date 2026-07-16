import { expect, test } from "../test-setup";

test.describe("Activity Viewer API", () => {
  test("admin can list activity", async ({ page }) => {
    const response = await page.request.get(
      "http://127.0.0.1/api/tools/activity?from=0&to=9999999999",
    );
    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    expect(body).toHaveProperty("items");
    expect(body).toHaveProperty("total");
  });

  test("admin can fetch activity stats", async ({ page }) => {
    const now = Math.floor(Date.now() / 1000);
    const from = now - 7 * 86400;
    const response = await page.request.get(
      `http://127.0.0.1/api/tools/activity/grouped?from=${from}&to=${now}&interval=hour&splitBy=eventType`,
    );
    expect(response.ok()).toBeTruthy();
    const body = await response.json();
    expect(body).toHaveProperty("buckets");
    expect(Array.isArray(body.buckets)).toBe(true);
  });

  test("non-admin cannot query another user's activity via username param", async ({
    browser,
  }) => {
    const context = await browser.newContext({
      storageState: undefined,
      baseURL: "http://127.0.0.1/",
    });
    const page = await context.newPage();

    await page.goto("/login");
    await page.getByPlaceholder("Username").fill("admin");
    await page.getByPlaceholder("Password").fill("admin");
    await page.getByRole("button", { name: "Login" }).click();
    await page.waitForURL("**/files/**");

    const usersRes = await page.request.get("http://127.0.0.1/api/users");
    expect(usersRes.ok()).toBeTruthy();
    const users = (await usersRes.json()) as Array<{ id: number; username: string; permissions?: { admin?: boolean } }>;
    const adminUser = users.find((u) => u.permissions?.admin);
    expect(adminUser).toBeDefined();

    const nonAdminName = `activity-test-${Date.now()}`;
    const createRes = await page.request.post("http://127.0.0.1/api/users", {
      headers: {
        "Content-Type": "application/json",
        "X-Password": "admin",
      },
      data: {
        which: [],
        data: {
          username: nonAdminName,
          password: "testpassword",
          permissions: {
            admin: false,
            share: true,
            api: false,
            realtime: false,
          },
          scopes: [
            {
              name: "playwright + files",
              scope: "/",
              permissions: {
                view: true,
                download: true,
                modify: true,
                create: true,
                delete: true,
              },
            },
          ],
        },
      },
    });
    expect(createRes.status()).toBe(201);

    await context.close();

    const userContext = await browser.newContext({
      storageState: undefined,
      baseURL: "http://127.0.0.1/",
    });
    const userPage = await userContext.newPage();
    await userPage.goto("/login");
    await userPage.getByPlaceholder("Username").fill(nonAdminName);
    await userPage.getByPlaceholder("Password").fill("testpassword");
    await userPage.getByRole("button", { name: "Login" }).click();
    await userPage.waitForURL("**/files/**");

    const now = Math.floor(Date.now() / 1000);
    const from = now - 86400;
    const scopedRes = await userPage.request.get(
      `http://127.0.0.1/api/tools/activity?from=${from}&to=${now}&username=${encodeURIComponent(adminUser!.username)}`,
    );
    expect(scopedRes.status()).toBe(403);

    await userContext.close();
  });
});

test("activity viewer tool page loads", async ({ page, checkForErrors }) => {
  await page.goto("/tools/activityViewer");
  await expect(page.locator(".topTitle")).toContainText(/Activity Viewer/i);
  await checkForErrors();
});
