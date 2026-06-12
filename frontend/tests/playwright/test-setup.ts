import type { Locator, Page } from "@playwright/test";
import { test as base, expect } from "@playwright/test";

const PLAYWRIGHT_RETRY_INTERVALS = [500, 1000, 1500, 2000];

async function dismissSharePrompt(page: Page, sharePrompt: Locator): Promise<void> {
  if (!(await sharePrompt.isVisible())) {
    return;
  }

  for (let attempt = 0; attempt < 3; attempt++) {
    await page.keyboard.press("Escape");
    try {
      await sharePrompt.waitFor({ state: "hidden", timeout: 2000 });
      return;
    } catch {
      // Retry Escape or fall through to the close button.
    }
  }

  const closeButton = sharePrompt.locator(".prompt-close");
  if (await closeButton.isVisible()) {
    await closeButton.click();
    await sharePrompt.waitFor({ state: "hidden", timeout: 3000 }).catch(() => {});
  }
}

/** Closes the share dialog when it is still open after creating or viewing a share. */
export async function closeSharePromptIfOpen(page: Page): Promise<void> {
  await dismissSharePrompt(page, page.locator("div[aria-label='share-prompt']"));
}

/** Closes the file-actions / listing context menu if it is still open. */
export async function closeContextMenuIfOpen(page: Page): Promise<void> {
  const contextMenu = page.locator("#context-menu");
  for (let attempt = 0; attempt < 3; attempt++) {
    if (!(await contextMenu.isVisible())) {
      return;
    }
    await page.keyboard.press("Escape");
    await contextMenu.waitFor({ state: "hidden", timeout: 2000 }).catch(() => {});
  }
}

async function waitForContextMenuReady(page: Page, contextMenu: Locator): Promise<void> {
  await contextMenu.waitFor({ state: "visible", timeout: 5000 });
  await page.waitForFunction(() => {
    const menus = document.querySelectorAll("#context-menu");
    const menu = menus[menus.length - 1];
    if (!(menu instanceof HTMLElement)) {
      return false;
    }
    const rect = menu.getBoundingClientRect();
    return rect.height > 40 && rect.width > 40 && menu.style.opacity !== "0";
  }, { timeout: 5000 });
}

/**
 * Standalone helper function to open the context menu (File-Actions button)
 * Can be used in both test fixtures and global setup
 */
export async function openContextMenuHelper(
  page: Page,
  options?: { timeout?: number },
): Promise<void> {
  const timeout = options?.timeout ?? 30000;

  await expect(async () => {
    await closeContextMenuIfOpen(page);
    await closeSharePromptIfOpen(page);

    const readyMarker = page.locator('[data-testid="file-actions-ready"]');

    try {
      await readyMarker.waitFor({ state: "attached", timeout: 5000 });
    } catch (error: unknown) {
      const originalMessage = error instanceof Error ? error.message : String(error);
      throw new Error(
        `File actions are not available on this page. Check that you are on a listing view with appropriate permissions. Original error: ${originalMessage}`,
      );
    }

    const isHidden = await readyMarker.getAttribute("data-hidden");
    if (isHidden === "true") {
      throw new Error(
        "File actions button is hidden (user does not have create permissions or is on invalid share)",
      );
    }

    const fileActionsButton = page.locator('[data-testid="file-actions-button"]');
    await fileActionsButton.waitFor({ state: "visible", timeout: 5000 });
    await fileActionsButton.click();
  }).toPass({ timeout, intervals: PLAYWRIGHT_RETRY_INTERVALS });
}

/**
 * Opens the sidebar File-Actions menu and clicks Share once the context menu is ready.
 */
export async function openShareFromFileActions(page: Page): Promise<void> {
  await closeSharePromptIfOpen(page);
  await closeContextMenuIfOpen(page);
  await openContextMenuHelper(page);

  const contextMenu = page.locator("#context-menu").last();
  await waitForContextMenuReady(page, contextMenu);

  const shareButton = contextMenu.locator('button[aria-label="Share"]');
  await shareButton.waitFor({ state: "visible", timeout: 5000 });
  await shareButton.scrollIntoViewIfNeeded();

  const sharePrompt = page.locator("div[aria-label='share-prompt']");
  const shareListRequest = page.waitForResponse(
    (response) =>
      response.url().includes("/api/share") &&
      response.request().method() === "GET" &&
      response.ok(),
    { timeout: 10000 },
  );

  await shareButton.click();
  await shareListRequest;
  await sharePrompt.waitFor({ state: "visible", timeout: 8000 });
}

/**
 * Creates a share via the authenticated API (for global setup data prep).
 */
export async function createShareViaApi(
  page: Page,
  options: {
    path: string;
    source: string;
    allowCreate?: boolean;
    allowModify?: boolean;
  },
): Promise<string> {
  const response = await page.request.post("http://127.0.0.1/api/share", {
    headers: { "Content-Type": "application/json" },
    data: {
      path: options.path,
      source: options.source,
      allowCreate: options.allowCreate ?? false,
      allowModify: options.allowModify ?? false,
      allowDelete: false,
      shareType: "normal",
      expires: "",
      unit: "hours",
      hash: "",
      sidebarLinks: [
        {
          name: "Share QR Code and Info",
          category: "shareInfo",
          target: "#",
          icon: "qr_code",
        },
        {
          name: "Download",
          category: "download",
          target: "#",
          icon: "download",
        },
      ],
    },
  });

  if (!response.ok()) {
    throw new Error(`Failed to create share: ${response.status()} ${await response.text()}`);
  }

  const body = await response.json() as { hash?: string };
  if (!body.hash) {
    throw new Error("Share API response missing hash");
  }
  return body.hash;
}

/**
 * Opens the share dialog and asserts the path, retrying on transient UI timing failures.
 */
export async function openShareAndExpectPath(
  page: Page,
  expectedPathText: string,
  openShare: () => Promise<void>,
  options?: { timeout?: number },
): Promise<void> {
  const timeout = options?.timeout ?? 30000;
  const sharePrompt = page.locator("div[aria-label='share-prompt']");
  const sharePath = sharePrompt.locator('[aria-label="share-path"]');

  await expect(async () => {
    await closeContextMenuIfOpen(page);

    if (await sharePrompt.isVisible()) {
      try {
        await sharePath.waitFor({ state: "visible", timeout: 1000 });
        const pathText = (await sharePath.textContent())?.trim();
        if (pathText === expectedPathText) {
          return;
        }
      } catch {
        // sharePath not ready yet, continue to dismiss and retry
      }
      await dismissSharePrompt(page, sharePrompt);
    }

    await openShare();
    await sharePrompt.waitFor({ state: "visible", timeout: 8000 });
    await sharePath.waitFor({ state: "visible", timeout: 8000 });
    await expect(sharePath).toHaveText(expectedPathText, { timeout: 5000 });
  }).toPass({ timeout, intervals: PLAYWRIGHT_RETRY_INTERVALS });
}

export const SHARE_PROMPT_ROWS =
  "div[aria-label='share-prompt'] .card-content table tbody tr:not(:has(th))";

/**
 * Opens the share dialog, confirms creation, and returns the hash — retried as one flow.
 */
export async function createShareAndGetHash(
  page: Page,
  expectedPathText: string,
  openShare: () => Promise<void>,
  options?: { timeout?: number },
): Promise<string> {
  const timeout = options?.timeout ?? 45000;
  const rows = page.locator(SHARE_PROMPT_ROWS);
  const confirmButton = page.locator('button[aria-label="Share-Confirm"]');
  const sharePrompt = page.locator("div[aria-label='share-prompt']");
  let shareHash = "";

  await expect(async () => {
    if ((await rows.count()) === 1) {
      const existingHash = (await rows.first().locator("td").first().textContent())?.trim();
      if (existingHash) {
        shareHash = existingHash;
        return;
      }
    }

    await dismissSharePrompt(page, sharePrompt);

    await openShareAndExpectPath(page, expectedPathText, openShare, { timeout: 20000 });

    await confirmButton.waitFor({ state: "visible", timeout: 5000 });
    await confirmButton.click();

    await expect(rows).toHaveCount(1);
    const hash = (await rows.first().locator("td").first().textContent())?.trim();
    if (!hash) {
      throw new Error("Share hash not yet available");
    }
    shareHash = hash;
  }).toPass({ timeout, intervals: PLAYWRIGHT_RETRY_INTERVALS });

  return shareHash;
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
    const theme = (testInfo.project.use as { theme?: 'light' | 'dark' }).theme || 'dark';
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
        } catch (_e) {
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

  // Track failed API calls (304 Not Modified is expected for cached preview requests)
  page.on("response", (response) => {
    const status = response.status();
    if (status === 304 || response.ok()) {
      return;
    }
    failedResponses.push({
      url: response.url(),
      status,
    });
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

  } catch (error: unknown) {
    // Handle page closed/navigation errors gracefully
    if (error instanceof Error && (error.message.includes('Target page') || error.message.includes('closed'))) {
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
    if (error instanceof Error && error.message.includes('waiting for')) {
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
      } catch (countError: unknown) {
        if (countError instanceof Error && (countError.message.includes('Target page') || countError.message.includes('closed'))) {
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

  } catch (error: unknown) {
    if (error instanceof Error && (error.message.includes('Target page') || error.message.includes('closed'))) {
      throw new Error(`Toast check failed: page was closed or navigated. Expected: "${message}"`);
    }

    if (error instanceof Error && error.message.includes('waiting for')) {
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
