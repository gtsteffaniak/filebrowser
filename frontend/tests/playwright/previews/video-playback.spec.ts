import { expect, test } from "../test-setup.ts";

const TESTDATA = "/files/playwright-files/myfolder/testdata";

/**
 * The Playwright CI image ships no ffmpeg, so media endpoints that probe or
 * transcode (`/api/media/metadata`, `/api/media/transcode/sessions`) fail there.
 * Count those failures so the error assertion stays precise instead of allowing
 * API errors wholesale.
 */
function trackMediaApiFailures(page: import("@playwright/test").Page): () => number {
  let failures = 0;
  page.on("response", (response) => {
    if (response.url().includes("/api/media/") && !response.ok()) {
      failures += 1;
    }
  });
  return () => failures;
}

const MINIMAL_PLAYLIST = `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:2.0,
seg/00000.m4s
#EXT-X-ENDLIST
`;

/**
 * Force the static app-config (and live media settings API) to advertise
 * transcoding so the plyr mode menu appears even when the CI image lacks ffmpeg.
 *
 * Hook getElementById so the patch runs when constants.js reads `#app-config`.
 * Only rewrite the JSON text — do not replace `window.globalVars` with a new
 * object, or the module-scoped binding used by isTranscodeAvailable stays stale.
 */
async function enableTranscodeUi(page: import("@playwright/test").Page) {
  await page.addInitScript(() => {
    const original = Document.prototype.getElementById;
    Document.prototype.getElementById = function patchedGetElementById(id) {
      const el = original.call(this, id);
      if (id === "app-config" && el && el.textContent && !el.dataset.transcodeUiPatched) {
        try {
          const cfg = JSON.parse(el.textContent);
          cfg.mediaAvailable = true;
          cfg.transcodeEnabled = true;
          if (!Array.isArray(cfg.transcodeProfiles) || cfg.transcodeProfiles.length === 0) {
            cfg.transcodeProfiles = ["quality", "balanced", "datasaver"];
          }
          el.textContent = JSON.stringify(cfg);
          el.dataset.transcodeUiPatched = "1";
        } catch (_) {
          // Leave the page config alone if it is not JSON.
        }
      }
      return el;
    };
  });

  await page.route("**/api/settings/media", async (route) => {
    if (route.request().method() !== "GET") {
      return route.continue();
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        enabled: true,
        profiles: ["quality", "balanced", "datasaver"],
        maxConcurrentPerUser: 1,
        hardwareAcceleration: false,
      }),
    });
  });
}

/** Mutate the shared globalVars object in place (module + window share one reference). */
async function forceTranscodeGlobalVars(page: import("@playwright/test").Page) {
  await page.evaluate(() => {
    const g = window.globalVars;
    if (!g || typeof g !== "object") {
      return;
    }
    g.mediaAvailable = true;
    g.transcodeEnabled = true;
    if (!Array.isArray(g.transcodeProfiles) || g.transcodeProfiles.length === 0) {
      g.transcodeProfiles = ["quality", "balanced", "datasaver"];
    }
  });
}

type SessionStart = {
  profile?: string;
  replaceSessionId?: string;
  source?: string;
  path?: string;
};

async function mockTranscodeSessions(
  page: import("@playwright/test").Page,
  opts: { playlistNotReadyFirst?: boolean } = {},
) {
  let sessionCounter = 0;
  const sessions = new Map<string, { id: string; profile: string; source: string; path: string; state: string }>();
  const starts: SessionStart[] = [];
  let playlistHits = 0;

  await page.route("**/api/media/transcode/sessions**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (method === "POST" && /\/sessions\/?$/.test(path)) {
      const body = request.postDataJSON() as SessionStart;
      starts.push(body);
      if (body.replaceSessionId) {
        sessions.delete(body.replaceSessionId);
      }
      // Mirror backend same-file handover when replaceSessionId is omitted.
      for (const [id, sess] of [...sessions.entries()]) {
        if (sess.source === body.source && sess.path === body.path) {
          sessions.delete(id);
        }
      }
      sessionCounter += 1;
      const id = `sess-${sessionCounter}`;
      const profile = body.profile || "quality";
      sessions.set(id, {
        id,
        profile,
        source: body.source || "",
        path: body.path || "",
        state: "starting",
      });
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          id,
          source: body.source,
          path: body.path,
          profile,
          state: "starting",
          startedAt: Date.now() / 1000,
          heartbeatIntervalSec: 12,
          deliveryBaseUrl: `/api/media/transcode/sessions/${id}`,
          snapshot: {
            enabled: true,
            canStart: true,
            sessions: [...sessions.values()],
          },
        }),
      });
      return;
    }

    if (method === "GET" && /\/sessions\/?$/.test(path)) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          enabled: true,
          canStart: sessions.size === 0,
          userLimit: 1,
          userActive: sessions.size,
          sessions: [...sessions.values()],
        }),
      });
      return;
    }

    const playlistMatch = path.match(/\/sessions\/([^/]+)\/index\.m3u8$/);
    if (method === "GET" && playlistMatch) {
      playlistHits += 1;
      if (opts.playlistNotReadyFirst && playlistHits === 1) {
        await route.fulfill({
          status: 503,
          headers: { "Retry-After": "1" },
          contentType: "application/json",
          body: JSON.stringify({ message: "transcode playlist not ready" }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/vnd.apple.mpegurl",
        body: MINIMAL_PLAYLIST,
      });
      return;
    }

    const segmentMatch = path.match(/\/sessions\/([^/]+)\/(?:init\.mp4|seg\/[^/]+)$/);
    if (method === "GET" && segmentMatch) {
      await route.fulfill({
        status: 200,
        contentType: "video/iso.segment",
        body: Buffer.from([0, 0, 0, 0]),
      });
      return;
    }

    if (method === "DELETE") {
      const id = path.split("/").pop() || "";
      sessions.delete(id);
      await route.fulfill({ status: 204, body: "" });
      return;
    }

    if (method === "PATCH" && path.endsWith("/heartbeat")) {
      await route.fulfill({ status: 204, body: "" });
      return;
    }

    await route.continue();
  });

  return { starts, sessions: () => sessions, playlistHits: () => playlistHits };
}

/** Top-level settings gear only — Plyr also stamps data-plyr="settings" on menu back buttons. */
function plyrSettingsButton(page: import("@playwright/test").Page) {
  return page.locator(".plyr-viewer button[data-plyr='settings']:not([role='menuitem'])");
}

function plyrTranscodeMenuButton(page: import("@playwright/test").Page) {
  return page.locator(".plyr-viewer button[data-plyr-menu='transcodeMode']");
}

/** DOM click — Plyr controls are often off-screen / covered in the CI viewport. */
async function domClick(locator: import("@playwright/test").Locator) {
  await locator.evaluate((el: HTMLElement) => el.click());
}

async function openTranscodeModeMenu(page: import("@playwright/test").Page) {
  const settings = plyrSettingsButton(page);
  await expect(settings).toBeAttached({ timeout: 15_000 });
  await domClick(settings);
  const transcodeBtn = plyrTranscodeMenuButton(page);
  await expect(transcodeBtn).toBeAttached({ timeout: 10_000 });
  // ensurePlaybackModeApplied should have cleared hidden when transcode is available.
  await expect(transcodeBtn).not.toHaveAttribute("hidden", { timeout: 10_000 });
  await domClick(transcodeBtn);
}

async function selectTranscodeMode(page: import("@playwright/test").Page, mode: string) {
  await openTranscodeModeMenu(page);
  const option = page.locator(`.plyr-viewer button[data-plyr='transcode-mode'][value='${mode}']`);
  await expect(option).toBeAttached({ timeout: 10_000 });
  await domClick(option);
}

test("mp4 preview mounts the native player", async ({ page, checkForErrors }) => {
  test.setTimeout(30_000);
  const mediaFailures = trackMediaApiFailures(page);

  await page.goto(`${TESTDATA}/sample-640x480.mp4`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - sample-640x480.mp4");

  const video = page.locator(".plyr-viewer video").first();
  await expect(video).toBeVisible({ timeout: 15_000 });
  // A browser-playable format streams straight from the media API, with no
  // transcode session involved.
  await expect(video).toHaveAttribute("src", /media\/stream/, { timeout: 15_000 });
  await expect(page.locator(".transcode-offer-message")).toHaveCount(0);

  checkForErrors(0, mediaFailures());
});

test("wmv preview requires transcoding", async ({ page, checkForErrors }) => {
  test.setTimeout(30_000);
  const mediaFailures = trackMediaApiFailures(page);

  await page.goto(`${TESTDATA}/file_example_WMV_480_1_2MB.wmv`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - file_example_WMV_480_1_2MB.wmv");

  // `.wmv` must be indexed as a video so the preview routes to the transcode UI
  // rather than the generic "no preview" screen.
  const message = page.locator(".transcode-offer-message");
  await expect(message).toBeVisible({ timeout: 15_000 });
  await expect(message).toContainText("not supported by your browser");

  checkForErrors(0, mediaFailures());
});

test("switching transcode modes sends replaceSessionId for the same file", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await enableTranscodeUi(page);
  const mock = await mockTranscodeSessions(page, { playlistNotReadyFirst: true });

  const sessionStatuses: number[] = [];
  const playlistStatuses: number[] = [];
  page.on("response", (response) => {
    const url = response.url();
    if (url.includes("/api/media/transcode/sessions") && response.request().method() === "POST") {
      sessionStatuses.push(response.status());
    }
    if (url.includes("/index.m3u8")) {
      playlistStatuses.push(response.status());
    }
  });

  await page.goto(`${TESTDATA}/sample-640x480.mp4`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - sample-640x480.mp4");
  await expect(page.locator(".plyr-viewer video").first()).toBeVisible({ timeout: 15_000 });
  await forceTranscodeGlobalVars(page);

  await selectTranscodeMode(page, "datasaver");
  await expect.poll(() => mock.starts.length).toBe(1);
  expect(mock.starts[0]?.profile).toBe("datasaver");
  expect(mock.starts[0]?.replaceSessionId).toBeFalsy();
  const firstSessionId = [...mock.sessions().keys()][0];
  expect(firstSessionId).toBeTruthy();
  // Wait for the first switch to finish (playlist may 503 once); otherwise the
  // menu ignores clicks while transcodeSwitchInProgress is true.
  await expect.poll(() => playlistStatuses.filter((status) => status === 200).length).toBeGreaterThanOrEqual(1);

  await selectTranscodeMode(page, "balanced");
  await expect.poll(() => mock.starts.length, { timeout: 15_000 }).toBe(2);
  expect(mock.starts[1]?.profile).toBe("balanced");
  expect(mock.starts[1]?.replaceSessionId).toBe(firstSessionId);

  // Both profile starts must succeed (no 409 "already in use").
  expect(sessionStatuses.every((status) => status === 201 || status === 200)).toBe(true);
  // First playlist poll may be 503 (not ready); later polls must be 200, never 500.
  await expect.poll(() => mock.playlistHits()).toBeGreaterThanOrEqual(1);
  expect(playlistStatuses.some((status) => status === 500)).toBe(false);
  expect(playlistStatuses.some((status) => status === 200 || status === 503)).toBe(true);
});

test("transcode mode menu stays hidden when media is unavailable", async ({
  page,
  checkForErrors,
}) => {
  test.setTimeout(30_000);
  const mediaFailures = trackMediaApiFailures(page);

  await page.goto(`${TESTDATA}/sample-640x480.mp4`);
  await expect(page.locator(".plyr-viewer video").first()).toBeVisible({ timeout: 15_000 });

  const mediaAvailable = await page.evaluate(() => (window as any).globalVars?.mediaAvailable === true);
  if (mediaAvailable) {
    test.skip(true, "ffmpeg is available in this environment; menu visibility is expected");
  }

  const settings = plyrSettingsButton(page);
  await expect(settings).toBeAttached({ timeout: 15_000 });
  await domClick(settings);
  // Slot may exist but must stay hidden when ffmpeg/media is unavailable.
  await expect(plyrTranscodeMenuButton(page)).toBeHidden();

  checkForErrors(0, mediaFailures());
});
