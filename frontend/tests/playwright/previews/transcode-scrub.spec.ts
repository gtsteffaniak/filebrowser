import type { Page } from "@playwright/test";
import { expect, test } from "../test-setup.ts";
import { enableTranscodeUi } from "./transcode-helpers.ts";

const TESTDATA = "/files/playwright-files/myfolder/testdata";

/**
 * Real-backend transcode scrubbing validation. Requires ffmpeg on the server;
 * the CI image ships none, so the spec skips itself when media is unavailable.
 * Verifies what mocked sessions cannot: a real hls.js stream that seeks
 * forward, backward, and repeatedly while playback keeps running without a
 * manual play() after each reposition.
 */

/** Capture the server's real mediaAvailable before enableTranscodeUi patches it. */
async function captureRealMediaAvailable(page: Page) {
  await page.addInitScript(() => {
    const original = Document.prototype.getElementById;
    Document.prototype.getElementById = function patchedGetElementById(id) {
      const el = original.call(this, id);
      if (id === "app-config" && el && el.textContent && window.__realMediaAvailable === undefined) {
        try {
          window.__realMediaAvailable = JSON.parse(el.textContent).mediaAvailable === true;
        } catch (_) {
          window.__realMediaAvailable = false;
        }
      }
      return el;
    };
  });
}

async function videoState(page: Page) {
  return page.locator(".plyr-viewer video").first().evaluate((el: HTMLVideoElement) => ({
    currentTime: el.currentTime,
    paused: el.paused,
    ended: el.ended,
    readyState: el.readyState,
  }));
}

async function seekTo(page: Page, target: number) {
  await page.locator(".plyr-viewer video").first().evaluate(
    (el: HTMLVideoElement, t) => { el.currentTime = t; },
    target,
  );
}

async function expectPlayingNear(page: Page, target: number, tolerance = 1.5) {
  // Playback must resume on its own after the seek — no manual unpause.
  await expect.poll(
    async () => {
      const s = await videoState(page);
      return !s.paused && !s.ended && Math.abs(s.currentTime - target) <= tolerance;
    },
    { timeout: 30_000, intervals: [250, 500, 1000] },
  ).toBe(true);
}

test("transcode playback survives forward, backward, and repeated scrubbing", async ({ page }) => {
  test.setTimeout(180_000);
  await captureRealMediaAvailable(page);
  await enableTranscodeUi(page);

  let sessionStarted = false;
  page.on("response", (response) => {
    if (
      response.url().includes("/api/media/transcode/sessions")
      && response.request().method() === "POST"
      && (response.status() === 200 || response.status() === 201)
    ) {
      sessionStarted = true;
    }
  });

  await page.goto(`${TESTDATA}/file_example_WMV_480_1_2MB.wmv?transcode=quality`);
  const video = page.locator(".plyr-viewer video").first();
  await expect(video).toBeVisible({ timeout: 15_000 });

  const mediaAvailable = await page.evaluate(() => window.__realMediaAvailable === true);
  if (!mediaAvailable) {
    test.skip(true, "ffmpeg unavailable on this server; real transcode cannot run");
  }

  // A real session must be created — proves the backend accepted the request.
  await expect.poll(() => sessionStarted, { timeout: 30_000 }).toBe(true);

  // Playback starts and advances.
  await expect.poll(
    async () => {
      const s = await videoState(page);
      return s.currentTime > 0.5 && !s.paused;
    },
    { timeout: 60_000, intervals: [500, 1000] },
  ).toBe(true);

  // Forward skip: lands near the target and keeps playing.
  await seekTo(page, 20);
  await expectPlayingNear(page, 20);

  // Backward seek inside produced coverage: stays in place, keeps playing.
  await seekTo(page, 6);
  await expectPlayingNear(page, 6);

  // Repeated scrubbing: alternating targets must each resolve without a stall.
  for (const target of [15, 4, 22]) {
    await seekTo(page, target);
    await expectPlayingNear(page, target);
  }
});
