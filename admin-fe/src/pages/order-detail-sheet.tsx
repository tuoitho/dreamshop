import { AlertCircle, CheckCircle2, Loader2, RefreshCw, RotateCcw } from "lucide-react"
import { useCallback, useEffect, useState, type FormEvent } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import { getAdminOrder, retryAdminOrderRefund, toClientError } from "@/lib/api"
import { formatCurrencyVnd, formatDateTime } from "@/lib/format"
import {
  ACTOR_LABELS,
  ORDER_STATUS_LABELS,
  PAYMENT_STATUS_LABELS,
  orderStatusBadgeVariant,
  paymentStatusBadgeVariant,
} from "@/lib/order-status"
import type { AdminOrderView } from "@/lib/types"
import { OrderStatusDialog } from "./order-status-dialog"

interface OrderDetailSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  orderId: string | null
  canUpdate: boolean
  onMutated: () => void
}

export function OrderDetailSheet({ open, onOpenChange, orderId, canUpdate, onMutated }: OrderDetailSheetProps) {
  const [view, setView] = useState<AdminOrderView | null>(null)
  const [loading, setLoading] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [refundDialogOpen, setRefundDialogOpen] = useState(false)
  const [refundReason, setRefundReason] = useState("")
  const [refundSubmitting, setRefundSubmitting] = useState(false)

  const loadOrder = useCallback(
    async (signal?: AbortSignal) => {
      if (!orderId) return
      setLoading(true)
      setErrorMessage(null)
      try {
        const res = await getAdminOrder(orderId, signal)
        if (!signal?.aborted) setView(res)
      } catch (error) {
        if (!signal?.aborted) setErrorMessage(toClientError(error).message)
      } finally {
        if (!signal?.aborted) setLoading(false)
      }
    },
    [orderId]
  )

  useEffect(() => {
    if (!open || !orderId) {
      setView(null)
      setErrorMessage(null)
      return
    }
    const controller = new AbortController()
    void loadOrder(controller.signal)
    return () => controller.abort()
  }, [open, orderId, loadOrder])

  const order = view?.order
  const address = order?.shippingAddress
  const canChangeStatus = canUpdate && (view?.allowedNextStatuses.length ?? 0) > 0
  const canManualRefund =
    canUpdate &&
    order?.status === "CANCELLED" &&
    order?.paymentMethod === "MOMO" &&
    order?.paymentStatus === "REFUND_FAILED"

  const handleManualRefund = async (event: FormEvent) => {
    event.preventDefault()
    if (!order || refundSubmitting) return
    setRefundSubmitting(true)
    try {
      const next = await retryAdminOrderRefund(order.orderId, refundReason)
      setView(next)
      setRefundDialogOpen(false)
      setRefundReason("")
      onMutated()
    } catch {
      void loadOrder()
    } finally {
      setRefundSubmitting(false)
    }
  }

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className="sm:max-w-[600px]">
          <SheetHeader>
            <SheetTitle>
              Đơn hàng <span className="font-mono">{order?.orderCode ?? ""}</span>
            </SheetTitle>
            <SheetDescription>Chi tiết đơn, thông tin giao hàng và lịch sử xử lý.</SheetDescription>
          </SheetHeader>

          {loading && !order ? (
            <div className="flex flex-1 items-center justify-center p-10 text-sm text-muted-foreground">
              <Loader2 className="mr-2 size-4 animate-spin" />
              Đang tải đơn hàng…
            </div>
          ) : order && address ? (
            <div className="flex-1 space-y-6 overflow-y-auto px-6 py-5">
              {errorMessage && (
                <div
                  role="alert"
                  className="flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-xs font-medium text-red-900"
                >
                  <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
                  <span>{errorMessage}</span>
                </div>
              )}

              <section className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={orderStatusBadgeVariant(order.status)}>
                    {ORDER_STATUS_LABELS[order.status]}
                  </Badge>
                  <Badge variant={paymentStatusBadgeVariant(order.paymentStatus)}>
                    {order.paymentMethod} · {PAYMENT_STATUS_LABELS[order.paymentStatus]}
                  </Badge>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => void loadOrder()}
                    disabled={loading}
                  >
                    <RefreshCw className={loading ? "size-3.5 animate-spin" : "size-3.5"} />
                    Làm mới
                  </Button>
                  {canChangeStatus && (
                    <Button type="button" size="sm" onClick={() => setDialogOpen(true)}>
                      Cập nhật trạng thái
                    </Button>
                  )}
                  {canManualRefund && (
                    <Button
                      type="button"
                      size="sm"
                      variant="destructive"
                      onClick={() => {
                        setRefundReason("")
                        setRefundDialogOpen(true)
                      }}
                      disabled={refundSubmitting}
                    >
                      <RotateCcw className="size-3.5" />
                      Hoàn tiền thủ công
                    </Button>
                  )}
                </div>
              </section>

              {order.paymentStatus === "REFUND_FAILED" && (
                <div
                  role="alert"
                  className="flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-xs text-red-900"
                >
                  <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
                  <div className="space-y-1">
                    <p className="font-semibold">Hoàn tiền tự động thất bại sau 3 lần thử.</p>
                    {order.refundLastError && (
                      <p className="text-red-800">Lỗi gần nhất: {order.refundLastError}</p>
                    )}
                  </div>
                </div>
              )}

              {order.paymentStatus === "REFUNDED" && (
                <div className="flex items-start gap-2.5 rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-2.5 text-xs text-emerald-900">
                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600" />
                  <div className="space-y-1">
                    <p className="font-semibold">Đã hoàn tiền MoMo thành công</p>
                    {order.refundTransId && (
                      <p>
                        Mã giao dịch hoàn tiền: <span className="font-mono font-medium">{order.refundTransId}</span>
                      </p>
                    )}
                    {order.refundedAt && <p>Thời gian hoàn tiền: {formatDateTime(order.refundedAt)}</p>}
                    {order.refundActorType && (
                      <p>Thực hiện bởi: {ACTOR_LABELS[order.refundActorType]}</p>
                    )}
                    {order.refundReason && <p>Ghi chú hoàn tiền: {order.refundReason}</p>}
                  </div>
                </div>
              )}

              <section className="space-y-1 text-sm">
                <h3 className="font-semibold text-foreground">Giao hàng</h3>
                <p className="text-foreground">
                  {address.recipientName} · <span className="font-mono">{address.phone}</span>
                </p>
                <p className="text-muted-foreground">
                  {[address.addressLine, address.wardName, address.provinceName].filter(Boolean).join(", ")}
                </p>
                {(order.shippingCarrier || order.trackingNumber) && (
                  <p className="text-muted-foreground">
                    Vận chuyển: <span className="text-foreground">{order.shippingCarrier || "—"}</span> · Mã
                    vận đơn: <span className="font-mono text-foreground">{order.trackingNumber || "—"}</span>
                  </p>
                )}
                {order.note && <p className="text-muted-foreground">Ghi chú: {order.note}</p>}
                {order.cancelReason && (
                  <p className="text-red-700">Lý do hủy: {order.cancelReason}</p>
                )}
              </section>

              <section className="space-y-2">
                <h3 className="text-sm font-semibold text-foreground">Sản phẩm</h3>
                <ul className="divide-y divide-border rounded-lg border border-border">
                  {order.items.map((item) => (
                    <li key={item.skuId} className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
                      <div className="min-w-0">
                        <div className="truncate font-medium text-foreground">{item.expectedName}</div>
                        <div className="text-xs text-muted-foreground">
                          {item.variantLabel} · <span className="font-mono">{item.skuId}</span>
                        </div>
                      </div>
                      <div className="shrink-0 text-right font-mono text-xs tabular-nums">
                        <div>
                          {formatCurrencyVnd(item.unitPrice)} × {item.quantity}
                        </div>
                        <div className="font-semibold text-foreground">{formatCurrencyVnd(item.lineTotal)}</div>
                      </div>
                    </li>
                  ))}
                </ul>
                <div className="space-y-1 text-sm">
                  <div className="flex justify-between text-muted-foreground">
                    <span>Tạm tính</span>
                    <span className="font-mono tabular-nums">{formatCurrencyVnd(order.subtotal)}</span>
                  </div>
                  <div className="flex justify-between text-muted-foreground">
                    <span>Phí vận chuyển</span>
                    <span className="font-mono tabular-nums">{formatCurrencyVnd(order.shippingFee)}</span>
                  </div>
                  <Separator />
                  <div className="flex justify-between font-semibold text-foreground">
                    <span>Tổng cộng</span>
                    <span className="font-mono tabular-nums">{formatCurrencyVnd(order.totalAmount)}</span>
                  </div>
                </div>
              </section>

              <section className="space-y-2">
                <h3 id="order-timeline-heading" className="text-sm font-semibold text-foreground">
                  Lịch sử trạng thái
                </h3>
                {order.statusHistory.length === 0 ? (
                  <p className="text-xs text-muted-foreground">Chưa có lịch sử được ghi nhận.</p>
                ) : null}
                <ol aria-labelledby="order-timeline-heading" className="space-y-3 border-l border-border pl-4">
                  {order.statusHistory.map((change, index) => (
                    <li key={`${change.createdAt}-${index}`} className="text-sm">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-foreground">{ORDER_STATUS_LABELS[change.toStatus]}</span>
                        <span className="text-xs text-muted-foreground">
                          {ACTOR_LABELS[change.actorType]} · {formatDateTime(change.createdAt)}
                        </span>
                      </div>
                      {(change.shippingCarrier || change.trackingNumber) && (
                        <div className="text-xs text-muted-foreground">
                          {change.shippingCarrier} {change.trackingNumber && `· ${change.trackingNumber}`}
                        </div>
                      )}
                      {change.reason && <div className="text-xs text-muted-foreground">{change.reason}</div>}
                    </li>
                  ))}
                </ol>
              </section>
            </div>
          ) : errorMessage ? (
            <div role="alert" className="m-6 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900">
              {errorMessage}
            </div>
          ) : null}
        </SheetContent>
      </Sheet>

      {order && view && (
        <OrderStatusDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          order={order}
          allowedNextStatuses={view.allowedNextStatuses}
          onUpdated={(next) => {
            setView(next)
            onMutated()
          }}
          onConflict={() => {
            void loadOrder()
            onMutated()
          }}
        />
      )}

      {order && (
        <Dialog open={refundDialogOpen} onOpenChange={setRefundDialogOpen}>
          <DialogContent className="sm:max-w-[440px]">
            <DialogHeader>
              <DialogTitle>Xác nhận hoàn tiền thủ công</DialogTitle>
              <DialogDescription>
                Gửi yêu cầu hoàn tiền toàn bộ đơn{" "}
                <span className="font-mono font-medium text-foreground">{order.orderCode}</span> (
                <span className="font-mono font-medium text-foreground">{formatCurrencyVnd(order.totalAmount)}</span>) qua
                MoMo.
              </DialogDescription>
            </DialogHeader>
            <form onSubmit={(e) => void handleManualRefund(e)} className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="manual-refund-reason">Lý do hoàn tiền (không bắt buộc)</Label>
                <Textarea
                  id="manual-refund-reason"
                  value={refundReason}
                  maxLength={128}
                  onChange={(e) => setRefundReason(e.target.value)}
                  rows={3}
                  placeholder="Ví dụ: MoMo đã hoạt động trở lại"
                />
              </div>
              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setRefundDialogOpen(false)}
                  disabled={refundSubmitting}
                >
                  Hủy
                </Button>
                <Button type="submit" variant="destructive" disabled={refundSubmitting}>
                  {refundSubmitting ? "Đang hoàn tiền…" : "Xác nhận hoàn tiền"}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      )}
    </>
  )
}
