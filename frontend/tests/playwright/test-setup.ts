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
  // Wait a bit for notifications to appear (they might be added asynchronously)
  await page.waitForTimeout(100);

  // Wait for notifications container to be available
  const container = page.locator('#notifications-container');
  await container.waitFor({ state: 'attached', timeout: 5000 }).catch(() => {
    // Container might not exist if no notifications
  });

  // Wait for at least one notification to appear (with timeout)
  const notificationItems = page.locator('.notification-item');
  try {
    await notificationItems.first().waitFor({ state: 'visible', timeout: 5000 });
  } catch {
    // If no notifications appear, continue to error handling below
  }

  const count = await notificationItems.count();

  if (count === 0) {
    // Get all notification messages to provide helpful error
    const allMessages = await page.locator('.notification-message').allTextContents();
    const errorMessage = allMessages.length === 0
      ? 'No notifications found on the page.'
      : `No notifications found. Current notifications: ${JSON.stringify(allMessages)}`;
    throw new Error(`Notification with message "${message}" not found. ${errorMessage}`);
  }

  // Check each notification for the message
  for (let i = 0; i < count; i++) {
    const notificationItem = notificationItems.nth(i);
    const messageElement = notificationItem.locator('.notification-message');

    // Wait for the message to be visible
    try {
      await messageElement.waitFor({ state: 'visible', timeout: 2000 });
    } catch {
      // Continue to next notification if this one isn't visible
      continue;
    }

    const text = await messageElement.textContent();

    if (text) {
      if (typeof message === 'string') {
        if (text.includes(message)) {
          return messageElement;
        }
      } else {
        // RegExp
        if (message.test(text)) {
          return messageElement;
        }
      }
    }
  }

  // If we get here, no notification matched
  const allMessages = await page.locator('.notification-message').allTextContents();
  const errorMessage = `Notification with message "${message}" not found. Current notifications: ${JSON.stringify(allMessages)}`;
  throw new Error(errorMessage);
}

export { expect };
