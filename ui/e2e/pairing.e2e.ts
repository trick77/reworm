// Pairing (D3): "Add a device" logs a second browser in as the admin's profile, "Invite someone" makes a new
// profile, and nobody switches into the admin profile.
import { expect, test, type Page } from "@playwright/test";
import { closeDevices, device, shot } from "./helpers";

test.afterEach(closeDevices);

// pairCode opens a code on the profile page and reads it off the panel.
async function pairCode(admin: Page, button: string): Promise<string> {
  await admin.getByRole("button", { name: button }).click();
  const panel = admin.getByRole("dialog");
  const code = panel.locator(".num");
  await expect(code).toHaveText(/^[A-Z0-9]{6}$/);
  return (await code.textContent())!;
}

test("pairing: add a device, invite someone, the admin profile is not switchable", async ({
  browser,
}) => {
  const admin = await device(browser, "laptop", { admin: true });
  await admin.goto("/me");
  await expect(admin.getByText("Jan", { exact: true }).first()).toBeVisible();
  await expect(admin.getByText(/· admin$/)).toBeVisible();

  // Add a device: the code claimed on a second browser logs it in as Jan
  const code = await pairCode(admin, "Add a device");
  await shot(admin, "pair-code");
  const second = await device(browser, "phone");
  await second.goto(`/pair/${code}`);
  await second.getByRole("button", { name: "Pair this device" }).click();
  await expect(second).toHaveURL(/\/$/);
  await expect(second.getByText("Today's round")).toBeVisible();
  await second.goto("/me");
  await expect(second.getByText(/· admin$/)).toBeVisible();
  // a used code is gone
  const third = await device(browser, "phone");
  await third.goto(`/pair/${code}`);
  await third.getByRole("button", { name: "Pair this device" }).click();
  await expect(third.getByRole("alert")).toContainText("unknown or expired");

  // Invite someone: the code asks for a name and an avatar and makes a new profile
  await admin.getByRole("button", { name: "Done" }).click();
  const invite = await pairCode(admin, "Invite someone");
  await third.goto(`/pair/${invite}`);
  await third.getByRole("button", { name: "Pair this device" }).click();
  await expect(third.getByText("You're invited!")).toBeVisible();
  const name = `Guest${test.info().project.name === "webkit" ? "W" : "C"}`;
  await third.getByLabel("Your name").fill(name);
  await third.getByRole("radio", { name: "Quaver" }).click();
  await shot(third, "pair-invite");
  await third.getByRole("button", { name: "Create my profile" }).click();
  await expect(third.getByText("Today's round")).toBeVisible();
  await third.goto("/me");
  await expect(third.getByText(name, { exact: true }).first()).toBeVisible();
  await expect(third.getByText(/· admin$/)).toHaveCount(0);

  // Who's playing: switching into the admin profile is refused, the device stays itself
  await third.goto("/who");
  await third.getByRole("button", { name: "Jan" }).click();
  await expect(third.getByRole("alert")).toContainText(
    "Jan is the admin profile",
  );
  await shot(third, "who-refused");
  await expect(third).toHaveURL(/\/who$/);
  await third.goto("/me");
  await expect(third.getByText(name, { exact: true }).first()).toBeVisible();
});
