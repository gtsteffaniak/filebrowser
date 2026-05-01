import { test, expect } from "../test-setup";

const jpgQuality = 85;

// Filename ends with `screenshots.spec.ts` so legacy Playwright configs that only match that pattern still pick up this test (Docker screenshot builds).

test("upload prompt", async ({ page, checkForErrors, openContextMenu, theme }) => {
  test.setTimeout(45_000);

  if (theme === "light") {
    return;
  }

  // Screenshots Docker uses a single "playwright" source (see _docker/src/screenshots/backend/config.yaml).
  await page.goto("/files/");
  await expect(page).toHaveTitle("FileBrowser Quantum - Files - playwright-files");
  await openContextMenu();

  // Context Action uses :label="$t('general.upload')" → accessible name can include suffix placeholders; match loosely.
  await page.locator("#context-menu").getByRole("button", { name: /Upload/i }).click();

  // Prompt shell sets aria-label="upload-prompt" (see Prompts.vue: prompt.name + '-prompt').
  const uploadPrompt = page.locator('.floating-window[aria-label="upload-prompt"]');
  await uploadPrompt.waitFor({ state: "visible", timeout: 15_000 });
  await page.waitForTimeout(400);

  await uploadPrompt.screenshot({
    path: `./generated/prompts/upload-${theme}.jpg`,
    type: "jpeg",
    quality: jpgQuality,
  });
  checkForErrors();
});
