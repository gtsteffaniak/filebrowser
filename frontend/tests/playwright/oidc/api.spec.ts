import { expect, test } from "../test-setup";

test("verify scoped user can't access files outside of their scope", async ({ page }) => {
    // set basic auth credentials for protected /subpath route
    await page.setExtraHTTPHeaders({
        'Authorization': `Basic ZGVtby0xMjcuMC4wLjE6U2VjdXJlUGFzczEyMyE=`
    });

    const response = await page.goto("/api/resources?path=../../etc/passwd&source=playwright-files", { waitUntil: 'networkidle' });

    const responseBody = await response?.json();
    expect(responseBody).toEqual({ status: 400, message: "invalid path: path traversal detected" });
    expect(response?.status()).toBe(400);

});

test("verify scoped user can't access files outside their scope", async ({ page }) => {
    // set basic auth credentials for protected /subpath route
    await page.setExtraHTTPHeaders({
        'Authorization': `Basic ZGVtby0xMjcuMC4wLjE6U2VjdXJlUGFzczEyMyE=`
    });

    const response = await page.goto("/api/resources?path=../&source=playwright-files", { waitUntil: 'networkidle' });

    const responseBody = await response?.json();
    expect(responseBody).toEqual({ status: 400, message: "invalid path: path traversal detected" });
    expect(response?.status()).toBe(400);

});
