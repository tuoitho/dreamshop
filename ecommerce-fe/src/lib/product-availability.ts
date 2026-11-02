import type {
  ProductAvailabilityStatus,
  ProductDetail,
  ProductSummary,
  ProductVariant,
} from "./types"

const DEFAULT_MAX_ORDER_QUANTITY = 10

export interface ResolvedProductAvailability {
  status: ProductAvailabilityStatus
  purchasable: boolean
  maxQuantity: number
  availableQuantity: number | null
  inventoryDegraded: boolean
  badgeLabel: string | null
  detailLabel: string
  ctaLabel: string
}

export function formatRating(rating: number): string {
  const safe = Number.isFinite(rating) ? rating : 0
  return safe.toFixed(1)
}

function isProductDetail(
  product: ProductSummary | ProductDetail
): product is ProductDetail {
  return "variants" in product || "deleted" in product
}

export function resolveProductAvailability(
  product: ProductSummary | ProductDetail
): ResolvedProductAvailability {
  const detail = isProductDetail(product) ? product : null
  const isDiscontinued =
    product.status === "UNAVAILABLE" ||
    Boolean(detail?.deleted) ||
    (Array.isArray(detail?.variants) && detail.variants.length === 0)

  if (isDiscontinued) {
    return {
      status: "UNAVAILABLE",
      purchasable: false,
      maxQuantity: 0,
      availableQuantity: 0,
      inventoryDegraded: false,
      badgeLabel: "Ngừng kinh doanh",
      detailLabel: "Ngừng kinh doanh",
      ctaLabel: "Ngừng kinh doanh",
    }
  }

  if (
    product.status === "OUT_OF_STOCK" ||
    (typeof product.availableQuantity === "number" &&
      product.availableQuantity <= 0 &&
      product.status !== "UNKNOWN")
  ) {
    return {
      status: "OUT_OF_STOCK",
      purchasable: false,
      maxQuantity: 0,
      availableQuantity: 0,
      inventoryDegraded: false,
      badgeLabel: "Tạm hết hàng",
      detailLabel: "Tạm hết hàng",
      ctaLabel: "Tạm hết hàng",
    }
  }

  if (product.status === "LIMITED") {
    const qty = Math.max(1, product.availableQuantity ?? 1)
    return {
      status: "LIMITED",
      purchasable: true,
      maxQuantity: Math.min(DEFAULT_MAX_ORDER_QUANTITY, qty),
      availableQuantity: qty,
      inventoryDegraded: false,
      badgeLabel: `Chỉ còn ${qty}`,
      detailLabel: `Sắp hết — Chỉ còn ${qty} sản phẩm`,
      ctaLabel: "Thêm vào giỏ",
    }
  }

  if (product.status === "AVAILABLE") {
    const qty =
      typeof product.availableQuantity === "number" && product.availableQuantity > 0
        ? product.availableQuantity
        : null
    const maxQuantity =
      qty !== null
        ? Math.min(DEFAULT_MAX_ORDER_QUANTITY, qty)
        : DEFAULT_MAX_ORDER_QUANTITY
    return {
      status: "AVAILABLE",
      purchasable: true,
      maxQuantity,
      availableQuantity: qty,
      inventoryDegraded: false,
      badgeLabel: null,
      detailLabel: qty !== null ? `Còn hàng (${qty} sản phẩm)` : "Còn hàng",
      ctaLabel: "Thêm vào giỏ",
    }
  }

  return {
    status: "UNKNOWN",
    purchasable: true,
    maxQuantity: DEFAULT_MAX_ORDER_QUANTITY,
    availableQuantity: null,
    inventoryDegraded: Boolean(product.inventoryDegraded),
    badgeLabel: null,
    detailLabel: product.inventoryDegraded
      ? "Đang kiểm tra tồn kho"
      : "Còn hàng",
    ctaLabel: "Thêm vào giỏ",
  }
}

export function resolveVariantAvailability(
  product: {
    deleted?: boolean
    status?: ProductAvailabilityStatus
    inventoryDegraded?: boolean
  },
  variant?: ProductVariant
): ResolvedProductAvailability {
  if (
    Boolean(product.deleted) ||
    product.status === "UNAVAILABLE" ||
    !variant ||
    variant.status === "UNAVAILABLE"
  ) {
    return {
      status: "UNAVAILABLE",
      purchasable: false,
      maxQuantity: 0,
      availableQuantity: 0,
      inventoryDegraded: false,
      badgeLabel: "Ngừng kinh doanh",
      detailLabel: "Ngừng kinh doanh",
      ctaLabel: "Ngừng kinh doanh",
    }
  }

  if (
    variant.status === "OUT_OF_STOCK" ||
    (typeof variant.availableQuantity === "number" &&
      variant.availableQuantity <= 0 &&
      variant.status !== "UNKNOWN")
  ) {
    return {
      status: "OUT_OF_STOCK",
      purchasable: false,
      maxQuantity: 0,
      availableQuantity: 0,
      inventoryDegraded: false,
      badgeLabel: "Tạm hết hàng",
      detailLabel: "Tạm hết hàng",
      ctaLabel: "Tạm hết hàng",
    }
  }

  if (variant.status === "LIMITED") {
    const qty = Math.max(1, variant.availableQuantity ?? 1)
    return {
      status: "LIMITED",
      purchasable: true,
      maxQuantity: Math.min(DEFAULT_MAX_ORDER_QUANTITY, qty),
      availableQuantity: qty,
      inventoryDegraded: false,
      badgeLabel: `Chỉ còn ${qty}`,
      detailLabel: `Sắp hết — Chỉ còn ${qty} sản phẩm`,
      ctaLabel: "Thêm vào giỏ",
    }
  }

  if (variant.status === "AVAILABLE") {
    const qty =
      typeof variant.availableQuantity === "number" && variant.availableQuantity > 0
        ? variant.availableQuantity
        : null
    const maxQuantity =
      qty !== null
        ? Math.min(DEFAULT_MAX_ORDER_QUANTITY, qty)
        : DEFAULT_MAX_ORDER_QUANTITY
    return {
      status: "AVAILABLE",
      purchasable: true,
      maxQuantity,
      availableQuantity: qty,
      inventoryDegraded: false,
      badgeLabel: null,
      detailLabel: qty !== null ? `Còn hàng (${qty} sản phẩm)` : "Còn hàng",
      ctaLabel: "Thêm vào giỏ",
    }
  }

  return {
    status: "UNKNOWN",
    purchasable: true,
    maxQuantity: DEFAULT_MAX_ORDER_QUANTITY,
    availableQuantity: null,
    inventoryDegraded: Boolean(product.inventoryDegraded),
    badgeLabel: null,
    detailLabel: product.inventoryDegraded
      ? "Đang kiểm tra tồn kho"
      : "Còn hàng",
    ctaLabel: "Thêm vào giỏ",
  }
}
