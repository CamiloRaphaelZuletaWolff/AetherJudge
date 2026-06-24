import { defineConfig } from "@playwright/test";

// E2E drives the real stack. Prerequisites (not started here):
//   task infra:up && task db:seed     — PostgreSQL, Redis, demo contest
//   task executor:images              — sandbox images
// The three app processes below are started (or reused if already running).
export default defineConfig({
  testDir: "./e2e",
  timeout: 180_000,
  expect: { timeout: 15_000 },
  retries: process.env.CI ? 1 : 0,
  // One worker: journeys share the seeded demo contest and judge capacity.
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://localhost:3000",
    trace: "retain-on-failure",
  },
  webServer: [
    {
      command: "go run ./cmd/server",
      cwd: "../backend/services/executor",
      port: 9090,
      reuseExistingServer: true,
      timeout: 180_000,
    },
    {
      command: "go run ./cmd/server",
      cwd: "../backend/services/api-gateway",
      url: "http://localhost:8080/readyz",
      reuseExistingServer: true,
      timeout: 180_000,
    },
    {
      command: "pnpm dev",
      url: "http://localhost:3000",
      reuseExistingServer: true,
      timeout: 180_000,
    },
  ],
});
