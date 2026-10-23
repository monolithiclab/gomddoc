import { defineConfig } from "@playwright/test";

// End-to-end tests against a real `gomddoc preview` of e2e/fixture/ (D48).
// The system Chrome runs them: no browser download.
export default defineConfig({
  testDir: "e2e",
  globalSetup: "./e2e/global-setup.mjs",
  workers: 1,
  retries: 0,
  reporter: "line",
  timeout: 10_000,
  use: { trace: "off" },
  projects: [{ name: "chrome", use: { channel: "chrome" } }],
});
