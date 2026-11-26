import * as fs from "node:fs"
import * as path from "node:path"
import { fileURLToPath } from "node:url"
import { expect, test } from "@playwright/test"
import {
  assertNoTokenInWebStorage,
  assertOrSkipKongStackReachable,
  loginViaKongCookieJar,
} from "../../fixtures/kong-auth"
import { AdminOrdersPage } from "../../pages/AdminOrdersPage"
import { AdminProductsPage } from "../../pages/AdminProductsPage"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const IMGS_DIR = path.resolve(__dirname, "../../reports/live-kong-e2e/imgs")

test.describe("Mode 1 (Live via Kong): Admin Console Smoke", () => {
  test.beforeAll(() => {
    fs.mkdirSync(IMGS_DIR, { recursive: true })
  })

  test("restores admin session via HttpOnly admin_refresh_token cookie through Kong, loads live orders/products, and confirms generic variant drawer", async ({
    page,
  }) => {
    await assertOrSkipKongStackReachable(page)

    // Đăng nhập admin thật qua Kong (POST /api/v1/auth/admin/login -> HttpOnly admin_refresh_token)
    await loginViaKongCookieJar(page, { role: "admin" })

    const refreshResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/auth/admin/refresh") && resp.status() === 200
    )
    const permsResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/users/me/permissions") && resp.status() === 200
    )

    const ordersPage = new AdminOrdersPage(page)
    await ordersPage.goto()
    await Promise.all([refreshResp, permsResp])
    await assertNoTokenInWebStorage(page)

    await page.screenshot({
      path: path.join(IMGS_DIR, "01-live-admin-orders-desktop.png"),
      fullPage: false,
    })

    const productsPage = new AdminProductsPage(page)
    await productsPage.goto()
    await productsPage.openCreateDrawer()
    await expect(page.getByText("Màu sắc biến thể")).toHaveCount(0)

    await page.screenshot({
      path: path.join(IMGS_DIR, "02-live-admin-product-drawer-desktop.png"),
      fullPage: false,
    })
  })
})
