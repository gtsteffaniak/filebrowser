import { expect, test } from "../test-setup";

/**
 * No JWT: `sharePrepStorage.json` from global setup has only localStorage
 * (shareHash, shareHashFile, rootShareHash), not `filebrowser_quantum_jwt`.
 */
test.use({ storageState: "sharePrepStorage.json" });

test("view share as anonymous user", async ({ page, checkForErrors }) => {
  await page.goto("/public/api/health");
  const shareHash = await page.evaluate(() => localStorage.getItem("shareHash"));
  if (!shareHash) {
    throw new Error("Share hash not found in localStorage");
  }

  const response = await page.goto(`/public/share/${shareHash}/testdata/`);
  expect(response?.status()).toBe(200);
  // Verify final URL and title after redirect
  await expect(page).toHaveURL(new RegExp(`/public/share/${shareHash}/testdata/`));
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await expect(page.locator('a[aria-label="gray-sample.jpg"]')).toBeVisible();

  const userCardHtml = await page.locator(".user-card").innerHTML();
  expect(userCardHtml).toContain('aria-label="Login"');

  await expect(page.locator('.user-card').getByRole("button", { name: "Login" })).toBeVisible();
  checkForErrors(0,1); // error 401 on login attempt
});

test("public share info JSON (no banner, canEditShare false for anonymous)", async ({ page }, testInfo) => {
  await page.goto("/public/api/health");
  const shareHash = await page.evaluate(() => localStorage.getItem("shareHash"));
  if (!shareHash) throw new Error("shareHash is missing (global-setup sharePrepStorage)");
  expect(shareHash, "shareHash from global-setup sharePrepStorage").toBeTruthy();

  // Leading "/" is resolved from the **origin root**, so with baseURL like http://host:8080/testing/
  // fetch("/public/...") hits http://host:8080/public/... and MISSES the /testing prefix. Build URL from
  // project baseURL instead.
  const rawBase =
    (testInfo.project.use as { baseURL?: string }).baseURL ?? "http://127.0.0.1/";
  const baseNorm = rawBase.endsWith("/") ? rawBase : `${rawBase}/`;
  const infoUrl = new URL(
    `public/api/share/info?hash=${encodeURIComponent(shareHash)}`,
    baseNorm,
  ).href;

  const payload = await page.evaluate(async (url) => {
    const res = await fetch(url, { credentials: "omit" });
    const text = await res.text();
    return { status: res.status, text };
  }, infoUrl);

  expect(payload.status, payload.text).toBe(200);
  const data = JSON.parse(payload.text) as Record<string, unknown>;

  const banner = data.banner;
  const bannerUrl = data.bannerUrl;
  expect(banner == null || banner === "").toBe(true);
  expect(bannerUrl == null || bannerUrl === "").toBe(true);

  // Fails if the API returns the incorrect case: anonymous viewer must not be told they can edit.
  expect(
    data.canEditShare === true,
    `anonymous share info must not set canEditShare to true.\nURL: ${infoUrl}\nBody:\n${payload.text}`,
  ).toBe(false);

  const sidebarLinks = (data.sidebarLinks ?? []) as Array<{
    name?: string;
    category?: string;
  }>;
  expect(
    sidebarLinks.some((l) => l.name === "sourceLocation"),
    `anonymous share info must not include a sourceLocation sidebar link.\nURL: ${infoUrl}\nBody:\n${payload.text}`,
  ).toBe(false);

  const src = data.sourceURL;
  expect(
    src == null || src === "",
    `anonymous share info must not include sourceURL (internal path).\nURL: ${infoUrl}\nBody:\n${payload.text}`,
  ).toBe(true);

  expect(data.shareType).toBe("normal");
  expect(typeof data.shareURL === "string").toBe(true);
  expect(String(data.shareURL)).toContain(shareHash);
});

test("anonymous visitor can stream video from myfolder share", async ({ page, checkForErrors }, testInfo) => {
  await page.goto("/public/api/health");
  const shareHash = await page.evaluate(() => localStorage.getItem("shareHash"));
  if (!shareHash) throw new Error("shareHash is missing (global-setup sharePrepStorage)");

  const streamResponse = page.waitForResponse(
    (res) =>
      res.url().includes("/public/api/media/stream") &&
      res.request().method() === "GET",
    { timeout: 60_000 },
  );

  await page.goto(`/public/share/${shareHash}/testdata/`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Share - testdata");
  await expect(page.locator('a[aria-label="sample-640x480.mp4"]')).toBeVisible();

  await page.locator('a[aria-label="sample-640x480.mp4"]').dblclick();
  await expect(page).toHaveTitle(/sample-640x480\.mp4/);

  const playButton = page.locator(".plyr-viewer .plyr__control--overlaid, .plyr-viewer button.plyr__control[data-plyr='play']");
  await playButton.first().click({ timeout: 15_000 });

  const stream = await streamResponse;
  expect(stream.status(), await stream.text()).not.toBe(403);
  expect([200, 206]).toContain(stream.status());

  const rawBase =
    (testInfo.project.use as { baseURL?: string }).baseURL ?? "http://127.0.0.1/";
  const baseNorm = rawBase.endsWith("/") ? rawBase : `${rawBase}/`;
  const viewTokenUrl = new URL(
    `public/api/resources/view-token?hash=${encodeURIComponent(shareHash)}`,
    baseNorm,
  ).href;
  const viewTokenPayload = await page.evaluate(async (url) => {
    const res = await fetch(url, {
      method: "POST",
      credentials: "omit",
    });
    const text = await res.text();
    return { status: res.status, text };
  }, viewTokenUrl);
  expect(viewTokenPayload.status, viewTokenPayload.text).toBe(200);
  const viewTokenBody = JSON.parse(viewTokenPayload.text) as { viewToken?: string };
  expect(viewTokenBody.viewToken?.length).toBeGreaterThan(0);

  checkForErrors(0, 1);
});
