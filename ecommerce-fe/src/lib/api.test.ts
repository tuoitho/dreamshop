import { describe, it, expect, vi, beforeEach } from "vitest"
import { AxiosError, AxiosHeaders } from "axios"
import { toast } from "sonner"
import * as apiModule from "./api"
import {
  addCartItem,
  addFavoriteItem,
  ApiClientError,
  clearCart,
  clearFavorites,
  fetchCurrentUser,
  getCart,
  getFavorites,
  getProduct,
  http,
  login,
  logoutRequest,
  refreshHttp,
  removeCartItem,
  removeFavoriteItem,
  searchProducts,
  softDeleteProduct,
  updateCartItemQuantity,
  updateCurrentUser,
} from "./api"
import { advanceSessionEpoch, clearAccessToken, getAccessToken, setAccessToken } from "./auth-session"

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

function axiosError(status: number, data: unknown): AxiosError {
  return new AxiosError(
    "Request failed",
    "ERR_BAD_REQUEST",
    undefined,
    undefined,
    {
      status,
      data,
      statusText: "",
      headers: new AxiosHeaders(),
       config: { headers: new AxiosHeaders(), method: "post" } as never,
    }
  )
}

describe("api client", () => {
  beforeEach(() => {
    clearAccessToken()
    vi.clearAllMocks()
    delete http.defaults.adapter
    delete refreshHttp.defaults.adapter
  })

  it("attaches the Bearer token when one is stored", async () => {
    setAccessToken("jwt-xyz")
    const adapter = vi.fn(async (config) => {
      throw new AxiosError("network error", "ERR_NETWORK", config)
    })
    http.defaults.adapter = adapter as never
    await fetchCurrentUser().catch(() => undefined)
    const config = adapter.mock.calls[0][0] as { headers?: { Authorization?: string } }
    expect(config.headers?.Authorization).toBe("Bearer jwt-xyz")
    delete http.defaults.adapter
  })

  it("does not expose internal actuator health probes on the public client", () => {
    expect("fetchHealth" in apiModule).toBe(false)
  })

  it("uses credentials for cookie-backed refresh", () => {
    expect(http.defaults.withCredentials).toBe(true)
  })

  it("translates an ApiError envelope into ApiClientError", async () => {
    vi.spyOn(http, "post").mockRejectedValueOnce(
      axiosError(409, { code: "EMAIL_ALREADY_REGISTERED", message: "Email is already registered", traceId: "t-1" })
    )
    const err = await login({ email: "a@b.com", password: "password-123" }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.status).toBe(409)
    expect(err.code).toBe("EMAIL_ALREADY_REGISTERED")
    expect(err.message).toBe("Email is already registered")
    expect(err.traceId).toBe("t-1")
  })

  it("falls back to the transport message when the body is not an ApiError", async () => {
    vi.spyOn(http, "post").mockRejectedValueOnce(axiosError(502, "<html>bad gateway</html>"))
    const err = await login({ email: "a@b.com", password: "password-123" }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.status).toBe(502)
  })

  it("carries field-level validation errors from the ApiError envelope", async () => {
    vi.spyOn(http, "patch").mockRejectedValueOnce(
      axiosError(400, {
        code: "VALIDATION_FAILED",
        message: "Request validation failed",
        errors: {
          phone: "must be a valid Vietnamese mobile number",
          dateOfBirth: "must be a past date",
        },
        traceId: "t-1",
      })
    )
    const err = await updateCurrentUser({ phone: "123" }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.code).toBe("VALIDATION_FAILED")
    expect(err.errors).toEqual({
      phone: "must be a valid Vietnamese mobile number",
      dateOfBirth: "must be a past date",
    })
  })

  it("shows the API success message for a mutation", async () => {
    http.defaults.adapter = vi.fn(async (config) => ({
      data: { message: "Logged out successfully" },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    })) as never

    await logoutRequest()

    expect(toast.success).toHaveBeenCalledWith("Logged out successfully")
    expect(toast.error).not.toHaveBeenCalled()
  })

  it("shows the API error message for a failed mutation", async () => {
    http.defaults.adapter = vi.fn(async (config) => {
      throw new AxiosError(
        "Request failed",
        "ERR_BAD_REQUEST",
        config,
        undefined,
        {
          status: 400,
          data: { message: "Request validation failed" },
          statusText: "Bad Request",
          headers: new AxiosHeaders(),
          config,
        }
      )
    }) as never

    await logoutRequest().catch(() => undefined)

    expect(toast.error).toHaveBeenCalledWith("Request validation failed")
    expect(toast.success).not.toHaveBeenCalled()
  })

  it("does not show a toast for GET requests", async () => {
    http.defaults.adapter = vi.fn(async (config) => ({
      data: { id: "u-1", email: "user@example.com", fullName: "Test User" },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    })) as never

    await fetchCurrentUser()

    expect(toast.success).not.toHaveBeenCalled()
    expect(toast.error).not.toHaveBeenCalled()
  })

  it("clears the stored token when a 401 refresh attempt fails", async () => {
    setAccessToken("jwt-expired")
    http.defaults.adapter = vi.fn(async (config) => {
      throw new AxiosError("Unauthorized", "ERR_BAD_REQUEST", config, undefined, {
        status: 401,
        data: { code: "TOKEN_EXPIRED", message: "Access token expired" },
        statusText: "Unauthorized",
        headers: new AxiosHeaders(),
        config,
      })
    }) as never
    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      throw new AxiosError("Unauthorized", "ERR_BAD_REQUEST", config, undefined, {
        status: 401,
        data: { code: "REFRESH_TOKEN_EXPIRED", message: "Refresh token expired" },
        statusText: "Unauthorized",
        headers: new AxiosHeaders(),
        config,
      })
    }) as never

    await expect(fetchCurrentUser()).rejects.toBeInstanceOf(ApiClientError)
    expect(getAccessToken()).toBeNull()
  })

  it("does not refresh or clear a newer session when a pre-logout or pre-login request returns 401", async () => {
    setAccessToken("jwt-old")
    let sentConfig!: never
    let rejectOldRequest!: (err: AxiosError) => void
    let notifyAdapterCalled!: () => void
    const adapterCalled = new Promise<void>((resolve) => {
      notifyAdapterCalled = resolve
    })
    http.defaults.adapter = vi.fn(
      (config) =>
        new Promise((_resolve, reject) => {
          sentConfig = config as never
          rejectOldRequest = reject
          notifyAdapterCalled()
        })
    ) as never
    const refreshAdapter = vi.fn(async (config) => {
      throw new AxiosError("Unauthorized", "ERR_BAD_REQUEST", config, undefined, {
        status: 401,
        data: { code: "REFRESH_TOKEN_EXPIRED", message: "Refresh token expired" },
        statusText: "Unauthorized",
        headers: new AxiosHeaders(),
        config,
      })
    })
    refreshHttp.defaults.adapter = refreshAdapter as never

    const pendingRequest = fetchCurrentUser()
    await adapterCalled

    // Session changes (e.g. login or logout) while the old request is still in flight
    advanceSessionEpoch()
    setAccessToken("jwt-new")

    rejectOldRequest(
      new AxiosError("Unauthorized", "ERR_BAD_REQUEST", sentConfig, undefined, {
        status: 401,
        data: { code: "TOKEN_EXPIRED", message: "Old token expired" },
        statusText: "Unauthorized",
        headers: new AxiosHeaders(),
        config: sentConfig,
      })
    )

    await expect(pendingRequest).rejects.toBeInstanceOf(ApiClientError)
    expect(refreshAdapter).not.toHaveBeenCalled()
    expect(getAccessToken()).toBe("jwt-new")
  })
})

describe("storefront product adapters", () => {
  beforeEach(() => {
    clearAccessToken()
    vi.clearAllMocks()
    delete http.defaults.adapter
    delete refreshHttp.defaults.adapter
  })

  it("builds storefront query params and parses the paginated envelope", async () => {
    const adapter = vi.fn(async (config) => ({
      data: {
        data: {
          items: [
            {
              id: "dt-1",
              name: "Nova X15 Pro",
              brand: "Nova",
              categoryId: "dien-thoai",
              categoryName: "Điện thoại",
              price: 18990000,
              originalPrice: 21990000,
              rating: 4.9,
              reviewCount: 1284,
              soldCount: 2310,
            },
          ],
          page: 1,
          pageSize: 20,
          totalItems: 1,
          totalPages: 1,
        },
        message: "Items retrieved.",
      },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    const result = await searchProducts({
      q: "nova",
      categoryId: "dien-thoai",
      priceMin: 500000,
      priceMax: 2000000,
      onSale: true,
      ratingMin: 4,
      sort: "gia-thap",
      page: 2,
      pageSize: 12,
    })

    const config = adapter.mock.calls[0][0] as { url: string; params: Record<string, unknown> }
    expect(config.url).toBe("/storefront/products")
    expect(config.params).toEqual({
      q: "nova",
      "danh-muc": "dien-thoai",
      gia: "500000-2000000",
      "giam-gia": 1,
      "danh-gia": 4,
      "sap-xep": "gia-thap",
      page: 2,
      pageSize: 12,
    })
    expect(result).toEqual({
      items: [expect.objectContaining({ id: "dt-1", name: "Nova X15 Pro" })],
      page: 1,
      pageSize: 20,
      totalItems: 1,
      totalPages: 1,
    })
  })

  it("omits empty and unbounded price values from the storefront query", async () => {
    const adapter = vi.fn(async (config) => ({
      data: { data: { items: [], page: 1, pageSize: 20, totalItems: 0, totalPages: 0 }, message: "Items retrieved." },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    await searchProducts({ priceMin: 10000000 })
    await searchProducts({})

    const configs = adapter.mock.calls.map((call) => call[0] as { params: Record<string, unknown> })
    expect(configs[0].params).toEqual({ gia: "10000000-0" })
    expect(configs[1].params).toEqual({})
  })

  it("parses a product detail envelope", async () => {
    const detail = {
      id: "dt-1",
      name: "Nova X15 Pro",
      brand: "Nova",
      categoryId: "dien-thoai",
      categoryName: "Điện thoại",
      price: 18990000,
      originalPrice: 21990000,
      rating: 4.9,
      reviewCount: 1284,
      soldCount: 2310,
      shortDescription: "Chip mạnh, pin trâu.",
      description: "Mô tả đầy đủ sản phẩm.",
      details: ["Màn hình 6.8 inch"],
      variants: [{ label: "Đen", colorHex: "#1f2937" }],
    }
    const adapter = vi.fn(async (config) => ({
      data: { data: detail, message: "Product loaded." },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    const result = await getProduct("dt-1")

    const config = adapter.mock.calls[0][0] as { url: string }
    expect(config.url).toBe("/storefront/products/dt-1")
    expect(result).toEqual(detail)
  })

  it("surfaces PRODUCT_NOT_FOUND as an ApiClientError", async () => {
    vi.spyOn(http, "get").mockRejectedValueOnce(
      axiosError(404, { code: "PRODUCT_NOT_FOUND", message: "Product not found", traceId: "t-1" })
    )

    const err = await getProduct("missing").catch((e) => e)

    expect(err).toBeInstanceOf(ApiClientError)
    expect(err.status).toBe(404)
    expect(err.code).toBe("PRODUCT_NOT_FOUND")
    expect(err.message).toBe("Product not found")
    expect(err.traceId).toBe("t-1")
  })
})

describe("cart adapters", () => {
  const cart = { items: [], degraded: false }

  beforeEach(() => {
    clearAccessToken()
    vi.clearAllMocks()
    delete http.defaults.adapter
    delete refreshHttp.defaults.adapter
  })

  it("extracts the cart envelope for GET", async () => {
    const adapter = vi.fn(async (config) => ({
      data: { data: cart, message: "Cart retrieved." },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    await expect(getCart()).resolves.toEqual(cart)
    expect((adapter.mock.calls[0][0] as { url: string; method: string }).url).toBe("/cart")
    expect((adapter.mock.calls[0][0] as { method: string }).method).toBe("get")
  })

  it("sends camelCase bodies for cart mutations and extracts each envelope", async () => {
    const adapter = vi.fn(async (config) => ({
      data: { data: cart, message: "Cart updated." },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    await addCartItem({ productId: "p-1", variantLabel: "16GB/512GB", quantity: 2 })
    await updateCartItemQuantity("item-1", 3)
    await removeCartItem("item-1")
    await clearCart()

    expect(adapter.mock.calls.map(([config]) => {
      const request = config as { url: string; method: string; data?: unknown }
      return {
        url: request.url,
        method: request.method,
        data: typeof request.data === "string" ? JSON.parse(request.data) : request.data,
      }
    })).toEqual([
      { url: "/cart/items", method: "post", data: { productId: "p-1", variantLabel: "16GB/512GB", quantity: 2 } },
      { url: "/cart/items/item-1", method: "patch", data: { quantity: 3 } },
      { url: "/cart/items/item-1", method: "delete", data: undefined },
      { url: "/cart", method: "delete", data: undefined },
    ])
  })

  it("posts the requested soft-delete state and extracts the response", async () => {
    const result = { id: "p-1", deleted: true }
    const adapter = vi.fn(async (config) => ({
      data: { data: result, message: "Product availability updated." },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    await expect(softDeleteProduct("p-1", true)).resolves.toEqual(result)
    const request = adapter.mock.calls[0][0] as { url: string; method: string; data?: unknown }
    expect({ ...request, data: typeof request.data === "string" ? JSON.parse(request.data) : request.data }).toMatchObject({
      url: "/admin/products/p-1/delete",
      method: "post",
      data: { deleted: true },
    })
  })

  it("maps a cart error envelope into ApiClientError", async () => {
    vi.spyOn(http, "post").mockRejectedValueOnce(
      axiosError(409, { code: "CART_ITEM_LIMITED", message: "Only 1 item remains", traceId: "cart-t-1" })
    )

    const error = await addCartItem({ productId: "p-1", variantLabel: "Red", quantity: 2 }).catch((value) => value)

    expect(error).toBeInstanceOf(ApiClientError)
    expect(error.status).toBe(409)
    expect(error.code).toBe("CART_ITEM_LIMITED")
    expect(error.message).toBe("Only 1 item remains")
    expect(error.traceId).toBe("cart-t-1")
  })

  it("suppresses toast.success on POST /cart/items when non-degraded response marks the added item OUT_OF_STOCK or UNAVAILABLE", async () => {
    http.defaults.adapter = vi.fn(async (config) => ({
      data: {
        data: {
          degraded: false,
          items: [
            {
              itemId: "item-1",
              productId: "p-1",
              variantLabel: "Đen",
              quantity: 1,
              name: "Nova X",
              brand: "Nova",
              categoryName: "Điện thoại",
              unitPrice: 1000000,
              imageEmoji: "📱",
              imageHue: [200, 240],
              imageUrl: "",
              status: "OUT_OF_STOCK",
              availableQuantity: 0,
            },
          ],
        },
        message: "Item added to cart.",
      },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    })) as never

    await addCartItem({ productId: "p-1", variantLabel: "Đen", quantity: 1 })
    expect(toast.success).not.toHaveBeenCalled()
  })

  it("keeps toast.success on POST /cart/items when cart.degraded is true even if status is UNAVAILABLE", async () => {
    http.defaults.adapter = vi.fn(async (config) => ({
      data: {
        data: {
          degraded: true,
          items: [
            {
              itemId: "item-1",
              productId: "p-1",
              variantLabel: "Đen",
              quantity: 1,
              name: "Nova X",
              brand: "Nova",
              categoryName: "Điện thoại",
              unitPrice: 1000000,
              imageEmoji: "📱",
              imageHue: [200, 240],
              imageUrl: "",
              status: "UNAVAILABLE",
              availableQuantity: 0,
            },
          ],
        },
        message: "Item added to cart.",
      },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    })) as never

    await addCartItem({ productId: "p-1", variantLabel: "Đen", quantity: 1 })
    expect(toast.success).toHaveBeenCalledWith("Item added to cart.")
  })
})

describe("favorites adapters", () => {
  const favorites = { items: [], degraded: false }

  beforeEach(() => {
    clearAccessToken()
    vi.clearAllMocks()
    delete http.defaults.adapter
    delete refreshHttp.defaults.adapter
  })

  it("fetches GET /favorites without triggering mutation toasts", async () => {
    const adapter = vi.fn(async (config) => ({
      data: { data: favorites, message: "Favorites retrieved." },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    await expect(getFavorites()).resolves.toEqual(favorites)
    expect((adapter.mock.calls[0][0] as { url: string; method: string }).url).toBe("/favorites")
    expect((adapter.mock.calls[0][0] as { method: string }).method).toBe("get")
    expect(toast.success).not.toHaveBeenCalled()
  })

  it("sends favorite mutations with URL-encoded productId and triggers single interceptor toast per mutation", async () => {
    const adapter = vi.fn(async (config) => ({
      data: { data: favorites, message: "Product added to favorites." },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    }))
    http.defaults.adapter = adapter as never

    await addFavoriteItem("dt-1")
    await removeFavoriteItem("dt/1 special")
    await clearFavorites()

    expect(adapter.mock.calls.map(([config]) => {
      const request = config as { url: string; method: string; data?: unknown }
      return {
        url: request.url,
        method: request.method,
        data: typeof request.data === "string" ? JSON.parse(request.data) : request.data,
      }
    })).toEqual([
      { url: "/favorites/items", method: "post", data: { productId: "dt-1" } },
      { url: "/favorites/items/dt%2F1%20special", method: "delete", data: undefined },
      { url: "/favorites", method: "delete", data: undefined },
    ])
    expect(toast.success).toHaveBeenCalledTimes(3)
  })
})
