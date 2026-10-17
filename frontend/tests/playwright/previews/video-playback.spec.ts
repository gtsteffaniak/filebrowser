import { expect, test } from "../test-setup.ts";
import {
  domClick,
  enableTranscodeUi,
  mockTranscodeSessions,
  plyrSettingsButton,
  plyrTranscodeMenuButton,
  selectTranscodeMode,
} from "./transcode-helpers.ts";

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

  const mediaAvailable = await page.evaluate(() => window.globalVars?.mediaAvailable === true);
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
