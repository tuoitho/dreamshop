import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { AxiosHeaders, type InternalAxiosRequestConfig } from "axios"
import {
  ApiClientError,
  adjustSkuStock,
  fetchCurrentUser,
  fetchMyPermissions,
  getAdminOrder,
  http,
  listAdminInventory,
  listAdminOrders,
  listAdminProducts,
  refreshHttp,
  retryAdminOrderRefund,
  setSkuStock,
  updateAdminOrderStatus,
} from "./api"
import { clearAccessToken, setAccessToken } from "./auth-session"

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe("admin api client", () => {
  const originalHttpAdapter = http.defaults.adapter
  const originalRefreshAdapter = refreshHttp.defaults.adapter

  beforeEach(() => {
    clearAccessToken()
  })

  afterEach(() => {
    http.defaults.adapter = originalHttpAdapter
    refreshHttp.defaults.adapter = originalRefreshAdapter
    clearAccessToken()
  })

  it("reads unwrapped /users/me and enveloped /users/me/permissions", async () => {
    setAccessToken("valid-token")
    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      if (config.url === "/users/me") {
        return {
          data: {
            userId: "u-1",
            email: "admin@sshop.vn",
            fullName: "Quản trị viên",
            roles: ["ADMIN"],
          },
          status: 200,
          statusText: "OK",
          headers: {},
          config,
        }
      }
      if (config.url === "/users/me/permissions") {
        return {
          data: {
            data: ["product.view", "inventory.view"],
            message: "Effective permissions retrieved.",
          },
          status: 200,
          statusText: "OK",
          headers: {},
          config,
        }
      }
      throw new Error(`Unexpected URL: ${config.url}`)
    }

    const user = await fetchCurrentUser()
    const perms = await fetchMyPermissions()

    expect(user.userId).toBe("u-1")
    expect(user.email).toBe("admin@sshop.vn")
    expect(perms).toEqual(["product.view", "inventory.view"])
  })

  it("coalesces concurrent 401 responses into a single refresh request and retries once", async () => {
    setAccessToken("expired-token")
    let refreshCount = 0

    refreshHttp.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      refreshCount += 1
      await new Promise((r) => setTimeout(r, 15))
      return {
        data: {
          data: {
            accessToken: "fresh-token",
            tokenType: "Bearer",
            expiresIn: 900,
          },
          message: "Token refreshed.",
        },
        status: 200,
        statusText: "OK",
        headers: {},
        config,
      }
    }

    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      const auth = config.headers?.Authorization
      if (auth !== "Bearer fresh-token") {
        const err = new Error("Unauthorized") as Error & {
          isAxiosError: boolean
          response: unknown
          config: InternalAxiosRequestConfig
        }
        err.isAxiosError = true
        err.config = config
        err.response = {
          status: 401,
          statusText: "Unauthorized",
          headers: new AxiosHeaders(),
          config,
          data: { code: "UNAUTHORIZED", message: "Expired token" },
        }
        return Promise.reject(Object.assign(new (await import("axios")).AxiosError("Unauthorized", "ERR_BAD_REQUEST", config, null, err.response as never)))
      }

      if (config.url === "/admin/products") {
        return {
          data: {
            data: {
              items: [],
              page: 1,
              pageSize: 20,
              totalItems: 0,
              totalPages: 0,
              activeCount: 0,
              deletedCount: 0,
              categories: [],
              stockDegraded: false,
            },
            message: "Products retrieved.",
          },
          status: 200,
          statusText: "OK",
          headers: {},
          config,
        }
      }

      return {
        data: {
          data: {
            items: [],
            page: 1,
            pageSize: 20,
            totalItems: 0,
            totalPages: 0,
            summary: {
              totalTrackedProducts: 0,
              totalTrackedSkus: 0,
              inStockCount: 0,
              lowStockCount: 0,
              outOfStockCount: 0,
              totalQuantity: 0,
              totalReserved: 0,
              totalAvailable: 0,
            },
          },
          message: "Inventory retrieved.",
        },
        status: 200,
        statusText: "OK",
        headers: {},
        config,
      }
    }

    const [products, inventory] = await Promise.all([
      listAdminProducts({ page: 1, pageSize: 20 }),
      listAdminInventory({ page: 1, pageSize: 20 }),
    ])

    expect(refreshCount).toBe(1)
    expect(products.items).toEqual([])
    expect(inventory.items).toEqual([])
  })

  it("maps structured backend error envelopes into ApiClientError with code and traceId", async () => {
    setAccessToken("valid-token")
    const { AxiosError } = await import("axios")
    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      throw new AxiosError("Conflict", "ERR_BAD_REQUEST", config, null, {
        status: 409,
        statusText: "Conflict",
        headers: new AxiosHeaders(),
        config,
        data: {
          code: "STOCK_BELOW_RESERVED",
          message: "Stock quantity cannot be lower than reserved quantity.",
          traceId: "trace-abc-123",
        },
      })
    }

    await expect(setSkuStock("dt-1-v1", "dt-1", 2)).rejects.toMatchObject({
      name: "ApiClientError",
      status: 409,
      code: "STOCK_BELOW_RESERVED",
      traceId: "trace-abc-123",
    } satisfies Partial<ApiClientError>)
  })

  it("sends SKU-scoped PUT and POST /adjust requests with productId in the request body", async () => {
    setAccessToken("valid-token")
    const calls: Array<{ method?: string; url?: string; body: unknown }> = []

    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      calls.push({
        method: config.method?.toUpperCase(),
        url: config.url,
        body: typeof config.data === "string" ? JSON.parse(config.data) : config.data,
      })
      return {
        data: {
          data: {
            skuId: "dt-1-v1",
            productId: "dt-1",
            quantity: config.url?.endsWith("/adjust") ? 22 : 25,
            reserved: 2,
            availableQuantity: config.url?.endsWith("/adjust") ? 20 : 23,
          },
          message: "Stock updated.",
        },
        status: 200,
        statusText: "OK",
        headers: {},
        config,
      }
    }

    const setRes = await setSkuStock("dt-1-v1", "dt-1", 25)
    const adjustRes = await adjustSkuStock("dt-1-v1", "dt-1", -3)

    expect(calls).toEqual([
      {
        method: "PUT",
        url: "/admin/inventory/skus/dt-1-v1",
        body: { productId: "dt-1", quantity: 25 },
      },
      {
        method: "POST",
        url: "/admin/inventory/skus/dt-1-v1/adjust",
        body: { productId: "dt-1", delta: -3 },
      },
    ])
    expect(setRes).toMatchObject({ skuId: "dt-1-v1", productId: "dt-1", quantity: 25 })
    expect(adjustRes).toMatchObject({ skuId: "dt-1-v1", productId: "dt-1", quantity: 22 })
  })

  it("lists, reads and updates admin orders through /admin/orders", async () => {
    setAccessToken("valid-token")
    const calls: Array<{ method?: string; url?: string; params?: unknown; body: unknown }> = []

    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      calls.push({
        method: config.method?.toUpperCase(),
        url: config.url,
        params: config.params,
        body: typeof config.data === "string" ? JSON.parse(config.data) : config.data,
      })
      const data =
        config.url === "/admin/orders"
          ? { items: [], page: 1, pageSize: 20, totalItems: 0, totalPages: 0, statusCounts: { SHIPPING: 2 } }
          : { order: { orderId: "ord-1", status: "SHIPPING" }, allowedNextStatuses: ["DELIVERED"] }
      return { data: { data, message: "ok" }, status: 200, statusText: "OK", headers: {}, config }
    }

    const page = await listAdminOrders({ status: "SHIPPING", q: "ORD", page: 1 })
    const view = await getAdminOrder("ord-1")
    const updated = await updateAdminOrderStatus("ord-1", {
      expectedStatus: "PREPARING",
      targetStatus: "SHIPPING",
      shippingCarrier: "GHN",
      trackingNumber: "T1",
    })

    expect(calls).toEqual([
      { method: "GET", url: "/admin/orders", params: { status: "SHIPPING", q: "ORD", page: 1 }, body: undefined },
      { method: "GET", url: "/admin/orders/ord-1", params: undefined, body: undefined },
      {
        method: "POST",
        url: "/admin/orders/ord-1/status",
        params: undefined,
        body: {
          expectedStatus: "PREPARING",
          targetStatus: "SHIPPING",
          shippingCarrier: "GHN",
          trackingNumber: "T1",
        },
      },
    ])
    expect(page.statusCounts.SHIPPING).toBe(2)
    expect(view.allowedNextStatuses).toEqual(["DELIVERED"])
    expect(updated.order.status).toBe("SHIPPING")
  })

  it("posts manual refund retry to /admin/orders/{orderId}/refund", async () => {
    setAccessToken("valid-token")
    let captured: { method?: string; url?: string; body: unknown } | undefined

    http.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
      captured = {
        method: config.method?.toUpperCase(),
        url: config.url,
        body: typeof config.data === "string" ? JSON.parse(config.data) : config.data,
      }
      return {
        data: {
          data: {
            order: { orderId: "ord-1", status: "CANCELLED", paymentStatus: "REFUNDED", refundTransId: "momo-ref-1" },
            allowedNextStatuses: [],
          },
          message: "Hoàn tiền đơn hàng thành công",
        },
        status: 200,
        statusText: "OK",
        headers: {},
        config,
      }
    }

    const res = await retryAdminOrderRefund("ord-1", "Thử lại sau sự cố")

    expect(captured).toEqual({
      method: "POST",
      url: "/admin/orders/ord-1/refund",
      body: { reason: "Thử lại sau sự cố" },
    })
    expect(res.order.paymentStatus).toBe("REFUNDED")
    expect(res.order.refundTransId).toBe("momo-ref-1")
  })
})
