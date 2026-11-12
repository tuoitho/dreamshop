import type { OrderStatus } from "./types"

const STATUS_LABELS: Record<OrderStatus, string> = {
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

const ACTOR_LABELS: Record<string, string> = {
  SYSTEM: "Hệ thống",
  CUSTOMER: "Bạn",
  ADMIN: "Cửa hàng",
}

/** Status tabs on the order history page; an empty value lists every order. */
export const ORDER_STATUS_TABS: Array<{ value: OrderStatus | ""; label: string }> = [
  { value: "", label: "Tất cả" },
  { value: "PENDING_PAYMENT", label: STATUS_LABELS.PENDING_PAYMENT },
  { value: "PENDING_CONFIRMATION", label: STATUS_LABELS.PENDING_CONFIRMATION },
  { value: "CONFIRMED", label: STATUS_LABELS.CONFIRMED },
  { value: "PREPARING", label: STATUS_LABELS.PREPARING },
  { value: "SHIPPING", label: STATUS_LABELS.SHIPPING },
  { value: "DELIVERED", label: STATUS_LABELS.DELIVERED },
  { value: "CANCELLED", label: STATUS_LABELS.CANCELLED },
]

/**
 * Statuses for which the cancel action is offered. The server decides whether a
 * cancellation is still allowed; this only avoids showing a button that cannot work.
 */
const CUSTOMER_CANCELLABLE = new Set<string>(["PENDING_PAYMENT", "PENDING_CONFIRMATION", "CONFIRMED"])

export function isCustomerCancellable(status: string): boolean {
  return CUSTOMER_CANCELLABLE.has(status)
}

export function orderStatusLabel(status: string): string {
  return STATUS_LABELS[status as OrderStatus] ?? status
}

export function actorLabel(actorType: string): string {
  return ACTOR_LABELS[actorType] ?? actorType
}

export function getOrderStatusMeta(status: string, paymentStatus: string): { label: string; className: string } {
  if (status === "CANCELLED" && paymentStatus === "REFUND_REQUIRED") {
    return {
      label: "Đã hủy • Cần hoàn tiền",
      className: "border-amber-500/40 bg-amber-500/15 text-amber-800 dark:text-amber-300",
    }
  }
  switch (status) {
    case "CONFIRMED":
    case "PREPARING":
    case "SHIPPING":
      return {
        label: orderStatusLabel(status),
        className: "border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-400",
      }
    case "DELIVERED":
      return {
        label: orderStatusLabel(status),
        className: "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
      }
    case "PENDING_PAYMENT":
    case "PENDING_CONFIRMATION":
      return {
        label: orderStatusLabel(status),
        className: "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400",
      }
    case "CANCELLED":
    case "DELIVERY_FAILED":
      return {
        label: orderStatusLabel(status),
        className: "border-destructive/30 bg-destructive/10 text-destructive",
      }
    default:
      return {
        label: "Đang xử lý",
        className: "border-border bg-muted text-muted-foreground",
      }
  }
}
