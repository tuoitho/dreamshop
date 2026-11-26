import { AlertCircle } from "lucide-react"
import { useEffect, useState, type FormEvent } from "react"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { toClientError, updateAdminOrderStatus } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { ORDER_STATUS_LABELS, requiresReason } from "@/lib/order-status"
import type { AdminOrder, AdminOrderView, OrderStatus } from "@/lib/types"
import { cn } from "@/lib/utils"

interface OrderStatusDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  order: AdminOrder
  allowedNextStatuses: OrderStatus[]
  onUpdated: (view: AdminOrderView) => void
  /** Called when another operator changed the order first, so the caller can reload it. */
  onConflict: () => void
}

const REASON_MAX_LENGTH = 128
const SHIPPING_FIELD_MAX_LENGTH = 100

export function OrderStatusDialog({
  open,
  onOpenChange,
  order,
  allowedNextStatuses,
  onUpdated,
  onConflict,
}: OrderStatusDialogProps) {
  const [target, setTarget] = useState<OrderStatus | null>(null)
  const [reason, setReason] = useState("")
  const [carrier, setCarrier] = useState("")
  const [trackingNumber, setTrackingNumber] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  useEffect(() => {
    if (open) {
      setTarget(null)
      setReason("")
      setCarrier("")
      setTrackingNumber("")
      setErrorMessage(null)
    }
  }, [open, order.orderId])

  const reasonRequired = target !== null && requiresReason(target)
  const reasonMissing = reasonRequired && reason.trim() === ""
  const canSubmit = target !== null && !reasonMissing && !submitting

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    if (!target || reasonMissing) return
    setSubmitting(true)
    setErrorMessage(null)
    try {
      const view = await updateAdminOrderStatus(order.orderId, {
        expectedStatus: order.status,
        targetStatus: target,
        reason: reason.trim(),
        shippingCarrier: target === "SHIPPING" ? carrier.trim() : "",
        trackingNumber: target === "SHIPPING" ? trackingNumber.trim() : "",
      })
      onUpdated(view)
      onOpenChange(false)
    } catch (error) {
      const clientError = toClientError(error)
      if (clientError.code === "ORDER_STATUS_CONFLICT") {
        setErrorMessage("Trạng thái đơn vừa thay đổi, vui lòng kiểm tra lại trước khi cập nhật.")
        onConflict()
      } else {
        setErrorMessage(resolveErrorMessage(clientError))
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Cập nhật trạng thái đơn</DialogTitle>
          <DialogDescription>
            Đơn <span className="font-mono">{order.orderCode}</span> đang ở trạng thái{" "}
            <span className="font-medium text-foreground">{ORDER_STATUS_LABELS[order.status]}</span>.
          </DialogDescription>
        </DialogHeader>

        {errorMessage && (
          <div
            role="alert"
            className="flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-xs font-medium text-red-900"
          >
            <AlertCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
            <span>{errorMessage}</span>
          </div>
        )}

        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-foreground">Chuyển sang</legend>
            <div className="grid gap-2 sm:grid-cols-2">
              {allowedNextStatuses.map((status) => (
                <label
                  key={status}
                  className={cn(
                    "flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors",
                    target === status
                      ? "border-primary bg-primary/5 font-medium text-foreground"
                      : "border-border text-muted-foreground hover:text-foreground"
                  )}
                >
                  <input
                    type="radio"
                    name="target-status"
                    value={status}
                    checked={target === status}
                    onChange={() => setTarget(status)}
                    className="accent-primary"
                  />
                  {ORDER_STATUS_LABELS[status]}
                </label>
              ))}
            </div>
          </fieldset>

          {target === "SHIPPING" && (
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label htmlFor="order-shipping-carrier">Đơn vị vận chuyển</Label>
                <Input
                  id="order-shipping-carrier"
                  value={carrier}
                  maxLength={SHIPPING_FIELD_MAX_LENGTH}
                  onChange={(e) => setCarrier(e.target.value)}
                  placeholder="GHN, GHTK…"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="order-tracking-number">Mã vận đơn</Label>
                <Input
                  id="order-tracking-number"
                  value={trackingNumber}
                  maxLength={SHIPPING_FIELD_MAX_LENGTH}
                  onChange={(e) => setTrackingNumber(e.target.value)}
                  className="font-mono"
                />
              </div>
            </div>
          )}

          {target !== null && (
            <div className="space-y-1.5">
              <Label htmlFor="order-status-reason">
                Lý do {reasonRequired ? "" : "(không bắt buộc)"}
              </Label>
              <Textarea
                id="order-status-reason"
                value={reason}
                maxLength={REASON_MAX_LENGTH}
                onChange={(e) => setReason(e.target.value)}
                rows={3}
              />
              {reasonMissing && (
                <p className="text-xs text-amber-800">
                  Bắt buộc nhập lý do khi chuyển sang {ORDER_STATUS_LABELS[target]}.
                </p>
              )}
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Hủy
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {submitting ? "Đang cập nhật…" : "Xác nhận"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
