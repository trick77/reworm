// End-to-end tests against the real binary (npm run e2e, make e2e; not part of the gate). One server for the
// run (global-setup.ts), one worker: every device shares the client address the server rate-limits on.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: /.*\.e2e\.ts$/,
  globalSetup: "./global-setup.ts",
  outputDir: "../test-results/e2e",
  workers: 1,
  fullyParallel: false,
  forbidOnly: true,
  retries: 0,
  timeout: 180_000,
  expect: { timeout: 10_000 },
  reporter: [["list"]],
  use: { trace: "retain-on-failure", screenshot: "only-on-failure" },
  projects: [
    // the first device's setup, once: the browser projects start from its admin cookies
    {
      name: "setup",
      testMatch: /setup\.e2e\.ts$/,
      use: { browserName: "chromium" },
    },
    {
      name: "chromium",
      testIgnore: /setup\.e2e\.ts$/,
      dependencies: ["setup"],
      use: {
        browserName: "chromium",
        // the stage plays clips without a tap of its own, as after the host's Start tap on a real device
        launchOptions: { args: ["--autoplay-policy=no-user-gesture-required"] },
      },
    },
    {
      name: "webkit",
      testIgnore: /setup\.e2e\.ts$/,
      dependencies: ["setup"],
      use: { browserName: "webkit" },
    },
  ],
});
