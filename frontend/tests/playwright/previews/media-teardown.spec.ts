import type { Page } from "@playwright/test";
import { expect, test } from "../test-setup.ts";

const MEDIA = "/files/playwright-files/myfolder/testdata";
const LISTING_TITLE = "Graham's Filebrowser - Files - testdata";

declare global {
  interface Window {
    __playedMedia?: HTMLMediaElement[];
  }
}

/**
 * Media endpoints that probe or transcode may fail. Count those failures so the
 * error assertion stays precise instead of allowing API errors wholesale.
 */
function trackMediaApiFailures(page: Page): () => number {
  let failures = 0;
  page.on("response", (response) => {
    if (response.url().includes("/api/media/") && !response.ok()) {
      failures += 1;
    }
  });
  return () => failures;
}

/**
 * Remember every media element that ever started playing. Teardown must stop
 * them even after Vue removes them from the DOM, otherwise a detached element
 * keeps playing audio in the background.
 */
async function trackPlayedMedia(page: Page) {
  await page.addInitScript(() => {
    window.__playedMedia = [];
    const originalPlay = HTMLMediaElement.prototype.play;
    HTMLMediaElement.prototype.play = function () {
      window.__playedMedia?.push(this);
      return originalPlay.call(this);
    };
  });
}

async function startPlayback(page: Page) {
  const video = page.locator(".plyr-viewer video").first();
  await expect(video).toBeVisible({ timeout: 15_000 });
  const isPlaying = () =>
    video.evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0);

  // Opening from the listing can autoplay; otherwise a trusted click on the
  // player surface (or the overlaid play button covering it) starts playback.
  if (!(await isPlaying())) {
    await video.click({ force: true });
  }
  if (!(await isPlaying())) {
    await page.keyboard.press("Space");
  }
  await expect.poll(isPlaying, {
    timeout: 15_000,
    intervals: [100, 250, 500],
  }).toBe(true);
}

async function expectNoMediaPlaying(page: Page) {
  await expect.poll(
    () => page.evaluate(() =>
      Array.from(document.querySelectorAll("video,audio"))
        .some((el) => !(el as HTMLMediaElement).paused),
    ),
    { timeout: 5_000, intervals: [50, 100, 250] },
  ).toBe(false);

  const orphanedPlaying = await page.evaluate(() =>
    (window.__playedMedia || []).filter((el) => !el.paused).length,
  );
  expect(orphanedPlaying).toBe(0);
}

/** After gallery prev/next, the prior file's element must pause even if the new preview autoplays. */
async function expectTrackedMediaPausedBeforeIndex(page: Page, endIndex: number) {
  await expect.poll(
    () =>
      page.evaluate((end) => {
        const tracked = window.__playedMedia || [];
        if (end > 0 && tracked.length < end) {
          return false;
        }
        for (let i = 0; i < end; i += 1) {
          if (!tracked[i].paused) {
            return false;
          }
        }
        return true;
      }, endIndex),
    { timeout: 10_000, intervals: [50, 100, 250, 500] },
  ).toBe(true);
}

test("closing the preview stops video playback", async ({ page, checkForErrors }) => {
  test.setTimeout(45_000);
  const mediaFailures = trackMediaApiFailures(page);
  await trackPlayedMedia(page);

  await page.goto(`${MEDIA}/clip-b.webm`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - clip-b.webm");
  await startPlayback(page);

  await page.locator('button[aria-label="Close"]').first().click();
  await expect(page).toHaveTitle(LISTING_TITLE);
  await expectNoMediaPlaying(page);

  checkForErrors(0, mediaFailures());
});

test("navigating back stops video playback", async ({ page, checkForErrors }) => {
  test.setTimeout(45_000);
  const mediaFailures = trackMediaApiFailures(page);
  await trackPlayedMedia(page);

  await page.goto(`${MEDIA}/`);
  await expect(page).toHaveTitle(LISTING_TITLE);
  await page.locator('a[aria-label="clip-b.webm"]').dblclick();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - clip-b.webm");
  await startPlayback(page);

  await page.goBack();
  await expect(page).toHaveTitle(LISTING_TITLE);
  await expectNoMediaPlaying(page);

  checkForErrors(0, mediaFailures());
});

test("previous navigation stops video playback", async ({ page, checkForErrors }) => {
  test.setTimeout(45_000);
  const mediaFailures = trackMediaApiFailures(page);
  await trackPlayedMedia(page);

  await page.goto(`${MEDIA}/clip-b.webm`);
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - clip-b.webm");
  await startPlayback(page);
  const playedBeforeNav = await page.evaluate(() => window.__playedMedia?.length ?? 0);
  expect(playedBeforeNav).toBeGreaterThan(0);

  await page.locator('button[aria-label="Previous"]').click();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - clip-a.webm");
  await expectTrackedMediaPausedBeforeIndex(page, playedBeforeNav);

  checkForErrors(0, mediaFailures());
});
