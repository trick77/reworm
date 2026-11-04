// The first device of a fresh server: the one-time setup link from the server log makes it the admin. Its
// cookies are what the other specs' paired devices start from.
import { expect, test } from "@playwright/test";
import { adminState, closeDevices, device, shot } from "./helpers";

test.afterEach(closeDevices);

test("first device: the setup link from the server log makes the admin", async ({
  browser,
}) => {
  const page = await device(browser, "laptop");
  await page.goto(process.env.E2E_SETUP_PATH!);
  await expect(page.getByText("Welcome to Reworm")).toBeVisible();
  await page.getByLabel("Your name").fill("Jan");
  await page.getByRole("radio", { name: /Vinyl Vince/ }).click();
  await shot(page, "setup");
  await page.getByRole("button", { name: "Create my profile" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByText("Today's round")).toBeVisible();
  await shot(page, "home-admin");
  await page.context().storageState({ path: adminState() });

  // the link works once
  const again = await device(browser, "laptop");
  await again.goto(process.env.E2E_SETUP_PATH!);
  await again.getByLabel("Your name").fill("Eve");
  await again.getByRole("button", { name: "Create my profile" }).click();
  await expect(again.getByRole("alert")).toContainText("already used");
});
