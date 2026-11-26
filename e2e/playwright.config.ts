import path from "node:path"
import { fileURLToPath } from "node:url"
import { defineConfig, devices } from "@playwright/test"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(__dirname, "..")

const e2eMode = process.env.E2E_MODE === "live" ? "live" : "mock"
const e2eApp = process.env.E2E_APP === "admin" ? "admin" : "storefront"

const appConfig =
  e2eApp === "admin"
    ? {
        baseURL: process.env.BASE_URL || "http://localhost:5174",
        command: "npm --prefix admin-fe run dev -- --port 5174 --strictPort",
      }
    : {
        baseURL: process.env.BASE_URL || "http://localhost:5173",
        command: "npm --prefix ecommerce-fe run dev -- --port 5173 --strictPort",
      }

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    baseURL: appConfig.baseURL,
    browserName: "chromium",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    actionTimeout: 10_000,
    navigationTimeout: 30_000,
  },
  webServer: {
    command: appConfig.command,
    cwd: repoRoot,
    url: appConfig.baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    {
      name: `chromium-${e2eApp}-${e2eMode}`,
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1440, height: 900 },
      },
    },
  ],
})
