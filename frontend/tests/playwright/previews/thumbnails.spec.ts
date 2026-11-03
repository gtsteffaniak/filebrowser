import { expect, test } from "../test-setup.ts";

const FILES_DIR = "/files/playwright-files/files/";
const REMOTE_IMAGE_ALT = "Remote screenshot from filebrowserquantum.com";

// Thumbnails of HTML files must not show images or videos. The full viewer still shows them.

test("html thumbnail in the listing shows text but no images or videos", async ({ page }) => {
  await page.goto(FILES_DIR);
  const tile = page.locator('a[aria-label="index.html"] .text-preview');
  await tile.waitFor({ state: "visible" });
  await expect(tile).toContainText("Relative path resolution");
  await expect(tile.locator("img, image, picture, video")).toHaveCount(0);
  await expect(tile.locator("[srcset], [background]")).toHaveCount(0);
});

test("html thumbnail for a page with a remote image omits the image", async ({ page }) => {
  await page.goto(`${FILES_DIR}nested/`);
  const tile = page.locator('a[aria-label="child.html"] .text-preview');
  await tile.waitFor({ state: "visible" });
  await expect(tile).toContainText("Child page");
  await expect(tile.locator("img, image, picture, video")).toHaveCount(0);
  await expect(tile.locator(`img[alt="${REMOTE_IMAGE_ALT}"]`)).toHaveCount(0);
});

test("full html viewer shows relative images and videos", async ({ page }) => {
  await page.goto(`${FILES_DIR}index.html`);
  const frame = page.frameLocator("iframe.html-content");

  const image = frame.locator('img[alt="Loaded from a relative path"]');
  await expect(image).toHaveCount(1);
  await expect
    .poll(() => image.evaluate((el: HTMLImageElement) => el.naturalWidth), { timeout: 15_000 })
    .toBeGreaterThan(0);

  const video = frame.locator("video");
  await expect(video).toHaveCount(1);
  // The src is the media stream URL, so the filename is followed by query parameters.
  await expect(video).toHaveAttribute("src", /file=%2Fmyfolder%2Ftestdata%2Fsample-640x480\.mp4(&|$)/);
});

test("full html viewer shows the remote image", async ({ page }) => {
  // The remote host is stubbed so the test does not need network access.
  const png = Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
    "base64",
  );
  await page.route("https://filebrowserquantum.com/**", (route) =>
    route.fulfill({ status: 200, contentType: "image/png", body: png }),
  );
  await page.goto(`${FILES_DIR}nested/child.html`);
  const frame = page.frameLocator("iframe.html-content");

  const image = frame.locator(`img[alt="${REMOTE_IMAGE_ALT}"]`);
  await expect(image).toHaveCount(1);
  await expect
    .poll(() => image.evaluate((el: HTMLImageElement) => el.naturalWidth), { timeout: 15_000 })
    .toBeGreaterThan(0);
});
