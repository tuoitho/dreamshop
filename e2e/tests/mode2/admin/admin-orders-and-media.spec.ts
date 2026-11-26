// Mode 2: Shape-Locked Mock E2E cho admin-fe (:5174)
// Ghép nhiều file domain golden snapshot: identity-auth.json + bff-admin.json
// Kiểm chứng: Login + RBAC gating + Stateful MoMo Manual Refund + Reload HttpOnly cookie + MinIO Presigned PUT (ADR-012).
import * as fs from "node:fs"
import * as path from "node:path"
import { fileURLToPath } from "node:url"
import { expect, test, type Page } from "@playwright/test"
import {
  loadContractSnapshots,
  mountContractRoutes,
  mountPresignedUploadTrap,
  type ContractRoute,
} from "../../../fixtures/contract-routes"
import { AdminLoginPage } from "../../../pages/AdminLoginPage"
import { AdminOrdersPage } from "../../../pages/AdminOrdersPage"
import { AdminProductsPage } from "../../../pages/AdminProductsPage"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const SNAPSHOTS_DIR = path.resolve(__dirname, "../../../../contracts/http-snapshots")
const IMGS_DIR = path.resolve(__dirname, "../../../reports/admin-orders-and-media/imgs")

const ADMIN_EMAIL = "admin@example.com"
const ADMIN_PASSWORD = "AdminPassword123!"

const OPERATOR_PERMISSIONS = [
  "product.view",
  "product.create",
  "product.update",
  "inventory.view",
  "order.admin.view",
  "order.admin.update",
]

const snapshots = loadContractSnapshots(
  [
    path.join(SNAPSHOTS_DIR, "identity-auth.json"),
    path.join(SNAPSHOTS_DIR, "bff-admin.json"),
  ],
  {
    nullable: ["phone", "dateOfBirth", "gender", "provinceCode", "wardCode", "addressLine"],
  }
)

function buildAdminRoutes(
  page: Page,
  state: { refunded: boolean; refundReason: string }
): ContractRoute[] {
  const adminProfile = structuredClone(
    snapshots.snapshots.users_me_200.responseBody as Record<string, unknown>
  )
  adminProfile.email = ADMIN_EMAIL
  adminProfile.fullName = "Quản trị viên Hệ thống"
  adminProfile.roles = ["ADMIN"]

  const adminPermsBody = structuredClone(
    snapshots.snapshots.users_me_permissions_200.responseBody as Record<string, unknown>
  )
  adminPermsBody.data = OPERATOR_PERMISSIONS

  const currentOrderSnapshotKey = () =>
    state.refunded ? "admin_order_retry_refund_200" : "admin_order_detail_200"

  const buildOrderDetailBody = () => {
    const base = structuredClone(
      snapshots.snapshots[currentOrderSnapshotKey()].responseBody as {
        data: { order: Record<string, unknown>; allowedNextStatuses: string[] }
        message: string
      }
    )
    if (state.refunded && state.refundReason) {
      base.data.order.refundReason = state.refundReason
    }
    return base
  }

  return [
    {
      method: "POST",
      path: "/api/v1/auth/admin/login",
      requestSnapshot: "auth_login_200",
      handle: async ({ body }) => {
        const creds = (body ?? {}) as { email?: string; password?: string }
        if (creds.email?.trim() !== ADMIN_EMAIL || creds.password !== ADMIN_PASSWORD) {
          return { snapshot: "auth_login_invalid_401" }
        }
        await page.context().addCookies([
          {
            name: "admin_refresh_token",
            value: "e2e-admin-refresh-cookie",
            domain: "localhost",
            path: "/api/v1/auth/admin",
            httpOnly: true,
            sameSite: "Lax",
          },
        ])
        return { snapshot: "auth_login_200" }
      },
    },
    {
      method: "POST",
      path: "/api/v1/auth/admin/refresh",
      handle: ({ cookies }) => {
        if (!cookies["admin_refresh_token"]) {
          return { snapshot: "auth_refresh_missing_401" }
        }
        return { snapshot: "auth_refresh_200" }
      },
    },
    {
      method: "GET",
      path: "/api/v1/users/me",
      auth: "bearer",
      handle: () => ({ snapshot: "users_me_200", body: adminProfile }),
    },
    {
      method: "GET",
      path: "/api/v1/users/me/permissions",
      auth: "bearer",
      handle: () => ({ snapshot: "users_me_permissions_200", body: adminPermsBody }),
    },
    {
      method: "GET",
      path: "/api/v1/admin/products",
      auth: "bearer",
      handle: () => ({ snapshot: "admin_products_list_200" }),
    },
    {
      method: "GET",
      path: "/api/v1/admin/inventory",
      auth: "bearer",
      handle: () => ({ snapshot: "admin_inventory_list_200" }),
    },
    {
      method: "GET",
      path: "/api/v1/admin/orders",
      auth: "bearer",
      handle: () => ({ snapshot: "admin_orders_list_200" }),
    },
    {
      method: "GET",
      path: /^\/api\/v1\/admin\/orders\/[^/]+$/,
      auth: "bearer",
      handle: () => ({
        snapshot: currentOrderSnapshotKey(),
        body: buildOrderDetailBody(),
      }),
    },
    {
      method: "POST",
      path: /^\/api\/v1\/admin\/orders\/[^/]+\/refund$/,
      auth: "bearer",
      requestSnapshot: "admin_order_retry_refund_200",
      handle: ({ body }) => {
        const payload = (body ?? {}) as { reason?: string }
        state.refunded = true
        state.refundReason = payload.reason?.trim() || "MoMo đã hoạt động trở lại"
        return {
          snapshot: "admin_order_retry_refund_200",
          body: buildOrderDetailBody(),
        }
      },
    },
    {
      method: "POST",
      path: "/api/v1/admin/products/media/presign",
      auth: "bearer",
      requestSnapshot: "admin_product_media_presign_200",
      handle: () => ({ snapshot: "admin_product_media_presign_200" }),
    },
  ]
}

test.describe("Mode 2 (Shape-Locked Mock): Admin Orders, RBAC & MinIO Media Upload", () => {
  test.beforeAll(() => {
    fs.mkdirSync(IMGS_DIR, { recursive: true })
  })

  test("đăng nhập admin, kiểm tra RBAC gate, hoàn tiền MoMo thủ công có stateful reload và chụp visual evidence desktop", async ({
    page,
  }) => {
    const state = { refunded: false, refundReason: "" }
    await mountContractRoutes(page, snapshots, buildAdminRoutes(page, state))

    const loginPage = new AdminLoginPage(page)
    const ordersPage = new AdminOrdersPage(page)

    await loginPage.goto()
    await loginPage.submit(ADMIN_EMAIL, ADMIN_PASSWORD)
    await expect(page).toHaveURL(/\/$/)

    // Xác minh quy tắc AGENTS.md: không lưu accessToken vào localStorage/sessionStorage
    const storedKeys = await page.evaluate(() => [
      ...Object.keys(window.localStorage),
      ...Object.keys(window.sessionStorage),
    ])
    expect(storedKeys.join(",")).not.toMatch(/token/i)

    // Kiểm tra RBAC PermissionGate: tài khoản không có quyền rbac.view bị chặn khi vào /rbac
    await page.goto("/rbac")
    await expect(page.getByText("Bạn không có quyền truy cập phân hệ này")).toBeVisible()
    await expect(page.getByText("rbac.view")).toBeVisible()

    // Điều hướng sang /orders và mở chi tiết đơn REFUND_FAILED
    await ordersPage.goto()
    await ordersPage.openOrderDetail("ORD-20261105-ABC123")
    await expect(page.getByText("Hoàn tiền tự động thất bại sau 3 lần thử.")).toBeVisible()
    await expect(page.getByText("Lỗi gần nhất: momo timeout 504")).toBeVisible()

    // Thực hiện hoàn tiền MoMo thủ công (request & response khóa theo admin_order_retry_refund_200)
    const refundResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/admin/orders/ord-123/refund") && resp.status() === 200
    )
    await ordersPage.executeManualRefund("MoMo đã hoạt động trở lại")
    await refundResp

    await expect(page.getByText("Đã hoàn tiền MoMo thành công")).toBeVisible()
    await expect(page.getByText("MOMO-REF-9002")).toBeVisible()
    await expect(page.getByText("Ghi chú hoàn tiền: MoMo đã hoạt động trở lại")).toBeVisible()
    await expect(ordersPage.manualRefundButton).toBeHidden()

    await page.screenshot({
      path: path.join(IMGS_DIR, "01-admin-order-refunded-desktop.png"),
      fullPage: false,
    })

    // Reload trang: HttpOnly cookie khôi phục phiên qua /auth/admin/refresh và state REFUNDED vẫn giữ nguyên
    const refreshAfterReload = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/auth/admin/refresh") && resp.status() === 200
    )
    await page.reload()
    await refreshAfterReload
    await ordersPage.openOrderDetail("ORD-20261105-ABC123")
    await expect(page.getByText("Đã hoàn tiền MoMo thành công")).toBeVisible()
    await expect(ordersPage.manualRefundButton).toBeHidden()
  })

  test("upload ảnh sản phẩm qua presign + MinIO PUT trực tiếp tuân thủ ADR-012 (không gắn Authorization header)", async ({
    page,
  }) => {
    const state = { refunded: false, refundReason: "" }
    await mountContractRoutes(page, snapshots, buildAdminRoutes(page, state))
    const uploads = await mountPresignedUploadTrap(page, {
      allowedContentTypes: ["image/png"],
    })

    // Bootstrap session bằng HttpOnly admin_refresh_token cookie (không dùng localStorage)
    await page.context().addCookies([
      {
        name: "admin_refresh_token",
        value: "e2e-admin-refresh-cookie",
        domain: "localhost",
        path: "/api/v1/auth/admin",
        httpOnly: true,
        sameSite: "Lax",
      },
    ])

    // Chặn GET ảnh publicUrl về SVG/PNG hợp lệ để ảnh preview render không broken
    const tinyPngBase64 =
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
    const tinyPng = Buffer.from(tinyPngBase64, "base64")
    await page.route("http://localhost:9000/product-media/**", async (route) => {
      if (route.request().method() === "GET") {
        await route.fulfill({ status: 200, contentType: "image/png", body: tinyPng })
        return
      }
      await route.fallback()
    })

    const productsPage = new AdminProductsPage(page)
    await productsPage.goto()
    await productsPage.openCreateDrawer()

    const presignResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/admin/products/media/presign") && resp.status() === 200
    )
    await productsPage.uploadMediaBuffer({
      name: "hero.png",
      mimeType: "image/png",
      buffer: tinyPng,
    })
    await presignResp

    await expect(page.getByRole("button", { name: "Xem chi tiết ảnh 1" })).toBeVisible()
    expect(uploads).toHaveLength(1)
    expect(uploads[0].contentType).toBe("image/png")
    expect(uploads[0].url).toContain("/product-media/products/2026/11/hero.png")

    await page.screenshot({
      path: path.join(IMGS_DIR, "02-admin-product-media-upload-desktop.png"),
      fullPage: false,
    })
  })
})
