import { useCallback, useEffect, useState } from "react"
import { Link, useParams, useSearchParams } from "react-router-dom"
import { AlertTriangle, ArrowLeft, CheckCircle2, ExternalLink, Loader2, MapPin } from "lucide-react"
import { ProductImage } from "@/components/product-image"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { cancelOrder, getOrder } from "@/lib/api"
import type { OrderDetail, OrderItemSnapshot } from "@/lib/types"
import { formatVND } from "@/lib/utils"

function DetailItemThumb({ item }: { item: OrderItemSnapshot }) {
  const [imgError, setImgError] = useState(false)
  const hue: [number, number] =
    item.imageHue && item.imageHue.length >= 2
      ? [item.imageHue[0] ?? 210, item.imageHue[1] ?? 250]
      : [210, 250]

  if (item.imageUrl && !imgError) {
    return (
      <img
        src={item.imageUrl}
        alt={item.expectedName}
        className="size-full object-cover"
        onError={() => setImgError(true)}
      />
    )
  }

  return (
    <ProductImage
      emoji={item.imageEmoji || "📦"}
      hue={hue}
      seed={item.skuId || item.productId}
      label={item.expectedName}
    />
  )
}

function formatOrderStatus(status: string): string {
  switch (status) {
    case "CONFIRMED":
      return "Đã xác nhận"
    case "PENDING_PAYMENT":
      return "Chờ thanh toán"
    case "PENDING_STOCK":
      return "Đang giữ kho"
    case "CANCELLED":
      return "Đã hủy"
    default:
      return status
  }
}

function formatPaymentStatus(paymentStatus: string): string {
  switch (paymentStatus) {
    case "PAID":
      return "Đã thanh toán"
    case "PENDING":
      return "Đang chờ thanh toán"
    case "UNPAID":
      return "Thanh toán khi nhận hàng"
    case "FAILED":
      return "Thanh toán thất bại"
    case "REFUND_REQUIRED":
      return "Đang đối soát hoàn tiền"
    default:
      return paymentStatus
  }
}

export default function OrderDetailPage() {
  const { orderId = "" } = useParams()
  const [searchParams] = useSearchParams()
  const justPlaced = searchParams.get("placed") === "1"

  const [order, setOrder] = useState<OrderDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [cancelling, setCancelling] = useState(false)

  const loadDetail = useCallback(async (signal?: AbortSignal) => {
    if (!orderId) {
      setLoading(false)
      setError("Không tìm thấy mã đơn hàng.")
      return
    }
    setLoading(true)
    setError(null)
    try {
      const data = await getOrder(orderId, signal)
      setOrder(data)
    } catch (err) {
      if (signal?.aborted) {
        return
      }
      setError(err instanceof Error ? err.message : "Không thể tải chi tiết đơn hàng.")
    } finally {
      if (!signal?.aborted) {
        setLoading(false)
      }
    }
  }, [orderId])

  useEffect(() => {
    const controller = new AbortController()
    void loadDetail(controller.signal)
    return () => controller.abort()
  }, [loadDetail])

  async function handleCancel() {
    if (!order) {
      return
    }
    setCancelling(true)
    try {
      const updated = await cancelOrder(order.orderId)
      setOrder(updated)
    } catch {
      // Mutation error toast is handled centrally by api.ts.
    } finally {
      setCancelling(false)
    }
  }

  if (loading) {
    return (
      <main className="mx-auto max-w-4xl px-4 py-12">
        <div className="flex items-center justify-center rounded-2xl border bg-card py-16">
          <Loader2 className="mr-2 size-5 animate-spin text-primary" />
          <span className="text-sm text-muted-foreground">Đang tải chi tiết đơn hàng...</span>
        </div>
      </main>
    )
  }

  if (error || !order) {
    return (
      <main className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-2xl border bg-card p-8 text-center">
          <p className="text-sm text-destructive">{error ?? "Không tìm thấy đơn hàng."}</p>
          <Button variant="outline" className="mt-4" asChild>
            <Link to="/don-hang">Quay lại đơn mua</Link>
          </Button>
        </div>
      </main>
    )
  }

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <div className="mb-4">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/don-hang">
            <ArrowLeft className="mr-1 size-4" />
            Đơn mua của tôi
          </Link>
        </Button>
      </div>

      {justPlaced && order.status === "CONFIRMED" && (
        <div className="mb-6 flex items-start gap-3 rounded-2xl border border-emerald-500/30 bg-emerald-500/10 p-4 text-sm text-emerald-900 dark:text-emerald-200">
          <CheckCircle2 className="mt-0.5 size-5 shrink-0 text-emerald-600" />
          <div>
            <p className="font-bold">Đặt hàng thành công!</p>
            <p className="mt-0.5">
              Đơn hàng <span className="font-mono font-semibold">{order.orderCode}</span> đã được ghi nhận và giữ hàng trong kho.
            </p>
          </div>
        </div>
      )}

      {order.status === "CANCELLED" && order.paymentStatus === "REFUND_REQUIRED" && (
        <div className="mb-6 flex items-start gap-3 rounded-2xl border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-900 dark:text-amber-200">
          <AlertTriangle className="mt-0.5 size-5 shrink-0 text-amber-600" />
          <div>
            <p className="font-bold">Đơn hàng đang đối soát hoàn tiền</p>
            <p className="mt-0.5">
              Khoản thanh toán MoMo đến sau thời điểm đơn hàng hết hạn giữ chỗ. Hệ thống đã ghi nhận yêu cầu hoàn tiền cho giao dịch này.
            </p>
          </div>
        </div>
      )}

      <div className="rounded-2xl border bg-card p-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className="text-xs text-muted-foreground">Mã đơn hàng</p>
            <h1 className="font-mono text-xl font-extrabold">{order.orderCode}</h1>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">{formatOrderStatus(order.status)}</Badge>
            <Badge variant="secondary">{formatPaymentStatus(order.paymentStatus)}</Badge>
          </div>
        </div>

        <Separator className="my-5" />

        <div className="grid gap-6 md:grid-cols-2">
          <div>
            <h2 className="mb-2 flex items-center gap-1.5 text-sm font-bold">
              <MapPin className="size-4 text-primary" />
              Địa chỉ nhận hàng
            </h2>
            <p className="text-sm font-semibold">
              {order.shippingAddress.recipientName} • {order.shippingAddress.phone}
            </p>
            <p className="mt-1 text-sm text-muted-foreground">
              {order.shippingAddress.addressLine}, {order.shippingAddress.wardName},{" "}
              {order.shippingAddress.provinceName}
            </p>
            {order.note && (
              <p className="mt-2 text-xs text-muted-foreground">Ghi chú: {order.note}</p>
            )}
          </div>

          <div>
            <h2 className="mb-2 text-sm font-bold">Thông tin thanh toán</h2>
            <p className="text-sm">
              Phương thức:{" "}
              <span className="font-semibold">
                {order.paymentMethod === "MOMO" ? "Ví điện tử MoMo" : "Thanh toán khi nhận hàng (COD)"}
              </span>
            </p>
            <p className="mt-1 text-sm text-muted-foreground">
              Trạng thái thanh toán: {formatPaymentStatus(order.paymentStatus)}
            </p>
            {order.cancelReason && (
              <p className="mt-1 text-xs text-destructive">Lý do hủy: {order.cancelReason}</p>
            )}
          </div>
        </div>

        <Separator className="my-5" />

        <h2 className="mb-3 text-sm font-bold">Sản phẩm đã đặt</h2>
        <div className="divide-y">
          {order.items.map((item, idx) => (
            <div key={item.cartItemId || `${item.skuId}-${idx}`} className="flex items-center gap-3 py-3">
              <div className="size-16 shrink-0 overflow-hidden rounded-lg border bg-muted">
                <DetailItemThumb item={item} />
              </div>
              <div className="min-w-0 flex-1">
                <Link
                  to={`/san-pham/${item.productId}`}
                  className="truncate text-sm font-semibold hover:text-primary"
                >
                  {item.expectedName}
                </Link>
                <p className="text-xs text-muted-foreground">
                  {item.variantLabel ? `${item.variantLabel} • ` : ""}Đơn giá {formatVND(item.unitPrice)} × {item.quantity}
                </p>
              </div>
              <div className="text-right text-sm font-bold">
                {formatVND(item.lineTotal || item.unitPrice * item.quantity)}
              </div>
            </div>
          ))}
        </div>

        <Separator className="my-5" />

        <div className="ml-auto max-w-xs space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Tạm tính</span>
            <span>{formatVND(order.subtotal)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Phí vận chuyển</span>
            <span>{order.shippingFee === 0 ? "Miễn phí" : formatVND(order.shippingFee)}</span>
          </div>
          <Separator />
          <div className="flex justify-between text-base font-extrabold">
            <span>Tổng cộng</span>
            <span className="text-primary">{formatVND(order.totalAmount)}</span>
          </div>
        </div>

        {order.status === "PENDING_PAYMENT" && (
          <div className="mt-6 flex flex-wrap justify-end gap-3 border-t pt-4">
            {order.payUrl && (
              <Button asChild>
                <a href={order.payUrl}>
                  Thanh toán qua MoMo
                  <ExternalLink className="ml-1 size-4" />
                </a>
              </Button>
            )}
            <Button
              type="button"
              variant="outline"
              disabled={cancelling}
              onClick={() => void handleCancel()}
            >
              {cancelling ? "Đang hủy..." : "Hủy đơn hàng"}
            </Button>
          </div>
        )}
      </div>
    </main>
  )
}
