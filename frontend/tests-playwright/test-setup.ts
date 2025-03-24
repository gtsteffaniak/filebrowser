import { test as base, expect, Page } from "@playwright/test";

export const test = base.extend<{ checkForErrors: () => void }>({
  checkForErrors: async ({ page }, use) => {
    const { checkForErrors } = setupErrorTracking(page);
    await use(checkForErrors);
  },
});

// Error tracking function
export function setupErrorTracking(page: Page) {
  const errors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") {
      errors.push(message.text());
    }
  });

  return {
    checkForErrors: () => {
      if (errors.length > 0) {
        console.error("Console Errors Detected:", errors);
      }
      expect(errors).toHaveLength(0);
    },
  };
}

export { expect };
