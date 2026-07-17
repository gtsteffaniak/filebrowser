import { writeFile } from "node:fs/promises";
import type { Browser, Page } from "@playwright/test";
import { expect, firefox } from "@playwright/test";
import {
  getOrCreateShareViaApi,
} from "./test-setup";

// Perform authentication and store auth state
async function globalSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  await page.goto("http://127.0.0.1/login");
  await page.getByPlaceholder("Username").fill("admin");
  await page.getByPlaceholder("Password").fill("admin");
  await page.getByRole("button", { name: "Login" }).click();
  await page.waitForURL("**/files/", { timeout: 1000 });

  const cookies = await context.cookies();
  expect(cookies.find((c) => c.name === "filebrowser_quantum_jwt")?.value).toBeDefined();
  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  await page.waitForURL("**/files/playwright%20+%20files/", { timeout: 1000 });

  await expect(page).toHaveTitle("Graham's Filebrowser - Files - playwright-files");

  const source = "playwright + files";

  const shareHash = await getOrCreateShareViaApi(page, {
    path: "/myfolder/",
    source,
    allowCreate: true,
    allowModify: true,
  });

  const shareHashFile = await getOrCreateShareViaApi(page, {
    path: "/1file1.txt",
    source,
  });

  const rootShareHash = await getOrCreateShareViaApi(page, {
    path: "/",
    source,
    allowCreate: true,
    allowModify: true,
  });

  // Authenticated sharing tests read these from loginAuth.json localStorage.
  await page.evaluate(
    ({ shareHash, shareHashFile, rootShareHash }) => {
      localStorage.setItem("shareHash", shareHash);
      localStorage.setItem("shareHashFile", shareHashFile);
      localStorage.setItem("rootShareHash", rootShareHash);
    },
    { shareHash, shareHashFile, rootShareHash },
  );

  // Anonymous share tests: same share hashes in localStorage, no JWT cookies.
  await writeFile(
    "./sharePrepStorage.json",
    JSON.stringify(
      {
        cookies: [],
        origins: [
          {
            origin: "http://127.0.0.1",
            localStorage: [
              { name: "shareHash", value: shareHash },
              { name: "shareHashFile", value: shareHashFile },
              { name: "rootShareHash", value: rootShareHash },
            ],
          },
        ],
      },
      null,
      2,
    ),
    "utf-8",
  );

  await context.storageState({ path: "./loginAuth.json" });
  await browser.close();
}

export default globalSetup;
