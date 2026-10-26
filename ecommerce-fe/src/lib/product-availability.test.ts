import { describe, expect, it } from "vitest"
import { formatRating, resolveProductAvailability } from "./product-availability"
import type { ProductDetail, ProductSummary } from "./types"

const BASE_SUMMARY: ProductSummary = {
  id: "dt-1",
  name: "Điện thoại Nova X",
  brand: "Nova",
  categoryId: "dien-thoai",
  categoryName: "Điện thoại",
  price: 18990000,
  originalPrice: 21990000,
  rating: 4.900000095367432,
  reviewCount: 1284,
  soldCount: 2310,
}

const BASE_DETAIL: ProductDetail = {
  ...BASE_SUMMARY,
  description: "Mô tả đầy đủ.",
  details: ["5G"],
  variants: [{ label: "Đen", colorHex: "#1f2937" }],
  deleted: false,
}

describe("product-availability", () => {
  it("formats float32 rating precision artifacts to one decimal place", () => {
    expect(formatRating(4.900000095367432)).toBe("4.9")
    expect(formatRating(5)).toBe("5.0")
    expect(formatRating(Number.NaN)).toBe("0.0")
  })

  it("caps maxQuantity at 10 when AVAILABLE stock exceeds 10 and uses exact stock when between 6 and 10", () => {
    const highStock = resolveProductAvailability({
      ...BASE_DETAIL,
      status: "AVAILABLE",
      availableQuantity: 25,
    })
    expect(highStock).toMatchObject({
      status: "AVAILABLE",
      purchasable: true,
      maxQuantity: 10,
      availableQuantity: 25,
      badgeLabel: null,
      detailLabel: "Còn hàng (25 sản phẩm)",
    })

    const mediumStock = resolveProductAvailability({
      ...BASE_DETAIL,
      status: "AVAILABLE",
      availableQuantity: 7,
    })
    expect(mediumStock.maxQuantity).toBe(7)
    expect(mediumStock.detailLabel).toBe("Còn hàng (7 sản phẩm)")
  })

  it("resolves LIMITED stock with card badge and exact maxQuantity", () => {
    const resolved = resolveProductAvailability({
      ...BASE_DETAIL,
      status: "LIMITED",
      availableQuantity: 3,
    })
    expect(resolved).toMatchObject({
      status: "LIMITED",
      purchasable: true,
      maxQuantity: 3,
      availableQuantity: 3,
      badgeLabel: "Chỉ còn 3",
      detailLabel: "Sắp hết — Chỉ còn 3 sản phẩm",
    })
  })

  it("resolves OUT_OF_STOCK as non-purchasable with Tạm hết hàng labels", () => {
    const resolved = resolveProductAvailability({
      ...BASE_SUMMARY,
      status: "OUT_OF_STOCK",
      availableQuantity: 0,
    })
    expect(resolved).toMatchObject({
      status: "OUT_OF_STOCK",
      purchasable: false,
      maxQuantity: 0,
      availableQuantity: 0,
      badgeLabel: "Tạm hết hàng",
      detailLabel: "Tạm hết hàng",
      ctaLabel: "Tạm hết hàng",
    })
  })

  it("forces UNAVAILABLE when deleted is true or variants are empty even if status was AVAILABLE", () => {
    const softDeleted = resolveProductAvailability({
      ...BASE_DETAIL,
      deleted: true,
      status: "AVAILABLE",
      availableQuantity: 20,
    })
    expect(softDeleted).toMatchObject({
      status: "UNAVAILABLE",
      purchasable: false,
      maxQuantity: 0,
      availableQuantity: 0,
      badgeLabel: "Ngừng kinh doanh",
      detailLabel: "Ngừng kinh doanh",
      ctaLabel: "Ngừng kinh doanh",
    })

    const noVariants = resolveProductAvailability({
      ...BASE_DETAIL,
      variants: [],
      status: "AVAILABLE",
      availableQuantity: 20,
    })
    expect(noVariants.status).toBe("UNAVAILABLE")
    expect(noVariants.purchasable).toBe(false)
  })

  it("falls back to UNKNOWN purchasable state capped at 10 when status is omitted or inventory is degraded", () => {
    const omitted = resolveProductAvailability(BASE_SUMMARY)
    expect(omitted).toMatchObject({
      status: "UNKNOWN",
      purchasable: true,
      maxQuantity: 10,
      badgeLabel: null,
    })

    const degraded = resolveProductAvailability({
      ...BASE_DETAIL,
      status: "UNKNOWN",
      inventoryDegraded: true,
    })
    expect(degraded).toMatchObject({
      status: "UNKNOWN",
      purchasable: true,
      maxQuantity: 10,
      inventoryDegraded: true,
    })
  })
})
