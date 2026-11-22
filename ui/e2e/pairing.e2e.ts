// Profiles (D3): "Add a device" logs a second browser in as the admin's profile; anyone else makes their own
// profile right where hosting asks for one and goes on into the lobby. Profiles are private: no list, no switching.
import { expect, test, type Page } from "@playwright/test";
import { closeDevices, device, shot } from "./helpers";

test.afterEach(closeDevices);

// pairCode opens a device code on the profile page and reads it off the panel.
async function pairCode(admin: Page): Promise<string> {
  await admin.getByRole("button", { name: "Add a device" }).click();
  const panel = admin.getByRole("dialog");
  const code = panel.locator(".num");
  await expect(code).toHaveText(/^[A-Z0-9]{6}$/);
  return (await code.textContent())!;
}

test("profiles: add a device, a stranger makes a profile on /host and hosts", async ({
  browser,
}) => {
  const admin = await device(browser, "laptop", { admin: true });
  await admin.goto("me");
  await expect(admin.getByText("Jan", { exact: true }).first()).toBeVisible();
  await expect(admin.getByText(/· admin$/)).toBeVisible();
  await expect(admin.getByText(/invite/i)).toHaveCount(0);

  // Add a device: the code claimed on a second browser logs it in as Jan
  const code = await pairCode(admin);
  await shot(admin, "pair-code");
  const second = await device(browser, "phone");
  await second.goto(`pair/${code}`);
  await second.getByRole("button", { name: "Use this code" }).click();
  await expect(second).toHaveURL(/\/reworm\/$/);
  await expect(second.getByText("Solo training")).toBeVisible();
  await second.goto("me");
  await expect(second.getByText(/· admin$/)).toBeVisible();
  // a used code is gone
  const third = await device(browser, "phone");
  await third.goto(`pair/${code}`);
  await third.getByRole("button", { name: "Use this code" }).click();
  await expect(third.getByRole("alert")).toContainText("unknown or expired");

  // A stranger: name and avatar right where hosting asks for a profile, then on into the lobby
  await third.goto("host");
  await expect(third.getByText("Create a profile to host")).toBeVisible();
  const name = `Guest${test.info().project.name === "webkit" ? "W" : "C"}`;
  await third.getByLabel("Your name").fill(name);
  await third.getByRole("radio", { name: "Quaver" }).click();
  await shot(third, "host-create-profile");
  await third.getByRole("button", { name: "Create my profile" }).click();
  await expect(third.locator('[aria-label="Room code"]')).toHaveText(
    /^[A-Z0-9]{4}$/,
  );
  await expect(third).toHaveURL(/\/reworm\/room\/[A-Z0-9]{4}$/);
  await third.goto("me");
  await expect(third.getByText(name, { exact: true }).first()).toBeVisible();
  await expect(third.getByText(/· admin$/)).toHaveCount(0);
  // private: Jan is nowhere on the stranger's profile page
  await expect(third.getByText("Jan", { exact: true })).toHaveCount(0);

  // another profile on the stranger's only device: asked first, kept on "Keep"
  await third
    .getByRole("button", { name: "Use another profile on this device" })
    .click();
  await third
    .getByRole("button", { name: "New here? Create a profile" })
    .click();
  await third.getByLabel("Your name").fill("Other");
  await third.getByRole("button", { name: "Create my profile" }).click();
  await expect(third.getByRole("alert")).toContainText(
    `This is ${name}'s only device.`,
  );
  await shot(third, "leave-confirm");
  await third.getByRole("button", { name: `Keep ${name}` }).click();
  await third.goto("me");
  await expect(third.getByText(name, { exact: true }).first()).toBeVisible();
});
