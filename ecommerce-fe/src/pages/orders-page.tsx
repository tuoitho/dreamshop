import { useCallback, useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { ExternalLink, Loader2, Package } from "lucide-react"
import { ProductImage } from "@/components/product-image"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cancelOrder, listOrders } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { ORDER_STATUS_TABS, getOrderStatusMeta, isCustomerCancellable } from "@/lib/order-status"
import type { OrderDetail, OrderItemSnapshot } from "@/lib/types"
import { cn, formatVND } from "@/lib/utils"

function OrderItemThumb({ item }: { item: OrderItemSnapshot }) {
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

export default function OrdersPage() {
  const [statusFilter, setStatusFilter] = useState("")
  const [page, setPage] = useState(1)
  const [orders, setOrders] = useState<OrderDetail[]>([])
  const [totalPages, setTotalPages] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [cancellingId, setCancellingId] = useState<string | null>(null)

  const loadOrders = useCallback(async (targetPage: number, targetStatus: string, signal?: AbortSignal) => {
    setLoading(true)
    setError(null)
    try {
      const result = await listOrders(
        { page: targetPage, pageSize: 10, status: targetStatus || undefined },
        signal
      )
      setOrders(result.items)
      setTotalPages(result.totalPages)
    } catch (err) {
      if (signal?.aborted) {
        return
      }
      setError(resolveErrorMessage(err, { fallback: "Không thể tải danh sách đơn mua." }))
    } finally {
      if (!signal?.aborted) {
        setLoading(false)
      }
    }
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    void loadOrders(page, statusFilter, controller.signal)
    return () => controller.abort()
  }, [loadOrders, page, statusFilter])

  async function handleCancelOrder(orderId: string) {
    setCancellingId(orderId)
    try {
      const updated = await cancelOrder(orderId)
      setOrders((prev) => prev.map((item) => (item.orderId === orderId ? updated : item)))
    } catch {
      // Mutation error toast is handled centrally by api.ts.
    } finally {
      setCancellingId(null)
    }
  }

  return (
    <main className="mx-auto max-w-5xl px-4 py-8">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-extrabold tracking-tight">Đơn mua của tôi</h1>
          <p className="text-sm text-muted-foreground">
            Theo dõi trạng thái đơn hàng, bản chụp sản phẩm lúc đặt và lịch sử thanh toán.
          </p>
        </div>
      </div>

      <div className="mb-6 flex flex-wrap gap-2">
        {ORDER_STATUS_TABS.map((tab) => {
          const active = statusFilter === tab.value
          return (
            <button
              key={tab.value || "all"}
              type="button"
              onClick={() => {
                setStatusFilter(tab.value)
                setPage(1)
              }}
              className={cn(
                "rounded-full border px-4 py-1.5 text-sm font-medium transition-colors",
                active
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-border bg-card text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
            >
              {tab.label}
            </button>
          )
        })}
      </div>

      {loading ? (
        <div className="flex items-center justify-center rounded-2xl border bg-card py-16">
          <Loader2 className="mr-2 size-5 animate-spin text-primary" />
          <span className="text-sm text-muted-foreground">Đang tải đơn mua...</span>
        </div>
      ) : error ? (
        <div className="rounded-2xl border bg-card p-8 text-center">
          <p className="text-sm text-destructive">{error}</p>
          <Button
            type="button"
            variant="outline"
            className="mt-4"
            onClick={() => void loadOrders(page, statusFilter)}
          >
            Thử lại
          </Button>
        </div>
      ) : orders.length === 0 ? (
        <div className="rounded-2xl border bg-card p-12 text-center">
          <Package className="mx-auto mb-3 size-12 text-muted-foreground" />
          <h2 className="text-lg font-bold">Chưa có đơn hàng nào</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Các đơn hàng đã đặt sẽ hiển thị đầy đủ tại đây.
          </p>
          <Button asChild className="mt-5">
            <Link to="/">Khám phá sản phẩm</Link>
          </Button>
        </div>
      ) : (
        <div className="space-y-4">
          {orders.map((order) => {
            const statusMeta = getOrderStatusMeta(order.status, order.paymentStatus)
            return (
              <article key={order.orderId} className="rounded-2xl border bg-card p-5">
                <div className="flex flex-wrap items-center justify-between gap-2 border-b pb-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-mono text-sm font-bold">{order.orderCode}</span>
                    <span className="text-xs text-muted-foreground">
                      • {order.paymentMethod === "MOMO" ? "Ví MoMo" : "Thanh toán COD"}
                    </span>
                  </div>
                  <Badge variant="outline" className={statusMeta.className}>
                    {statusMeta.label}
                  </Badge>
                </div>

                <div className="divide-y py-2">
                  {order.items.map((item, idx) => (
                    <div key={`${order.orderId}-${item.cartItemId || `${item.skuId}-${idx}`}`} className="flex items-center gap-3 py-3">
                      <div className="size-14 shrink-0 overflow-hidden rounded-lg border bg-muted">
                        <OrderItemThumb item={item} />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-semibold">{item.expectedName}</p>
                        <p className="text-xs text-muted-foreground">
                          {item.variantLabel ? `Phân loại: ${item.variantLabel} • ` : ""}x{item.quantity}
                        </p>
                      </div>
                      <div className="text-right text-sm font-semibold">
                        {formatVND(item.lineTotal || item.unitPrice * item.quantity)}
                      </div>
                    </div>
                  ))}
                </div>

                <div className="mt-2 flex flex-wrap items-center justify-between gap-3 border-t pt-3">
                  <div>
                    <span className="text-xs text-muted-foreground">Tổng thanh toán: </span>
                    <span className="text-base font-extrabold text-primary">
                      {formatVND(order.totalAmount)}
                    </span>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    {order.status === "PENDING_PAYMENT" && order.payUrl && (
                      <Button size="sm" asChild>
                        <a href={order.payUrl}>
                          Thanh toán MoMo
                          <ExternalLink className="ml-1 size-3.5" />
                        </a>
                      </Button>
                    )}
                    {isCustomerCancellable(order.status) && (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={cancellingId === order.orderId}
                        onClick={() => void handleCancelOrder(order.orderId)}
                      >
                        {cancellingId === order.orderId ? "Đang hủy..." : "Hủy đơn"}
                      </Button>
                    )}
                    <Button size="sm" variant="secondary" asChild>
                      <Link to={`/don-hang/${order.orderId}`}>Xem chi tiết</Link>
                    </Button>
                  </div>
                </div>
              </article>
            )
          })}

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-3 pt-4">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Trang trước
              </Button>
              <span className="text-sm text-muted-foreground">
                Trang {page} / {totalPages}
              </span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Trang sau
              </Button>
            </div>
          )}
        </div>
      )}
    </main>
  )
}
