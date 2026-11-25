import { useEffect, useRef, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { AlertTriangle, CheckCircle2, Clock, Loader2, XCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useCart } from "@/hooks/use-cart"
import { getOrder } from "@/lib/api"
import type { OrderDetail } from "@/lib/types"
import { formatVND } from "@/lib/utils"

const MAX_POLL_ATTEMPTS = 20
const POLL_INTERVAL_MS = 500

export default function PaymentResultPage() {
  const [searchParams] = useSearchParams()
  const orderId = searchParams.get("orderId")?.trim() ?? ""
  const redirectResult = searchParams.get("result")?.trim() ?? ""
  const { retryLoad } = useCart()

  const [order, setOrder] = useState<OrderDetail | null>(null)
  const [loading, setLoading] = useState(() => Boolean(orderId))
  const [pollingExhausted, setPollingExhausted] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [reloadNonce, setReloadNonce] = useState(0)
  const cartRefreshedRef = useRef(false)

  useEffect(() => {
    if (!orderId) {
      return
    }

    let cancelled = false
    let timerId: ReturnType<typeof setTimeout> | null = null
    let attempts = 0

    async function pollOrder() {
      try {
        const detail = await getOrder(orderId)
        if (cancelled) {
          return
        }
        setOrder(detail)
        setLoading(false)
        setError(null)

        if (detail.status === "CONFIRMED") {
          if (!cartRefreshedRef.current) {
            cartRefreshedRef.current = true
            void retryLoad()
          }
          return
        }

        if (detail.status === "PENDING_PAYMENT" || detail.status === "PENDING_STOCK") {
          if (attempts < MAX_POLL_ATTEMPTS) {
            attempts += 1
            timerId = setTimeout(() => {
              void pollOrder()
            }, POLL_INTERVAL_MS)
          } else {
            setPollingExhausted(true)
          }
          return
        }
      } catch (err) {
        if (!cancelled) {
          setLoading(false)
          setError(err instanceof Error ? err.message : "Không thể tải thông tin đơn hàng.")
        }
      }
    }

    setLoading(true)
    setPollingExhausted(false)
    void pollOrder()

    return () => {
      cancelled = true
      if (timerId !== null) {
        clearTimeout(timerId)
      }
    }
  }, [orderId, reloadNonce, retryLoad])

  if (!orderId) {
    return (
      <main className="mx-auto max-w-2xl px-4 py-12">
        <div className="rounded-2xl border bg-card p-8 text-center">
          <AlertTriangle className="mx-auto mb-3 size-12 text-amber-500" />
          <h1 className="text-xl font-bold">Không tìm thấy thông tin giao dịch</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Đường dẫn kết quả thanh toán thiếu mã đơn hàng hợp lệ.
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button asChild>
              <Link to="/don-hang">Xem đơn mua</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/">Về trang chủ</Link>
            </Button>
          </div>
        </div>
      </main>
    )
  }

  const isSynchronizing =
    loading ||
    (!pollingExhausted &&
      order !== null &&
      (order.status === "PENDING_PAYMENT" || order.status === "PENDING_STOCK"))

  if (isSynchronizing) {
    return (
      <main className="mx-auto max-w-2xl px-4 py-12">
        <div className="rounded-2xl border bg-card p-8 text-center">
          <Loader2 className="mx-auto mb-4 size-12 animate-spin text-primary" />
          <h1 className="text-xl font-bold">Đang đồng bộ kết quả thanh toán...</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Hệ thống đang xác nhận trạng thái giao dịch cho đơn hàng của bạn. Vui lòng giữ trang trong giây lát.
          </p>
          {order?.orderCode && (
            <p className="mt-3 text-xs font-medium text-muted-foreground">
              Mã đơn hàng: <span className="font-mono text-foreground">{order.orderCode}</span>
            </p>
          )}
        </div>
      </main>
    )
  }

  if (error && !order) {
    return (
      <main className="mx-auto max-w-2xl px-4 py-12">
        <div className="rounded-2xl border bg-card p-8 text-center">
          <AlertTriangle className="mx-auto mb-3 size-12 text-destructive" />
          <h1 className="text-xl font-bold">Chưa thể tải kết quả đơn hàng</h1>
          <p className="mt-2 text-sm text-muted-foreground">{error}</p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button type="button" onClick={() => setReloadNonce((n) => n + 1)}>
              Thử lại
            </Button>
            <Button variant="outline" asChild>
              <Link to="/don-hang">Danh sách đơn mua</Link>
            </Button>
          </div>
        </div>
      </main>
    )
  }

  if (order?.status === "CONFIRMED") {
    return (
      <main className="mx-auto max-w-2xl px-4 py-12">
        <div className="rounded-2xl border border-emerald-500/30 bg-card p-8 text-center">
          <CheckCircle2 className="mx-auto mb-4 size-14 text-emerald-600" />
          <h1 className="text-2xl font-extrabold tracking-tight">Thanh toán thành công</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Đơn hàng <span className="font-mono font-semibold text-foreground">{order.orderCode}</span> đã được xác nhận.
          </p>
          <div className="mx-auto mt-5 max-w-sm rounded-xl bg-muted/50 p-4 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Tổng thanh toán</span>
              <span className="font-bold text-primary">{formatVND(order.totalAmount)}</span>
            </div>
            <div className="mt-2 flex items-center justify-between">
              <span className="text-muted-foreground">Phương thức</span>
              <span className="font-medium">{order.paymentMethod === "MOMO" ? "Ví MoMo" : "COD"}</span>
            </div>
          </div>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button asChild>
              <Link to={`/don-hang/${order.orderId}`}>Xem chi tiết đơn hàng</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/">Tiếp tục mua sắm</Link>
            </Button>
          </div>
        </div>
      </main>
    )
  }

  if (order?.status === "CANCELLED" && order.paymentStatus === "REFUNDED") {
    return (
      <main className="mx-auto max-w-2xl px-4 py-12">
        <div className="rounded-2xl border border-emerald-500/30 bg-emerald-500/5 p-8 text-center">
          <CheckCircle2 className="mx-auto mb-4 size-14 text-emerald-600" />
          <h1 className="text-2xl font-extrabold tracking-tight">Đã hoàn tiền</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Khoản thanh toán MoMo cho đơn{" "}
            <span className="font-mono font-semibold text-foreground">{order.orderCode}</span> đã được hoàn lại thành công.
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button asChild>
              <Link to={`/don-hang/${order.orderId}`}>Xem chi tiết đơn hàng</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/don-hang">Đơn mua của tôi</Link>
            </Button>
          </div>
        </div>
      </main>
    )
  }

  if (
    order?.status === "CANCELLED" &&
    (order.paymentStatus === "REFUND_REQUIRED" || order.paymentStatus === "REFUND_FAILED")
  ) {
    return (
      <main className="mx-auto max-w-2xl px-4 py-12">
        <div className="rounded-2xl border border-amber-500/40 bg-amber-500/5 p-8 text-center">
          <AlertTriangle className="mx-auto mb-4 size-14 text-amber-600" />
          <h1 className="text-2xl font-extrabold tracking-tight">Đơn hàng đang xử lý hoàn tiền</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Hệ thống đã ghi nhận yêu cầu đối soát MoMo cho đơn{" "}
            <span className="font-mono font-semibold text-foreground">{order.orderCode}</span> và đang tiến hành xử lý.
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button asChild>
              <Link to={`/don-hang/${order.orderId}`}>Xem chi tiết đơn hàng</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/don-hang">Đơn mua của tôi</Link>
            </Button>
          </div>
        </div>
      </main>
    )
  }

  if (order?.status === "CANCELLED") {
    return (
      <main className="mx-auto max-w-2xl px-4 py-12">
        <div className="rounded-2xl border border-destructive/30 bg-card p-8 text-center">
          <XCircle className="mx-auto mb-4 size-14 text-destructive" />
          <h1 className="text-2xl font-extrabold tracking-tight">Giao dịch không thành công</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Đơn hàng <span className="font-mono font-semibold text-foreground">{order.orderCode}</span> đã bị hủy và lượng hàng giữ chỗ đã được hoàn trả về kho.
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button asChild>
              <Link to={`/don-hang/${order.orderId}`}>Xem chi tiết đơn hàng</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/gio-hang">Quay lại giỏ hàng</Link>
            </Button>
          </div>
        </div>
      </main>
    )
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-12">
      <div className="rounded-2xl border bg-card p-8 text-center">
        <Clock className="mx-auto mb-4 size-14 text-amber-500" />
        <h1 className="text-xl font-bold">
          {redirectResult === "failed" ? "Thanh toán chưa hoàn tất" : "Đang chờ xác nhận từ cổng thanh toán"}
        </h1>
        {order && (
          <p className="mt-2 text-sm text-muted-foreground">
            Đơn hàng <span className="font-mono font-semibold text-foreground">{order.orderCode}</span> hiện ở trạng thái chờ thanh toán ({formatVND(order.totalAmount)}).
          </p>
        )}
        <div className="mt-6 flex flex-wrap justify-center gap-3">
          {order?.payUrl && (
            <Button asChild>
              <a href={order.payUrl}>Thanh toán lại qua MoMo</a>
            </Button>
          )}
          <Button
            type="button"
            variant="outline"
            onClick={() => setReloadNonce((n) => n + 1)}
          >
            Kiểm tra lại
          </Button>
          {order && (
            <Button variant="ghost" asChild>
              <Link to={`/don-hang/${order.orderId}`}>Xem đơn hàng</Link>
            </Button>
          )}
        </div>
      </div>
    </main>
  )
}
