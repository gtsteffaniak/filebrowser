import { expect, test } from "../test-setup.ts";

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

test("anonymous file and directory shares apply the theme toggle", async ({ page, checkForErrors }) => {
  test.setTimeout(20000);
  await page.goto("/public/api/health");
  const shareHash = await page.evaluate(() => localStorage.getItem("shareHash"));
  const shareHashFile = await page.evaluate(() => localStorage.getItem("shareHashFile"));
  if (!shareHash || !shareHashFile) {
    throw new Error("Share hash not found in localStorage");
  }

  const userUpdates: string[] = [];
  page.on("request", (request) => {
    if (request.method() !== "PATCH") {
      return;
    }
    const pathname = new URL(request.url()).pathname;
    if (pathname.endsWith("/api/users")) {
      userUpdates.push(request.url());
    }
  });

  const shares = [
    {
      path: `/public/share/${shareHashFile}`,
      title: "Graham's Filebrowser - Share - 1file1.txt",
    },
    {
      path: `/public/share/${shareHash}/testdata/`,
      title: "Graham's Filebrowser - Share - testdata",
    },
  ];

  for (const share of shares) {
    const response = await page.goto(share.path);
    expect(response?.status()).toBe(200);
    await expect(page).toHaveTitle(share.title);

    const toggle = page.locator('[aria-label="Toggle Theme"]');
    await expect(toggle).toBeVisible();

    const readTheme = () => page.evaluate(() => {
      const control = document.querySelector('[aria-label="Toggle Theme"]');
      const darkMode = document.documentElement.classList.contains("dark-mode");
      return {
        darkMode,
        colorScheme: document.documentElement.style.colorScheme,
        active: control?.classList.contains("active") ?? false,
      };
    });

    const start = await readTheme();
    expect(start.active).toBe(start.darkMode);
    expect(start.colorScheme).toBe(start.darkMode ? "dark" : "light");

    await toggle.click();
    await expect.poll(readTheme).toEqual({
      darkMode: !start.darkMode,
      colorScheme: start.darkMode ? "light" : "dark",
      active: !start.darkMode,
    });

    await toggle.click();
    await expect.poll(readTheme).toEqual(start);
  }

  expect(userUpdates).toEqual([]);
  // Each share load records the anonymous self-user 401. Toggling adds none.
  checkForErrors(0, 2);
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
