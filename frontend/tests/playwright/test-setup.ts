import { test as base, expect, Page } from "@playwright/test";

/**
 * Standalone helper function to open the context menu (File-Actions button)
 * Can be used in both test fixtures and global setup
 */
export async function openContextMenuHelper(page: Page): Promise<void> {
  await page.locator('button[aria-label="File-Actions"]').waitFor({ state: 'visible' });
  await page.locator('button[aria-label="File-Actions"]').click();
}

export const test = base.extend<{
  checkForErrors: (expectedConsoleErrors?: number, expectedApiErrors?: number) => void;
  openContextMenu: () => Promise<void>;
  theme: 'light' | 'dark';
  checkForNotification: (message: string | RegExp) => Promise<import('@playwright/test').Locator>;
}>({
  checkForErrors: async ({ page }, use) => {
    const { checkForErrors } = setupErrorTracking(page);
    await use(checkForErrors);
  },
  openContextMenu: async ({ page }, use) => {
    await use(async () => {
      await openContextMenuHelper(page);
    });
  },
  theme: async ({}, use, testInfo) => {
    const theme = (testInfo.project.use as any).theme || 'dark';
    await use(theme);
  },
  checkForNotification: async ({ page }, use) => {
    await use(async (message: string | RegExp) => {
      return await checkForNotification(page, message);
    });
  },
});

// Error tracking function
export function setupErrorTracking(page: Page) {
  const consoleErrors: string[] = [];
  const failedResponses: { url: string; status: number }[] = [];

  // Track console errors
  page.on("console", async (message) => {
    if (message.type() === "error") {
      const errorText = message.text();
      const args = message.args();

      // Try to extract more detailed error information
      let detailedError = errorText;

      if (args.length > 0) {
        try {
          // Get the first argument which usually contains the error object
          const firstArg = await args[0].jsonValue().catch(() => null);

          if (firstArg && typeof firstArg === 'object') {
            if (firstArg.stack) {
              // If we have a stack trace, use it
              detailedError = firstArg.stack;
            } else if (firstArg.message) {
              // Otherwise use the message if available
              detailedError = `${firstArg.name || 'Error'}: ${firstArg.message}`;
            }
          }
        } catch (e) {
          // If we can't extract detailed info, try to get string representation of args
          try {
            const argsText = await Promise.all(
              args.map(async (arg) => {
                try {
                  return await arg.evaluate((obj) => {
                    if (obj && typeof obj === 'object' && obj.stack) {
                      return obj.stack;
                    }
                    return String(obj);
                  });
                } catch {
                  return '[Unable to serialize]';
                }
              })
            );

            const combinedArgs = argsText.join(' ');
            if (combinedArgs.trim() && combinedArgs !== errorText) {
              detailedError = combinedArgs;
            }
          } catch {
            // Fallback to original text
            detailedError = errorText;
          }
        }
      }

      consoleErrors.push(detailedError);
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
        console.error(`\n=== Unexpected Console Errors (Expected: ${expectedConsoleErrors}, Got: ${consoleErrors.length}) ===`);
        consoleErrors.forEach((error, index) => {
          console.error(`\nError ${index + 1}:`);
          console.error(error);
          console.error('---');
        });
        console.error('=== End Console Errors ===\n');
      }

      if (failedResponses.length !== expectedApiErrors) {
        console.error(`\n=== Unexpected Failed API Calls (Expected: ${expectedApiErrors}, Got: ${failedResponses.length}) ===`);
        failedResponses.forEach((response, index) => {
          console.error(`\nFailed Request ${index + 1}: ${response.status} - ${response.url}`);
        });
        console.error('=== End Failed API Calls ===\n');
      }

      expect(consoleErrors).toHaveLength(expectedConsoleErrors);
      expect(failedResponses).toHaveLength(expectedApiErrors);
    },
  };
}



/**
 * Helper function to check for a notification or toast with the given message
 * @param page - Playwright page object
 * @param message - Expected message text (string or RegExp)
 * @returns Locator for the matching notification or toast message
 */
export async function checkForNotification(page: Page, message: string | RegExp): Promise<import('@playwright/test').Locator> {
  // Check both notifications and toasts
  const notificationMessage = page.locator('.notification-message');
  const toastMessage = page.locator('.toast-message');
  const allMessages = page.locator('.notification-message, .toast-message');

  try {
    // Wait for a notification or toast containing the message to appear
    let matchingMessage: import('@playwright/test').Locator | null = null;

    if (typeof message === 'string') {
      // For string matching, use text content filter
      matchingMessage = allMessages.filter({ hasText: message }).first();
    } else {
      // For RegExp, we need to check all and find the match
      // Wait for at least one notification or toast first
      await allMessages.first().waitFor({ state: 'visible', timeout: 5000 });

      // Then check all messages
      const count = await allMessages.count();
      for (let i = 0; i < count; i++) {
        const messageElement = allMessages.nth(i);
        const text = await messageElement.textContent();
        if (text && message.test(text)) {
          matchingMessage = messageElement;
          break;
        }
      }
    }

    if (matchingMessage) {
      // Wait for it to be visible (with retry logic)
      await matchingMessage.waitFor({ state: 'visible', timeout: 5000 });
      return matchingMessage;
    }

    // If no match found, get all messages for error reporting
    const [notificationTexts, toastTexts] = await Promise.all([
      notificationMessage.allTextContents(),
      toastMessage.allTextContents(),
    ]);
    const allTexts = {
      notifications: notificationTexts,
      toasts: toastTexts,
    };
    const errorMessage = `Message "${message}" not found. Current messages: ${JSON.stringify(allTexts)}`;
    throw new Error(errorMessage);

  } catch (error: any) {
    // Handle page closed/navigation errors gracefully
    if (error.message && (error.message.includes('Target page') || error.message.includes('closed'))) {
      // Try to get current messages before page closed
      try {
        const [notificationTexts, toastTexts] = await Promise.all([
          notificationMessage.allTextContents(),
          toastMessage.allTextContents(),
        ]);
        const allTexts = {
          notifications: notificationTexts,
          toasts: toastTexts,
        };
        throw new Error(`Message check failed: page was closed or navigated. Expected: "${message}". Found before page closed: ${JSON.stringify(allTexts)}`);
      } catch {
        throw new Error(`Message check failed: page was closed or navigated before message could be checked. Expected: "${message}"`);
      }
    }

    // If no messages found, provide helpful error
    if (error.message && error.message.includes('waiting for')) {
      try {
        const [notificationTexts, toastTexts] = await Promise.all([
          notificationMessage.allTextContents(),
          toastMessage.allTextContents(),
        ]);
        const allTexts = {
          notifications: notificationTexts,
          toasts: toastTexts,
        };
        const totalCount = notificationTexts.length + toastTexts.length;
        const errorMessage = totalCount === 0
          ? 'No notifications or toasts found on the page.'
          : `No matching message found. Current messages: ${JSON.stringify(allTexts)}`;
        throw new Error(`Message "${message}" not found. ${errorMessage}`);
      } catch (countError: any) {
        if (countError.message && (countError.message.includes('Target page') || countError.message.includes('closed'))) {
          throw error;
        }
        throw countError;
      }
    }

    throw error;
  }
}

/**
 * Helper function to check specifically for a toast message
 * Use this when you want to verify that a toast (not a notification) was shown
 * @param page - Playwright page object
 * @param message - Expected message text (string or RegExp)
 * @returns Locator for the matching toast message
 */
export async function checkForToast(page: Page, message: string | RegExp): Promise<import('@playwright/test').Locator> {
  const toastMessage = page.locator('.toast-message');

  try {
    let matchingToast: import('@playwright/test').Locator | null = null;

    if (typeof message === 'string') {
      matchingToast = toastMessage.filter({ hasText: message }).first();
    } else {
      await toastMessage.first().waitFor({ state: 'visible', timeout: 5000 });
      const count = await toastMessage.count();
      for (let i = 0; i < count; i++) {
        const toast = toastMessage.nth(i);
        const text = await toast.textContent();
        if (text && message.test(text)) {
          matchingToast = toast;
          break;
        }
      }
    }

    if (matchingToast) {
      await matchingToast.waitFor({ state: 'visible', timeout: 5000 });
      return matchingToast;
    }

    const allToasts = await toastMessage.allTextContents();
    throw new Error(`Toast with message "${message}" not found. Current toasts: ${JSON.stringify(allToasts)}`);

  } catch (error: any) {
    if (error.message && (error.message.includes('Target page') || error.message.includes('closed'))) {
      throw new Error(`Toast check failed: page was closed or navigated. Expected: "${message}"`);
    }

    if (error.message && error.message.includes('waiting for')) {
      const allToasts = await toastMessage.allTextContents().catch(() => []);
      const errorMessage = allToasts.length === 0
        ? 'No toasts found on the page.'
        : `No matching toast found. Current toasts: ${JSON.stringify(allToasts)}`;
      throw new Error(`Toast with message "${message}" not found. ${errorMessage}`);
    }

    throw error;
  }
}

export { expect };
