import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests/browser",
  workers: 1,
  timeout: 60000,
  use: {
    headless: true,
    launchOptions: {
      args: [
        "--enable-experimental-web-platform-features",
        "--enable-features=WebMCP",
      ],
    },
  },
});
