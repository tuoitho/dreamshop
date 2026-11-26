import { describe, expect, it } from "vitest"
import { ApiClientError } from "./api"
import { formatInventoryInsufficientError, resolveErrorMessage } from "./error-messages"

describe("formatInventoryInsufficientError", () => {
  const findItem = (skuId: string) =>
    skuId === "dt-1-v2" ? { name: "Điện thoại Pro Max", variantLabel: "Đen 256GB" } : undefined

  it("formats out-of-stock message with product name and variant when available is 0", () => {
    const message = formatInventoryInsufficientError(
      { sku_id: "dt-1-v2", product_id: "dt-1", available: "0", requested: "2" },
      findItem
    )
    expect(message).toBe('Sản phẩm "Điện thoại Pro Max" (Đen 256GB) đã hết hàng.')
  })

  it("formats partial-stock message with available and requested counts", () => {
    const message = formatInventoryInsufficientError(
      { sku_id: "dt-1-v2", product_id: "dt-1", available: "3", requested: "24" },
      findItem
    )
    expect(message).toBe(
      'Sản phẩm "Điện thoại Pro Max" (Đen 256GB) chỉ còn 3 sản phẩm (bạn đang đặt 24).'
    )
  })

  it("omits variant parentheses when item has no variantLabel", () => {
    const message = formatInventoryInsufficientError(
      { sku_id: "plain-sku", available: "0", requested: "1" },
      () => ({ name: "Bàn phím cơ", variantLabel: "" })
    )
    expect(message).toBe('Sản phẩm "Bàn phím cơ" đã hết hàng.')
  })

  it("falls back cleanly when item lookup is unavailable", () => {
    expect(
      formatInventoryInsufficientError({ sku_id: "dt-1-v2", available: "3", requested: "24" })
    ).toBe("Một sản phẩm trong đơn chỉ còn 3 sản phẩm (bạn đang đặt 24).")
    expect(formatInventoryInsufficientError({ sku_id: "dt-1-v2", available: "0", requested: "1" })).toBe(
      "Một sản phẩm trong đơn đã hết hàng."
    )
    expect(formatInventoryInsufficientError(undefined)).toBe(
      "Một số sản phẩm trong đơn hàng không đủ tồn kho. Vui lòng kiểm tra lại giỏ hàng."
    )
  })
})

describe("resolveErrorMessage", () => {
  it("translates known backend error codes to Vietnamese", () => {
    expect(
      resolveErrorMessage(new ApiClientError(401, "Email or password is invalid.", "INVALID_CREDENTIALS"))
    ).toBe("Email hoặc mật khẩu không đúng.")
    expect(
      resolveErrorMessage(
        new ApiClientError(409, "Email is already registered", "EMAIL_ALREADY_REGISTERED")
      )
    ).toBe("Email này đã được đăng ký.")
    expect(
      resolveErrorMessage(new ApiClientError(404, "Cart item not found.", "CART_ITEM_NOT_FOUND"))
    ).toBe("Không tìm thấy sản phẩm trong giỏ hàng.")
    expect(
      resolveErrorMessage(
        new ApiClientError(409, "Product is no longer available.", "PRODUCT_UNAVAILABLE")
      )
    ).toBe("Sản phẩm hiện không còn kinh doanh.")
    expect(
      resolveErrorMessage(
        new ApiClientError(502, "Catalog service is unavailable.", "CATALOG_UNAVAILABLE")
      )
    ).toBe("Dịch vụ sản phẩm tạm thời gián đoạn. Vui lòng thử lại sau.")
  })

  it("formats INVENTORY_INSUFFICIENT with context item lookup", () => {
    const err = new ApiClientError(
      409,
      'insufficient stock for sku "dt-1-v2": available=3 requested=24',
      "INVENTORY_INSUFFICIENT",
      "trace-1",
      { sku_id: "dt-1-v2", product_id: "dt-1", available: "3", requested: "24" }
    )
    expect(
      resolveErrorMessage(err, {
        findItemBySku: (skuId) =>
          skuId === "dt-1-v2" ? { name: "Điện thoại Pro", variantLabel: "Xanh" } : undefined,
      })
    ).toBe('Sản phẩm "Điện thoại Pro" (Xanh) chỉ còn 3 sản phẩm (bạn đang đặt 24).')
  })

  it("returns network error message for network errors and uses fallback for non-error values", () => {
    expect(resolveErrorMessage(new ApiClientError(null, "Network Error"))).toBe(
      "Kết nối mạng bị gián đoạn. Vui lòng kiểm tra kết nối và thử lại."
    )
    expect(resolveErrorMessage(new TypeError("Failed to fetch"))).toBe(
      "Kết nối mạng bị gián đoạn. Vui lòng kiểm tra kết nối và thử lại."
    )
    expect(resolveErrorMessage(undefined, { fallback: "Không thể xử lý yêu cầu lúc này." })).toBe(
      "Không thể xử lý yêu cầu lúc này."
    )
  })

  it("uses caller fallback for unknown codes with non-Vietnamese messages", () => {
    expect(
      resolveErrorMessage(new ApiClientError(500, "Something went wrong", "UNKNOWN_CODE"), {
        fallback: "Không thể xử lý yêu cầu lúc này.",
      })
    ).toBe("Không thể xử lý yêu cầu lúc này.")
  })

  it("preserves already-Vietnamese messages for unknown codes", () => {
    expect(
      resolveErrorMessage(new ApiClientError(400, "Dữ liệu không hợp lệ từ máy chủ", "CUSTOM_VN"))
    ).toBe("Dữ liệu không hợp lệ từ máy chủ")
  })
})
