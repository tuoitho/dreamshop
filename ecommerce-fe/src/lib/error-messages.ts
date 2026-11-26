import { AxiosError } from "axios"
import type { ApiError } from "./types"

export interface InventoryItemLabel {
  name: string
  variantLabel?: string
}

export interface ResolveErrorOptions {
  fallback?: string
  findItemBySku?: (skuId: string) => InventoryItemLabel | undefined
}

interface NormalizedErrorShape {
  status?: number | null
  code?: string
  message: string
  errors?: Record<string, string>
}

const VIETNAMESE_DIACRITIC_RE =
  /[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]/i

const NETWORK_ERROR_RE = /network error|failed to fetch|timeout|err_network|econnaborted/i

const ERROR_CODE_MESSAGES: Record<string, string> = {
  // Identity & Auth
  INVALID_CREDENTIALS: "Email hoặc mật khẩu không đúng.",
  EMAIL_ALREADY_REGISTERED: "Email này đã được đăng ký.",
  INVALID_REFRESH_TOKEN: "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
  AUTHENTICATION_UNAVAILABLE: "Dịch vụ đăng nhập tạm thời gián đoạn. Vui lòng thử lại sau.",
  USER_NOT_FOUND: "Không tìm thấy tài khoản người dùng.",
  ADDRESS_NOT_FOUND: "Không tìm thấy địa chỉ giao hàng.",
  VALIDATION_FAILED: "Thông tin nhập vào chưa hợp lệ. Vui lòng kiểm tra lại.",
  UNAUTHORIZED: "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
  FORBIDDEN: "Bạn không có quyền thực hiện thao tác này.",
  CUSTOMER_ROLE_REQUIRED: "Trang mua sắm chỉ dành cho tài khoản khách hàng.",
  NOT_FOUND: "Không tìm thấy dữ liệu yêu cầu.",
  INTERNAL_ERROR: "Hệ thống đang gặp sự cố. Vui lòng thử lại sau.",

  // Cart, Favorites & Catalog
  CART_ITEM_NOT_FOUND: "Không tìm thấy sản phẩm trong giỏ hàng.",
  INVALID_QUANTITY: "Số lượng sản phẩm phải lớn hơn 0.",
  CART_UNAVAILABLE: "Dịch vụ giỏ hàng tạm thời gián đoạn. Vui lòng thử lại sau.",
  PRODUCT_NOT_FOUND: "Không tìm thấy sản phẩm.",
  PRODUCT_UNAVAILABLE: "Sản phẩm hiện không còn kinh doanh.",
  CATALOG_UNAVAILABLE: "Dịch vụ sản phẩm tạm thời gián đoạn. Vui lòng thử lại sau.",
  FAVORITES_UNAVAILABLE: "Dịch vụ yêu thích tạm thời gián đoạn. Vui lòng thử lại sau.",
  INVALID_PARAMETER: "Thông tin yêu cầu không hợp lệ. Vui lòng kiểm tra lại.",

  // Order, Payment & Inventory
  ORDER_NOT_FOUND: "Không tìm thấy đơn hàng.",
  ORDER_NOT_CANCELLABLE: "Đơn hàng hiện tại không thể hủy.",
  ORDER_STATUS_CONFLICT: "Trạng thái đơn hàng vừa thay đổi. Vui lòng tải lại trang.",
  ORDER_INVALID_TRANSITION: "Không thể cập nhật trạng thái đơn hàng ở bước này.",
  ORDER_REASON_REQUIRED: "Vui lòng nhập lý do trước khi thực hiện.",
  ORDER_CONFLICT: "Yêu cầu đơn hàng bị xung đột. Vui lòng tải lại và thử lại.",
  ORDER_SERVICE_UNAVAILABLE: "Dịch vụ đơn hàng tạm thời gián đoạn. Vui lòng thử lại sau.",
  PRODUCT_SNAPSHOT_MISMATCH: "Giá hoặc thông tin sản phẩm trong đơn vừa thay đổi.",
  PAYMENT_GATEWAY_REJECTED: "Cổng thanh toán từ chối giao dịch. Vui lòng thử lại hoặc chọn phương thức khác.",
  PAYMENT_GATEWAY_UNAVAILABLE: "Cổng thanh toán tạm thời gián đoạn. Vui lòng thử lại sau.",
  PAYMENT_NOT_FOUND: "Không tìm thấy giao dịch thanh toán.",
  PAYMENT_SERVICE_UNAVAILABLE: "Dịch vụ thanh toán tạm thời gián đoạn. Vui lòng thử lại sau.",
  INVALID_MOMO_SIGNATURE: "Xác thực chữ ký thanh toán MoMo không hợp lệ.",
  INVENTORY_UNAVAILABLE: "Dịch vụ tồn kho tạm thời gián đoạn. Vui lòng thử lại sau.",
  IDEMPOTENCY_KEY_REUSED: "Yêu cầu đặt hàng đã được sử dụng trước đó. Vui lòng bấm đặt hàng lại.",
  REQUEST_IN_PROGRESS: "Đơn hàng đang được xử lý. Vui lòng chờ trong giây lát.",
  AUTHORIZATION_SERVICE_UNAVAILABLE: "Dịch vụ xác thực tạm thời gián đoạn. Vui lòng thử lại sau.",
}

function parseNonNegativeInt(raw?: string): number | null {
  if (!raw) return null
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

function formatSubject(item?: InventoryItemLabel): string {
  const name = item?.name.trim()
  if (!name) return "Một sản phẩm trong đơn"
  const variant = item?.variantLabel?.trim()
  return variant ? `Sản phẩm "${name}" (${variant})` : `Sản phẩm "${name}"`
}

export function formatInventoryInsufficientError(
  errors?: Record<string, string>,
  findItemBySku?: (skuId: string) => InventoryItemLabel | undefined
): string {
  const skuId = errors?.sku_id?.trim() ?? ""
  const available = parseNonNegativeInt(errors?.available)
  const requested = parseNonNegativeInt(errors?.requested)
  const item = skuId && findItemBySku ? findItemBySku(skuId) : undefined
  const subject = formatSubject(item)

  if (available === 0) {
    return `${subject} đã hết hàng.`
  }
  if (available !== null) {
    if (requested !== null) {
      return `${subject} chỉ còn ${available} sản phẩm (bạn đang đặt ${requested}).`
    }
    return `${subject} chỉ còn ${available} sản phẩm.`
  }
  if (item?.name.trim()) {
    return `${subject} không đủ tồn kho.`
  }
  return "Một số sản phẩm trong đơn hàng không đủ tồn kho. Vui lòng kiểm tra lại giỏ hàng."
}

function normalizeError(error: unknown): NormalizedErrorShape {
  if (error instanceof AxiosError) {
    const body = error.response?.data as Partial<ApiError> | undefined
    return {
      status: error.response?.status ?? null,
      code: typeof body?.code === "string" ? body.code : undefined,
      message: typeof body?.message === "string" ? body.message : error.message,
      errors: body?.errors,
    }
  }
  if (error && typeof error === "object") {
    const candidate = error as Partial<NormalizedErrorShape>
    const status =
      typeof candidate.status === "number" || candidate.status === null ? candidate.status : undefined
    if (status !== undefined || typeof candidate.code === "string") {
      return {
        status: status ?? null,
        code: typeof candidate.code === "string" ? candidate.code : undefined,
        message: typeof candidate.message === "string" ? candidate.message : "",
        errors: candidate.errors,
      }
    }
  }
  if (error instanceof Error) {
    return { status: undefined, message: error.message }
  }
  return { status: undefined, message: "" }
}

export function resolveErrorMessage(error: unknown, options: ResolveErrorOptions = {}): string {
  const normalized = normalizeError(error)
  const trimmedMessage = normalized.message.trim()

  if (normalized.code === "INVENTORY_INSUFFICIENT") {
    return formatInventoryInsufficientError(normalized.errors, options.findItemBySku)
  }

  if (trimmedMessage && VIETNAMESE_DIACRITIC_RE.test(trimmedMessage)) {
    return trimmedMessage
  }

  if (normalized.code && normalized.code in ERROR_CODE_MESSAGES) {
    return ERROR_CODE_MESSAGES[normalized.code]
  }

  if (
    (normalized.status === null && !trimmedMessage) ||
    (normalized.status == null && NETWORK_ERROR_RE.test(trimmedMessage))
  ) {
    return "Kết nối mạng bị gián đoạn. Vui lòng kiểm tra kết nối và thử lại."
  }

  if (trimmedMessage && trimmedMessage.toLowerCase() !== "unexpected error") {
    if (options.fallback && normalized.code) {
      return options.fallback
    }
    return trimmedMessage
  }

  if (options.fallback) {
    return options.fallback
  }

  if (normalized.status === 401) {
    return "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại."
  }
  if (normalized.status === 403) {
    return "Bạn không có quyền thực hiện thao tác này."
  }
  if (normalized.status === 404) {
    return "Không tìm thấy dữ liệu yêu cầu."
  }
  if (typeof normalized.status === "number" && normalized.status >= 500) {
    return "Hệ thống đang bận. Vui lòng thử lại sau."
  }

  return "Đã xảy ra lỗi. Vui lòng thử lại."
}
