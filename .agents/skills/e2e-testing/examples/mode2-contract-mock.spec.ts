// Reference Mode 2 spec: ghép nhiều file domain golden snapshot (identity-auth.json + bff-orders.json),
// test vòng đời reload bằng HttpOnly refresh_token cookie (zero localStorage), viewport desktop 1440x900.
import * as path from "node:path"
import { test, expect } from "@playwright/test"
import {
  loadContractSnapshots,
  mountContractRoutes,
  type ContractRoute,
} from "./contract-route-fixture"

test.use({ viewport: { width: 1440, height: 900 } })

const SNAPSHOTS_DIR = path.resolve(process.cwd(), "../contracts/http-snapshots")
const snapshots = loadContractSnapshots(
  [
    path.join(SNAPSHOTS_DIR, "identity-auth.json"),
    path.join(SNAPSHOTS_DIR, "bff-orders.json"),
  ],
  {
    nullable: ["phone", "refundTransId", "refundedAt"],
  }
)

test.describe("Customer Orders — Mode 2 Multi-Domain Contract Lock", () => {
  test("logs in via HttpOnly refresh cookie, reloads session, and views order detail", async ({ page }) => {
    let refreshCount = 0

    const routes: ContractRoute[] = [
      // Background layout calls ngoài phạm vi bài test được khai báo rõ là visible stubs.
      { method: "GET", path: "/api/v1/cart", stub: { status: 200, body: { data: { items: [], totalItems: 0 }, message: "OK" } } },
      { method: "GET", path: "/api/v1/favorites", stub: { status: 200, body: { data: { items: [] }, message: "OK" } } },
      {
        method: "POST",
        path: "/api/v1/auth/login",
        requestSnapshot: "auth_login_200",
        handle: async () => {
          await page.context().addCookies([
            {
              name: "refresh_token",
              value: "mock-refresh-token-1",
              url: "http://localhost:8000/api/v1/auth",
              httpOnly: true,
              sameSite: "Lax",
            },
          ])
          return {
            snapshot: "auth_login_200",
            headers: {
              "set-cookie": "refresh_token=mock-refresh-token-1; Path=/api/v1/auth; HttpOnly; SameSite=Lax",
            },
          }
        },
      },
      {
        method: "POST",
        path: "/api/v1/auth/refresh",
        auth: "refreshCookie",
        handle: () => {
          refreshCount += 1
          return { snapshot: "auth_refresh_200" }
        },
      },
      {
        method: "GET",
        path: "/api/v1/users/me",
        auth: "bearer",
        handle: () => ({ snapshot: "users_me_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/orders",
        auth: "bearer",
        handle: () => ({ snapshot: "orders_list_200" }),
      },
    ]

    await mountContractRoutes(page, snapshots, routes)

    await page.goto("/login")
    await page.getByLabel("Email").fill("customer1@gmail.com")
    await page.getByLabel("Mật khẩu").fill("123456123456")
    await page.getByRole("button", { name: /đăng nhập/i }).click()

    await page.goto("/don-hang")
    await expect(page.getByRole("heading", { name: /đơn hàng/i })).toBeVisible()

    // Kiểm tra reload giữ phiên bằng HttpOnly cookie (không dùng localStorage)
    const refreshResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/auth/refresh") && resp.request().method() === "POST"
    )
    await page.reload()
    expect((await refreshResp).status()).toBe(200)
    expect(refreshCount).toBeGreaterThanOrEqual(1)

    const tokenKeys = await page.evaluate(() =>
      Object.keys(window.localStorage).filter((k) => /token|jwt|auth/i.test(k))
    )
    expect(tokenKeys).toEqual([])
  })
})
