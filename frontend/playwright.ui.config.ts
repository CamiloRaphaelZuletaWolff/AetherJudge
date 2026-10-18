import { defineConfig } from "@playwright/test";

// Browser-level UX checks use deterministic API/WS fixtures; no judge stack required.
export default defineConfig({
  testDir: "./e2e/ui",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  workers: 1,
  reporter: "list",
  outputDir: "./test-results/ui",
  use: { baseURL: "http://localhost:3100", trace: "retain-on-failure" },
  webServer: {
    command: "node node_modules/next/dist/bin/next dev --turbopack --port 3100",
    url: "http://localhost:3100",
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
  },
});
