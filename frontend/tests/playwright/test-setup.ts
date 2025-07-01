import { test as base, expect, Page } from "@playwright/test";

export const test = base.extend<{
  checkForErrors: (expectedConsoleErrors?: number, expectedApiErrors?: number) => void;
  openContextMenu: () => Promise<void>;
}>({
  checkForErrors: async ({ page }, use) => {
    const { checkForErrors } = setupErrorTracking(page);
    await use(checkForErrors);
  },
  openContextMenu: async ({ page }, use) => {
    await use(async () => {
      const listingView = await page.locator('#listingView');
      const box = await listingView.boundingBox();
      if (!box) throw new Error("Could not find listingView bounding box");
      const x = box.x + box.width / 2;
      const y = box.y + box.height - 1;
      await page.mouse.click(x, y, { button: "right" });
    });
  }
});

// Error tracking function
export function setupErrorTracking(page: Page) {
  const consoleErrors: string[] = [];
  const failedResponses: { url: string; status: number }[] = [];

  // Track console errors
  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });

  // Track failed API calls
  page.on("response", (response) => {
    if (!response.ok()) {
      failedResponses.push({
        url: response.url(),
        status: response.status(),
      });
    }
  });

  return {
    checkForErrors: (expectedConsoleErrors = 0, expectedApiErrors = 0) => {
      if (consoleErrors.length !== expectedConsoleErrors) {
        console.error("Unexpected Console Errors:", consoleErrors);
      }

      if (failedResponses.length !== expectedApiErrors) {
        console.error("Unexpected Failed API Calls:", failedResponses);
      }

      expect(consoleErrors).toHaveLength(expectedConsoleErrors);
      expect(failedResponses).toHaveLength(expectedApiErrors);
    },
  };
}



export { expect };
