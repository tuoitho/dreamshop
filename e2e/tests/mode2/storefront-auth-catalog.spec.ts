import * as fs from "node:fs"
import * as path from "node:path"
import { fileURLToPath } from "node:url"
import { expect, test, type Page } from "@playwright/test"
import {
  loadContractSnapshots,
  mountContractRoutes,
  type ContractRoute,
} from "../../fixtures/contract-routes"
import { assertNoTokenInWebStorage } from "../../fixtures/kong-auth"
import { HomePage } from "../../pages/HomePage"
import { LoginPage } from "../../pages/LoginPage"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const SNAPSHOTS_DIR = path.resolve(__dirname, "../../../contracts/http-snapshots")
const IMGS_DIR = path.resolve(__dirname, "../../reports/storefront-auth-catalog/imgs")

const VALID_EMAIL = "customer1@gmail.com"
const VALID_PASSWORD = "123456123456"

const snapshots = loadContractSnapshots(
  [
    path.join(SNAPSHOTS_DIR, "identity-auth.json"),
    path.join(SNAPSHOTS_DIR, "bff-storefront.json"),
  ],
  {
    nullable: ["phone", "dateOfBirth", "gender", "provinceCode", "wardCode", "addressLine"],
  }
)

function buildStorefrontRoutes(page: Page): ContractRoute[] {
  return [
    {
      method: "POST",
      path: "/api/v1/auth/login",
      requestSnapshot: "auth_login_200",
      handle: async ({ body }) => {
        const creds = (body ?? {}) as { email?: string; password?: string }
        if (creds.email?.trim() !== VALID_EMAIL || creds.password !== VALID_PASSWORD) {
          return { snapshot: "auth_login_invalid_401" }
        }
        await page.context().addCookies([
          {
            name: "refresh_token",
            value: "e2e-mock-refresh-session",
            domain: "localhost",
            path: "/api/v1/auth",
            httpOnly: true,
            sameSite: "Lax",
          },
        ])
        return { snapshot: "auth_login_200" }
      },
    },
    {
      method: "POST",
      path: "/api/v1/auth/refresh",
      auth: "refreshCookie",
      handle: () => ({ snapshot: "auth_refresh_200" }),
    },
    {
      method: "GET",
      path: "/api/v1/users/me",
      auth: "bearer",
      handle: () => ({ snapshot: "users_me_200" }),
    },
    {
      method: "GET",
      path: "/api/v1/storefront/products",
      handle: () => ({ snapshot: "storefront_products_200" }),
    },
    {
      method: "GET",
      path: "/api/v1/cart",
      auth: "bearer",
      unauthenticatedSnapshot: "bff_unauthenticated_401",
      handle: () => ({ snapshot: "cart_get_200" }),
    },
    {
      method: "GET",
      path: "/api/v1/favorites",
      auth: "bearer",
      unauthenticatedSnapshot: "bff_unauthenticated_401",
      handle: () => ({ snapshot: "favorites_get_200" }),
    },
  ]
}

test.describe("Mode 2 (Shape-Locked Mock): Storefront Auth & Catalog", () => {
  test.beforeAll(() => {
    fs.mkdirSync(IMGS_DIR, { recursive: true })
  })

  test("shows 401 INVALID_CREDENTIALS alert on wrong password", async ({ page }) => {
    await mountContractRoutes(page, snapshots, buildStorefrontRoutes(page))

    const loginPage = new LoginPage(page)
    await loginPage.goto()
    await loginPage.submit(VALID_EMAIL, "wrong-password")

    await loginPage.expectError("Email or password is invalid.")
    await expect(page).toHaveURL(/\/login$/)
  })

  test("logs in, renders BFF catalog, restores session via HttpOnly cookie on reload, and keeps Web Storage token-free", async ({
    page,
  }) => {
    await mountContractRoutes(page, snapshots, buildStorefrontRoutes(page))

    const loginPage = new LoginPage(page)
    const homePage = new HomePage(page)

    await loginPage.goto()

    const productsLoaded = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/storefront/products") && resp.status() === 200
    )
    await loginPage.submit(VALID_EMAIL, VALID_PASSWORD)

    await expect(page).toHaveURL(/\/$/)
    await productsLoaded
    await homePage.expectLoaded()
    await homePage.expectProductVisible("Điện thoại Nova X")
    await homePage.expectProductVisible("Laptop AeroBook Pro 14")

    await assertNoTokenInWebStorage(page)

    await page.screenshot({
      path: path.join(IMGS_DIR, "01-storefront-home-desktop.png"),
      fullPage: false,
    })

    const refreshAfterReload = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/auth/refresh") && resp.status() === 200
    )
    const meAfterReload = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/users/me") && resp.status() === 200
    )
    await page.reload()
    await Promise.all([refreshAfterReload, meAfterReload])
    await assertNoTokenInWebStorage(page)
    await homePage.expectProductVisible("Điện thoại Nova X")
  })
})
