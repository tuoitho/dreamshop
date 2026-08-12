import { describe, it, expect, vi, beforeEach } from "vitest"
import { AxiosError, AxiosHeaders } from "axios"
import { toast } from "sonner"
import { ApiClientError, fetchHealth, getProduct, http, login, logoutRequest, searchProducts, updateCurrentUser } from "./api"
import { setAccessToken, clearAccessToken } from "./auth-session"

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
  })

  it("attaches the Bearer token when one is stored", async () => {
    setAccessToken("jwt-xyz")
    const adapter = vi.fn(async (config) => {
      throw new AxiosError("network error", "ERR_NETWORK", config)
    })
    http.defaults.adapter = adapter as never
    await fetchHealth().catch(() => undefined)
    const config = adapter.mock.calls[0][0] as { headers?: { Authorization?: string } }
    expect(config.headers?.Authorization).toBe("Bearer jwt-xyz")
    delete http.defaults.adapter
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
      data: { status: "UP", message: "Health loaded" },
      status: 200,
      statusText: "OK",
      headers: {},
      config,
    })) as never

    await fetchHealth()

    expect(toast.success).not.toHaveBeenCalled()
    expect(toast.error).not.toHaveBeenCalled()
  })
})

describe("storefront product adapters", () => {
  beforeEach(() => {
    clearAccessToken()
    vi.clearAllMocks()
    delete http.defaults.adapter
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
      variants: [{ color: "Đen", colorHex: "#1f2937" }],
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
