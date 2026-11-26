import type { BadgeProps } from "@/components/ui/badge"
import type { OrderActorType, OrderPaymentStatus, OrderStatus } from "./types"

export const ORDER_STATUS_LABELS: Record<OrderStatus, string> = {
  PENDING_STOCK: "Đang giữ hàng",
  PENDING_PAYMENT: "Chờ thanh toán",
  PENDING_CONFIRMATION: "Chờ xác nhận",
  CONFIRMED: "Đã xác nhận",
  PREPARING: "Đang chuẩn bị",
  SHIPPING: "Đang giao",
  DELIVERED: "Đã giao",
  DELIVERY_FAILED: "Giao thất bại",
  CANCELLED: "Đã hủy",
}

export const PAYMENT_STATUS_LABELS: Record<OrderPaymentStatus, string> = {
  UNPAID: "Chưa thanh toán",
  PENDING: "Đang chờ thanh toán",
  PAID: "Đã thanh toán",
  FAILED: "Thanh toán thất bại",
  REFUND_REQUIRED: "Cần hoàn tiền",
  REFUNDED: "Đã hoàn tiền",
  REFUND_FAILED: "Hoàn tiền thất bại",
}

export const ACTOR_LABELS: Record<OrderActorType, string> = {
  SYSTEM: "Hệ thống",
  CUSTOMER: "Khách hàng",
  ADMIN: "Quản trị viên",
}

/** Status tabs in fulfillment order; `undefined` lists every order. */
export const ORDER_STATUS_TABS: Array<{ status?: OrderStatus; label: string }> = [
  { label: "Tất cả" },
  { status: "PENDING_PAYMENT", label: ORDER_STATUS_LABELS.PENDING_PAYMENT },
  { status: "PENDING_CONFIRMATION", label: ORDER_STATUS_LABELS.PENDING_CONFIRMATION },
  { status: "CONFIRMED", label: ORDER_STATUS_LABELS.CONFIRMED },
  { status: "PREPARING", label: ORDER_STATUS_LABELS.PREPARING },
  { status: "SHIPPING", label: ORDER_STATUS_LABELS.SHIPPING },
  { status: "DELIVERED", label: ORDER_STATUS_LABELS.DELIVERED },
  { status: "DELIVERY_FAILED", label: ORDER_STATUS_LABELS.DELIVERY_FAILED },
  { status: "CANCELLED", label: ORDER_STATUS_LABELS.CANCELLED },
]

export function orderStatusBadgeVariant(status: OrderStatus): BadgeProps["variant"] {
  switch (status) {
    case "DELIVERED":
      return "success"
    case "CANCELLED":
    case "DELIVERY_FAILED":
      return "destructive"
    case "PENDING_PAYMENT":
    case "PENDING_CONFIRMATION":
    case "PENDING_STOCK":
      return "warning"
    case "CONFIRMED":
    case "PREPARING":
    case "SHIPPING":
      return "default"
  }
}

export function paymentStatusBadgeVariant(status: OrderPaymentStatus): BadgeProps["variant"] {
  switch (status) {
    case "PAID":
    case "REFUNDED":
      return "success"
    case "FAILED":
    case "REFUND_REQUIRED":
    case "REFUND_FAILED":
      return "destructive"
    case "PENDING":
      return "warning"
    case "UNPAID":
      return "secondary"
  }
}

/** Target statuses that the server rejects without a reason. */
export function requiresReason(status: OrderStatus): boolean {
  return status === "CANCELLED" || status === "DELIVERY_FAILED"
}
