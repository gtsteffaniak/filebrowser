import { expect, test } from "../test-setup.ts";
import {
  domClick,
  enableTranscodeUi,
  mockTranscodeSessions,
  nativePlyrVideo,
  plyrSettingsButton,
  plyrTranscodeMenuButton,
  selectTranscodeMode,
  startNativeVideoPlayback,
  waitForNativePlyrMounted,
} from "./transcode-helpers.ts";

const TESTDATA = "/files/playwright-files/myfolder/testdata";
/** VP8/WebM plays natively in Playwright Chromium; H.264 MP4 often does not in headless builds. */
const NATIVE_PREVIEW_VIDEO = "clip-a.webm";

/**
 * Media endpoints that probe or transcode (`/api/media/metadata`,
 * `/api/media/transcode/sessions`) can fail. Count those failures so the error
 * assertion stays precise instead of allowing API errors wholesale.
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

test("mp4 preview mounts the native player", async ({ page, checkForErrors }) => {
  const mediaFailures = trackMediaApiFailures(page);

  await page.goto(`${TESTDATA}/${NATIVE_PREVIEW_VIDEO}`);
  await expect(page).toHaveTitle(`Graham's Filebrowser - Files - ${NATIVE_PREVIEW_VIDEO}`);

  // Native previews lazy-bind /api/media/stream on first play, not at mount.
  await startNativeVideoPlayback(page);
  const video = nativePlyrVideo(page);
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
  // In offer mode the page shows the title (not .transcode-offer-message, which is
  // only rendered for the other modes) and the Transcode button.
  const title = page.locator(".info .title");
  await expect(title).toBeVisible({ timeout: 15_000 });
  await expect(title).toContainText("not supported by your browser");
  await expect(page.locator('button[aria-label="Transcode"]')).toBeVisible();

  checkForErrors(0, mediaFailures());
});

test("switching transcode modes sends replaceSessionId for the same file", async ({
  page,
}) => {
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

  await page.goto(`${TESTDATA}/${NATIVE_PREVIEW_VIDEO}`);
  await expect(page).toHaveTitle(`Graham's Filebrowser - Files - ${NATIVE_PREVIEW_VIDEO}`);
  await waitForNativePlyrMounted(page);

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

test("transcode mode menu is shown when the server enables transcoding", async ({
  page,
  checkForErrors,
}) => {
  const mediaFailures = trackMediaApiFailures(page);

  // Real server config (no mocks): the preview image enables ffmpeg and transcoding.
  await page.goto(`${TESTDATA}/${NATIVE_PREVIEW_VIDEO}`);
  await waitForNativePlyrMounted(page);

  const ffmpegAvailable = await page.evaluate(() => window.globalVars?.ffmpegAvailable === true);
  expect(ffmpegAvailable, "server reports ffmpeg unavailable").toBe(true);

  const settings = plyrSettingsButton(page);
  await expect(settings).toBeAttached({ timeout: 15_000 });
  await domClick(settings);
  await expect(plyrTranscodeMenuButton(page)).toBeVisible({ timeout: 10_000 });

  checkForErrors(0, mediaFailures());
});
