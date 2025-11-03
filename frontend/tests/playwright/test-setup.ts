import { test as base, expect, Page } from "@playwright/test";

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
      await page.locator('button[aria-label="File-Actions"]').waitFor({ state: 'visible' });
      await page.locator('button[aria-label="File-Actions"]').click();
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
 * Helper function to check for a notification with the given message
 * @param page - Playwright page object
 * @param message - Expected message text (string or RegExp)
 * @returns Locator for the matching notification message
 */
export async function checkForNotification(page: Page, message: string | RegExp): Promise<import('@playwright/test').Locator> {
  // Use Playwright's built-in waiting mechanism with a locator filter
  // This will automatically retry until the notification appears or timeout
  const notificationMessage = page.locator('.notification-message');

  try {
    // Wait for a notification containing the message to appear
    // Use Playwright's filter to find the matching notification
    let matchingNotification: import('@playwright/test').Locator | null = null;

    if (typeof message === 'string') {
      // For string matching, use text content filter
      matchingNotification = notificationMessage.filter({ hasText: message }).first();
    } else {
      // For RegExp, we need to check all and find the match
      // Wait for at least one notification first
      await notificationMessage.first().waitFor({ state: 'visible', timeout: 5000 });

      // Then check all notifications
      const count = await notificationMessage.count();
      for (let i = 0; i < count; i++) {
        const notification = notificationMessage.nth(i);
        const text = await notification.textContent();
        if (text && message.test(text)) {
          matchingNotification = notification;
          break;
        }
      }
    }

    if (matchingNotification) {
      // Wait for it to be visible (with retry logic)
      await matchingNotification.waitFor({ state: 'visible', timeout: 5000 });
      return matchingNotification;
    }

    // If no match found, get all messages for error reporting
    const allMessages = await notificationMessage.allTextContents();
    const errorMessage = `Notification with message "${message}" not found. Current notifications: ${JSON.stringify(allMessages)}`;
    throw new Error(errorMessage);

  } catch (error: any) {
    // Handle page closed/navigation errors gracefully
    if (error.message && (error.message.includes('Target page') || error.message.includes('closed'))) {
      // Try to get current notifications before page closed
      try {
        const allMessages = await notificationMessage.allTextContents();
        throw new Error(`Notification check failed: page was closed or navigated. Expected message: "${message}". Found notifications before page closed: ${JSON.stringify(allMessages)}`);
      } catch {
        throw new Error(`Notification check failed: page was closed or navigated before notification could be checked. Expected message: "${message}"`);
      }
    }

    // If no notifications found, provide helpful error
    if (error.message && error.message.includes('waiting for')) {
      try {
        const allMessages = await notificationMessage.allTextContents();
        const errorMessage = allMessages.length === 0
          ? 'No notifications found on the page.'
          : `No notifications found. Current notifications: ${JSON.stringify(allMessages)}`;
        throw new Error(`Notification with message "${message}" not found. ${errorMessage}`);
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

export { expect };
