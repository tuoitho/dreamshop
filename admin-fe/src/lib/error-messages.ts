import { AxiosError } from "axios"
import type { ApiError } from "./types"

export interface ResolveErrorOptions {
  fallback?: string
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

const ADMIN_ERROR_CODE_MESSAGES: Record<string, string> = {
  // Auth, Users & RBAC
  INVALID_CREDENTIALS: "Email hoặc mật khẩu không đúng.",
  EMAIL_ALREADY_REGISTERED: "Email này đã được đăng ký.",
  INVALID_REFRESH_TOKEN: "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
  AUTHENTICATION_UNAVAILABLE: "Dịch vụ đăng nhập tạm thời gián đoạn. Vui lòng thử lại sau.",
  USER_NOT_FOUND: "Không tìm thấy người dùng.",
  VALIDATION_FAILED: "Dữ liệu nhập vào chưa hợp lệ. Vui lòng kiểm tra lại.",
  UNAUTHORIZED: "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
  FORBIDDEN: "Bạn không có quyền thực hiện thao tác này.",
  NOT_FOUND: "Không tìm thấy dữ liệu yêu cầu.",
  INTERNAL_ERROR: "Hệ thống đang gặp sự cố. Vui lòng thử lại sau.",
  RBAC_ITEM_NOT_FOUND: "Không tìm thấy mục phân quyền.",
  RBAC_ITEM_EXISTS: "Mục phân quyền này đã tồn tại.",
  RBAC_HIERARCHY_CONFLICT: "Thay đổi phân cấp quyền không hợp lệ (có thể gây vòng lặp).",
  RBAC_SYSTEM_ITEM_PROTECTED: "Mục phân quyền hệ thống được bảo vệ, không thể thay đổi hoặc xóa.",
  SELF_DISABLE_NOT_ALLOWED: "Không thể tự vô hiệu hóa tài khoản quản trị của chính mình.",
  LAST_SUPER_ADMIN_REQUIRED: "Hệ thống phải có ít nhất một tài khoản Super Admin đang hoạt động.",

  // Products, Media & Inventory
  PRODUCT_NOT_FOUND: "Không tìm thấy sản phẩm.",
  PRODUCT_ID_EXISTS: "Mã sản phẩm này đã tồn tại.",
  SKU_ALREADY_EXISTS: "Mã SKU phân loại này đã tồn tại ở sản phẩm khác.",
  SKU_NOT_FOUND: "Không tìm thấy phân loại SKU trên sản phẩm này.",
  MEDIA_ALREADY_ATTACHED: "Một hoặc nhiều tệp ảnh đã được gắn cho sản phẩm khác.",
  MEDIA_STORAGE_UNAVAILABLE: "Dịch vụ lưu trữ hình ảnh tạm thời gián đoạn.",
  STOCK_BELOW_RESERVED: "Số lượng tồn kho không thể nhỏ hơn số lượng đang được giữ cho đơn hàng.",
  VARIANT_HAS_RESERVED_STOCK: "Không thể xóa phân loại vì đang có đơn hàng giữ tồn kho.",
  CATALOG_UNAVAILABLE: "Dịch vụ sản phẩm tạm thời gián đoạn. Vui lòng thử lại sau.",
  INVENTORY_UNAVAILABLE: "Dịch vụ tồn kho tạm thời gián đoạn. Vui lòng thử lại sau.",
  INVALID_REQUEST: "Dữ liệu yêu cầu không hợp lệ. Vui lòng kiểm tra lại.",
  INVALID_PARAMETER: "Tham số yêu cầu không hợp lệ. Vui lòng kiểm tra lại.",

  // Orders, Refunds & Payments
  ORDER_NOT_FOUND: "Không tìm thấy đơn hàng.",
  ORDER_STATUS_CONFLICT: "Trạng thái đơn hàng vừa thay đổi. Vui lòng tải lại trước khi thao tác.",
  ORDER_INVALID_TRANSITION: "Không thể chuyển sang trạng thái này từ trạng thái hiện tại của đơn.",
  ORDER_NOT_CANCELLABLE: "Đơn hàng hiện tại không thể hủy.",
  ORDER_REASON_REQUIRED: "Vui lòng nhập lý do trước khi xác nhận.",
  ORDER_REFUND_NOT_ELIGIBLE: "Đơn hàng không đủ điều kiện hoàn tiền.",
  REFUND_AUTO_RETRY_IN_PROGRESS: "Hệ thống đang tự động thử hoàn tiền lại. Vui lòng chờ.",
  REFUND_IN_PROGRESS: "Yêu cầu hoàn tiền đang được xử lý.",
  PAYMENT_GATEWAY_REJECTED: "Cổng thanh toán từ chối yêu cầu.",
  PAYMENT_GATEWAY_UNAVAILABLE: "Cổng thanh toán tạm thời gián đoạn. Vui lòng thử lại sau.",
  PAYMENT_REFUND_NOT_FOUND: "Không tìm thấy giao dịch hoàn tiền.",
  ORDER_SERVICE_UNAVAILABLE: "Dịch vụ đơn hàng tạm thời gián đoạn. Vui lòng thử lại sau.",
  ORDER_UNAVAILABLE: "Dịch vụ đơn hàng tạm thời gián đoạn. Vui lòng thử lại sau.",
  ORDER_CONFLICT: "Xung đột dữ liệu đơn hàng. Vui lòng tải lại trang.",
  AUTHORIZATION_SERVICE_UNAVAILABLE: "Dịch vụ phân quyền tạm thời gián đoạn. Vui lòng thử lại sau.",
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

  if (normalized.code === "VARIANT_HAS_RESERVED_STOCK") {
    const skuId = normalized.errors?.skuId?.trim() || normalized.errors?.sku_id?.trim()
    if (skuId) {
      return `Không thể xóa phân loại "${skuId}" vì đang có đơn hàng giữ tồn kho.`
    }
    if (trimmedMessage && VIETNAMESE_DIACRITIC_RE.test(trimmedMessage)) {
      return trimmedMessage
    }
    return ADMIN_ERROR_CODE_MESSAGES.VARIANT_HAS_RESERVED_STOCK
  }

  if (trimmedMessage && VIETNAMESE_DIACRITIC_RE.test(trimmedMessage)) {
    return trimmedMessage
  }

  if (normalized.code && normalized.code in ADMIN_ERROR_CODE_MESSAGES) {
    return ADMIN_ERROR_CODE_MESSAGES[normalized.code]
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
