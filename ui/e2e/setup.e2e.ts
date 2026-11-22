// The first device of a fresh server: the one-time setup link from the server log makes it the admin. Its
// cookies are what the other specs' paired devices start from.
import { expect, test } from "@playwright/test";
import { adminState, closeDevices, device, shot } from "./helpers";

test.afterEach(closeDevices);

test("first device: the setup link from the server log makes the admin", async ({
  browser,
}) => {
  const page = await device(browser, "laptop");
  // before setup the app points to the link in the log, and offers no game
  await page.goto("./");
  await expect(page.getByText("Reworm isn't set up yet")).toBeVisible();
  await expect(page.getByText(/docker logs/)).toBeVisible();
  await expect(page.getByRole("button", { name: /^Host/ })).toHaveCount(0);
  await shot(page, "home-fresh");
  await page.goto("host");
  await expect(page.getByText("Reworm isn't set up yet")).toBeVisible();
  await page.goto(process.env.E2E_SETUP_PATH!);
  await expect(page.getByText("Welcome to Reworm")).toBeVisible();
  await page.getByLabel("Your name").fill("Jan");
  await page.getByRole("radio", { name: /Vinyl Vince/ }).click();
  await shot(page, "setup");
  await page.getByRole("button", { name: "Create my profile" }).click();
  await expect(page).toHaveURL(/\/reworm\/$/);
  await expect(page.getByText("Solo training")).toBeVisible();
  await shot(page, "home-admin");
  await page.context().storageState({ path: adminState() });
  // the old Host / Join chooser: a hard load of /party lands on home
  await page.goto("party");
  await expect(page).toHaveURL(/\/reworm\/$/);
  await expect(page.getByText("Play with friends")).toBeVisible();

  // under PUBLIC_PATH: the device cookie is scoped to the prefix, and a hard reload on a deep route loads the
  // bundle, icon and manifest from below the prefix (not the shell posing as each)
  const cookies = await page.context().cookies();
  expect(cookies.find((c) => c.name === "reworm_device")?.path).toBe("/reworm/");
  const assets: { url: string; type: string }[] = [];
  page.on("response", (r) => {
    if (r.url().includes("/assets/"))
      assets.push({ url: r.url(), type: r.headers()["content-type"] ?? "" });
  });
  await page.goto("train");
  await page.reload();
  await expect(page.getByText("New session")).toBeVisible();
  expect(assets.length).toBeGreaterThan(0);
  for (const a of assets) {
    expect(a.url).toContain("/reworm/assets/");
    expect(a.type).not.toContain("text/html");
  }
  for (const [sel, type] of [
    ['link[rel="manifest"]', "application/manifest+json"],
    ['link[rel="icon"]', "image/svg+xml"],
  ]) {
    const href = await page.locator(sel).getAttribute("href");
    expect(href).toMatch(/^\/reworm\//);
    const r = await page.request.get(href!);
    expect(r.status()).toBe(200);
    expect(r.headers()["content-type"]).toContain(type);
  }

  // the link works once
  const again = await device(browser, "laptop");
  await again.goto(process.env.E2E_SETUP_PATH!);
  await again.getByLabel("Your name").fill("Eve");
  await again.getByRole("button", { name: "Create my profile" }).click();
  await expect(again.getByRole("alert")).toContainText("already used");
});
