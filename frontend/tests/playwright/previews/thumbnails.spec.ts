import type { Page } from "@playwright/test";
import { expect, test, ensureListingViewMode } from "../test-setup.ts";

const FILES_DIR = "/files/playwright-files/files/";
const REMOTE_IMAGE_ALT = "Remote screenshot from filebrowserquantum.com";

/** Drive the StatusBar size slider so the gallery tiles are wide enough for a text preview. */
async function setGallerySize(page: Page, size: number): Promise<void> {
  await page.locator("#gallery-size").evaluate((el, value) => {
    const input = el as HTMLInputElement;
    input.value = String(value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.dispatchEvent(new Event("change", { bubbles: true }));
  }, size);
}

/** Text previews only render inline once the tile is wide enough; gallery does that. */
async function showTextPreviewGallery(page: Page, path: string): Promise<void> {
  await page.goto(path);
  await setGallerySize(page, 9);
  await ensureListingViewMode(page, "gallery");
}

// Thumbnails of HTML files must not show images or videos. The full viewer still shows them.

test("html thumbnail in the listing shows text but no images or videos", async ({ page }) => {
  await showTextPreviewGallery(page, FILES_DIR);
  const tile = page.locator('a[aria-label="index.html"] .text-preview');
  await tile.waitFor({ state: "visible" });
  await expect(tile).toContainText("Relative path resolution");
  await expect(tile.locator("img, image, picture, video")).toHaveCount(0);
  await expect(tile.locator("[srcset], [background]")).toHaveCount(0);
  // The truncation fade was removed; truncated content is clipped instead.
  await expect(tile.locator(".text-preview-fade")).toHaveCount(0);
});

test("html thumbnail for a page with a remote image omits the image", async ({ page }) => {
  await showTextPreviewGallery(page, `${FILES_DIR}nested/`);
  const tile = page.locator('a[aria-label="child.html"] .text-preview');
  await tile.waitFor({ state: "visible" });
  await expect(tile).toContainText("Child page");
  await expect(tile.locator("img, image, picture, video")).toHaveCount(0);
  await expect(tile.locator(`img[alt="${REMOTE_IMAGE_ALT}"]`)).toHaveCount(0);
});

test("small tiles show a preview-available indicator instead of the text preview", async ({ page }) => {
  await page.goto(FILES_DIR);
  await ensureListingViewMode(page, "list");

  const item = page.locator('a[aria-label="index.html"]');
  // The tile is too small to render text, so the icon plus a badge is shown instead.
  await expect(item.locator(".text-preview")).toHaveCount(0);
  await expect(item.locator(".overlay-icon--preview-hint")).toHaveCount(1);
});

test("hovering a small tile still opens the text preview popup", async ({ page }) => {
  await page.goto(FILES_DIR);
  await ensureListingViewMode(page, "list");

  const item = page.locator('a[aria-label="index.html"]');
  // The popup handler lives on the small .image-preview Icon, not the full-width list row,
  // so hover the first preview icon (the quick-download icon can add a second one).
  await item.locator(".image-preview").first().hover();
  const popup = page.locator(".popup-preview .text-preview");
  await popup.waitFor({ state: "visible" });
  await expect(popup).toContainText("Relative path resolution");
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
