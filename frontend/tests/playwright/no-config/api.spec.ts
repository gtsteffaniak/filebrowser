import { test, expect } from "../test-setup";


test("verify scoped user can't access files outside of their scope", async ({ page }) => {
    // set basic auth credentials for protected /subpath route
    await page.setExtraHTTPHeaders({
        'Authorization': `Basic ZGVtby0xMjcuMC4wLjE6U2VjdXJlUGFzczEyMyE=`
    });

    // try to access protected route without credentials - should get 401
    const response = await page.goto("/api/resources?path=../&source=backend", { waitUntil: 'networkidle' });
    expect(response?.status()).toBe(403);
});

