import { Browser, firefox, expect, Page, FullConfig } from "@playwright/test";

// Perform authentication and store auth state
async function localSetup() {
  const browser: Browser = await firefox.launch();
  const context = await browser.newContext();
  const page: Page = await context.newPage();

  await page.goto("http://localhost:8080/login");
  await page.getByPlaceholder("Username").fill("admin");
  await page.getByPlaceholder("Password").fill("admin");
  await page.getByRole("button", { name: "Login" }).click();
  
  // Wait for the login request to complete and cookie to be set
  await page.waitForResponse(
    (response) => response.url().includes("/api/auth/login") && response.status() === 200,
    { timeout: 5000 }
  );
  
  await page.waitForURL("**/files/", { timeout: 5000 });

  // Get cookies for the specific URL
  const cookies = await context.cookies("http://localhost:8080");
  expect(cookies.find((c) => c.name === "filebrowser_quantum_jwt")?.value).toBeDefined();

  // click acknowledgement button if prompt exists
  try {
    await page.locator('div[aria-label="generic-prompt"]').waitFor({ state: 'visible', timeout: 3000 });
    await page.locator('button[aria-label="Acknowledge"]').click();
    console.log("Clicked acknowledgement button");
  } catch (error) {
    console.log("No acknowledgement prompt appeared");
  }

  await context.storageState({ path: "./loginAuth.json" });
  await browser.close();
}

export default localSetup;