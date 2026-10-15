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
