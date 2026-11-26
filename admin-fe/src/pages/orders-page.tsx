import { ChevronLeft, ChevronRight, Eye, RefreshCw, Search } from "lucide-react"
import { useCallback, useEffect, useState, type FormEvent } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useAuth } from "@/hooks/use-auth"
import { listAdminOrders } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { formatCurrencyVnd, formatDateTime, formatNumber } from "@/lib/format"
import {
  ORDER_STATUS_LABELS,
  ORDER_STATUS_TABS,
  PAYMENT_STATUS_LABELS,
  orderStatusBadgeVariant,
  paymentStatusBadgeVariant,
} from "@/lib/order-status"
import type { AdminOrderPage, ListAdminOrdersParams, OrderStatus } from "@/lib/types"
import { cn } from "@/lib/utils"
import { OrderDetailSheet } from "./order-detail-sheet"

const PAGE_SIZE = 20

export function OrdersPage() {
  const { can } = useAuth()
  const canUpdate = can("order.admin.update")

  const [statusFilter, setStatusFilter] = useState<OrderStatus | undefined>(undefined)
  const [searchDraft, setSearchDraft] = useState("")
  const [appliedQuery, setAppliedQuery] = useState("")
  const [page, setPage] = useState(1)
  const [data, setData] = useState<AdminOrderPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null)

  const loadOrders = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true)
      setErrorMessage(null)
      const params: ListAdminOrdersParams = {}
      if (statusFilter) params.status = statusFilter
      if (appliedQuery) params.q = appliedQuery
      params.page = page
      params.pageSize = PAGE_SIZE
      try {
        const res = await listAdminOrders(params, signal)
        if (signal?.aborted) return
        setData(res)
      } catch (error) {
        if (!signal?.aborted) {
          setErrorMessage(resolveErrorMessage(error))
        }
      } finally {
        if (!signal?.aborted) {
          setLoading(false)
        }
      }
    },
    [statusFilter, appliedQuery, page]
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadOrders(controller.signal)
    return () => controller.abort()
  }, [loadOrders])

  const handleSearchSubmit = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setAppliedQuery(searchDraft.trim())
  }

  const counts = data?.statusCounts ?? {}
  const totalCount = Object.values(counts).reduce((sum, value) => sum + (value ?? 0), 0)
  const orders = data?.items ?? []

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">Quản lý đơn hàng</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Theo dõi đơn theo trạng thái, xem lịch sử xử lý và cập nhật tiến độ giao hàng.
          </p>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={() => void loadOrders()} disabled={loading}>
          <RefreshCw className={loading ? "size-3.5 animate-spin" : "size-3.5"} />
          Làm mới
        </Button>
      </div>

      {errorMessage && (
        <div role="alert" className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900">
          {errorMessage}
        </div>
      )}

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
          <div className="inline-flex flex-wrap items-center gap-1 rounded-lg border border-border bg-muted p-1">
            {ORDER_STATUS_TABS.map((tab) => {
              const active = statusFilter === tab.status
              const count = tab.status ? (counts[tab.status] ?? 0) : totalCount
              return (
                <button
                  key={tab.status ?? "all"}
                  type="button"
                  aria-pressed={active}
                  onClick={() => {
                    setStatusFilter(tab.status)
                    setPage(1)
                  }}
                  className={cn(
                    "inline-flex cursor-pointer items-center gap-2 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                    active
                      ? "border border-border bg-card font-semibold text-foreground shadow-2xs"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  <span>{tab.label}</span>
                  <span className="rounded-full bg-background px-1.5 py-0.5 font-mono text-[11px] tabular-nums">
                    {formatNumber(count)}
                  </span>
                </button>
              )
            })}
          </div>

          <form onSubmit={handleSearchSubmit} className="flex items-center gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={searchDraft}
                onChange={(e) => setSearchDraft(e.target.value)}
                placeholder="Tìm theo mã đơn…"
                aria-label="Tìm theo mã đơn"
                className="w-48 pl-8 text-xs sm:w-56"
              />
            </div>
            <Button type="submit" variant="outline" size="sm">
              Lọc
            </Button>
          </form>
        </div>

        <CardContent className="p-0">
          {loading && !data ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : orders.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Mã đơn</TableHead>
                  <TableHead>Người nhận</TableHead>
                  <TableHead className="text-right">Tổng tiền</TableHead>
                  <TableHead>Trạng thái</TableHead>
                  <TableHead>Thanh toán</TableHead>
                  <TableHead>Ngày tạo</TableHead>
                  <TableHead className="text-right">Thao tác</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.map((order) => (
                  <TableRow key={order.orderId}>
                    <TableCell className="font-mono text-xs font-medium text-foreground">{order.orderCode}</TableCell>
                    <TableCell>
                      <div className="text-sm text-foreground">{order.shippingAddress.recipientName}</div>
                      <div className="font-mono text-[11px] text-muted-foreground">{order.shippingAddress.phone}</div>
                    </TableCell>
                    <TableCell className="text-right font-mono font-semibold tabular-nums">
                      {formatCurrencyVnd(order.totalAmount)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={orderStatusBadgeVariant(order.status)}>
                        {ORDER_STATUS_LABELS[order.status]}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={paymentStatusBadgeVariant(order.paymentStatus)}>
                        {order.paymentMethod} · {PAYMENT_STATUS_LABELS[order.paymentStatus]}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {formatDateTime(order.createdAt)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        aria-label={`Xem đơn ${order.orderCode}`}
                        onClick={() => setSelectedOrderId(order.orderId)}
                      >
                        <Eye className="size-3.5" />
                        Xem
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <div className="px-6 py-12 text-center text-sm text-muted-foreground">Không có đơn hàng phù hợp.</div>
          )}

          {data && data.totalPages > 1 && (
            <div className="flex items-center justify-between border-t border-border px-5 py-3 text-xs text-muted-foreground">
              <span>
                Trang <span className="font-mono font-semibold text-foreground">{data.page}</span> /{" "}
                <span className="font-mono">{data.totalPages}</span> ({formatNumber(data.totalItems)} đơn)
              </span>
              <div className="flex items-center gap-1.5">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  <ChevronLeft className="size-3.5" />
                  Trước
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={page >= data.totalPages}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Sau
                  <ChevronRight className="size-3.5" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <OrderDetailSheet
        open={selectedOrderId !== null}
        onOpenChange={(next) => {
          if (!next) setSelectedOrderId(null)
        }}
        orderId={selectedOrderId}
        canUpdate={canUpdate}
        onMutated={() => void loadOrders()}
      />
    </div>
  )
}
