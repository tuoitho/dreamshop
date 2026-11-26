import {
  AlertTriangle,
  ArrowRight,
  ArrowUpRight,
  Boxes,
  ClipboardList,
  Package,
  RefreshCw,
  ShieldCheck,
  SlidersHorizontal,
  Users,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useState, type ComponentType } from "react"
import { Link } from "react-router-dom"
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
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
import {
  listAdminInventory,
  listAdminOrders,
  listAdminProducts,
  listAdminUsers,
  listRbacItems,
} from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { formatCurrencyVnd, formatDateTime, formatNumber } from "@/lib/format"
import { ORDER_STATUS_LABELS, orderStatusBadgeVariant } from "@/lib/order-status"
import type {
  AdminInventoryItem,
  AdminInventoryPage,
  AdminOrderPage,
  AdminProductsPage,
  AdminUserPage,
  OrderStatus,
  RbacItem,
} from "@/lib/types"
import { cn } from "@/lib/utils"
import { StockDialog, type StockDialogTarget } from "./stock-dialog"

const STOCK_COLORS = {
  inStock: "#0d9488",
  lowStock: "#d97706",
  outOfStock: "#dc2626",
}

const ORDER_BUCKET_COLORS = {
  completed: "#0d9488",
  processing: "#2563eb",
  pending: "#ea580c",
  cancelled: "#dc2626",
}

function safePercent(part: number, total: number): number {
  if (!Number.isFinite(part) || !Number.isFinite(total) || total <= 0) return 0
  return Math.max(0, Math.min(100, Math.round((part / total) * 100)))
}

function sumStatuses(
  counts: Partial<Record<OrderStatus, number>> | undefined,
  statuses: OrderStatus[]
): number {
  if (!counts) return 0
  return statuses.reduce((acc, status) => acc + (counts[status] ?? 0), 0)
}

interface KpiCardProps {
  label: string
  value: string
  sublabel: string
  ratioPercent: number
  ratioLabel: string
  to: string
  linkLabel: string
  icon: ComponentType<{ className?: string }>
  iconClassName: string
  barColor: string
  loading: boolean
}

function KpiCard({
  label,
  value,
  sublabel,
  ratioPercent,
  ratioLabel,
  to,
  linkLabel,
  icon: Icon,
  iconClassName,
  barColor,
  loading,
}: KpiCardProps) {
  return (
    <Card className="group hover:shadow-[0_2px_6px_rgba(16,24,40,0.06),0_10px_24px_-12px_rgba(16,24,40,0.1)]">
      <CardContent className="flex h-full flex-col justify-between gap-4 p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="text-xs font-medium text-muted-foreground">{label}</div>
            {loading ? (
              <Skeleton className="mt-2.5 h-8 w-24" />
            ) : (
              <div className="mt-1.5 font-mono text-[28px] font-semibold leading-none tracking-tight text-foreground tabular-nums">
                {value}
              </div>
            )}
          </div>
          <div
            className={cn(
              "flex size-10 shrink-0 items-center justify-center rounded-xl transition-transform duration-200 group-hover:scale-105",
              iconClassName
            )}
          >
            <Icon className="size-[18px]" />
          </div>
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between text-[11px]">
            <span className="truncate text-muted-foreground">{ratioLabel}</span>
            <span className="font-mono font-semibold text-foreground tabular-nums">
              {ratioPercent}%
            </span>
          </div>
          <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full transition-all duration-300"
              style={{ width: `${ratioPercent}%`, backgroundColor: barColor }}
            />
          </div>
        </div>

        <div className="flex items-center justify-between border-t border-border/75 pt-3 text-xs">
          <span className="truncate text-muted-foreground">{sublabel}</span>
          <Link
            to={to}
            className="inline-flex shrink-0 items-center gap-0.5 font-medium text-foreground transition-colors hover:text-teal-700"
          >
            {linkLabel}
            <ArrowUpRight className="size-3.5" />
          </Link>
        </div>
      </CardContent>
    </Card>
  )
}

export function OverviewPage() {
  const { user, can } = useAuth()
  const canViewProducts = can("product.view")
  const canViewOrders = can("order.admin.view")
  const canViewInventory = can("inventory.view")
  const canUpdateInventory = can("inventory.update")
  const canViewUsers = can("user.view")
  const canViewRbac = can("rbac.view")

  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [productsData, setProductsData] = useState<AdminProductsPage | null>(null)
  const [ordersData, setOrdersData] = useState<AdminOrderPage | null>(null)
  const [inventoryData, setInventoryData] = useState<AdminInventoryPage | null>(null)
  const [watchlistItems, setWatchlistItems] = useState<AdminInventoryItem[]>([])
  const [usersData, setUsersData] = useState<AdminUserPage | null>(null)
  const [rbacItems, setRbacItems] = useState<RbacItem[]>([])
  const [stockTarget, setStockTarget] = useState<StockDialogTarget | null>(null)

  const loadDashboard = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true)
      setErrorMessage(null)
      try {
        const [
          productsOutcome,
          ordersOutcome,
          outOfStockOutcome,
          lowStockOutcome,
          usersOutcome,
          rbacOutcome,
        ] = await Promise.allSettled([
          canViewProducts
            ? listAdminProducts({ page: 1, pageSize: 1 }, signal)
            : Promise.resolve(null),
          canViewOrders
            ? listAdminOrders({ page: 1, pageSize: 5 }, signal)
            : Promise.resolve(null),
          canViewInventory
            ? listAdminInventory({ stockStatus: "out_of_stock", page: 1, pageSize: 8 }, signal)
            : Promise.resolve(null),
          canViewInventory
            ? listAdminInventory({ stockStatus: "low_stock", page: 1, pageSize: 8 }, signal)
            : Promise.resolve(null),
          canViewUsers ? listAdminUsers({ page: 1, pageSize: 1 }, signal) : Promise.resolve(null),
          canViewRbac ? listRbacItems(undefined, signal) : Promise.resolve([]),
        ])
        if (signal?.aborted) return

        const outcomes = [
          productsOutcome,
          ordersOutcome,
          outOfStockOutcome,
          lowStockOutcome,
          usersOutcome,
          rbacOutcome,
        ]
        const firstRejected = outcomes.find(
          (outcome): outcome is PromiseRejectedResult => outcome.status === "rejected"
        )
        if (firstRejected) {
          setErrorMessage(resolveErrorMessage(firstRejected.reason))
        }

        if (productsOutcome.status === "fulfilled") {
          setProductsData(productsOutcome.value)
        }
        if (ordersOutcome.status === "fulfilled") {
          setOrdersData(ordersOutcome.value)
        }
        const outOfStockRes =
          outOfStockOutcome.status === "fulfilled" ? outOfStockOutcome.value : null
        const lowStockRes = lowStockOutcome.status === "fulfilled" ? lowStockOutcome.value : null
        if (lowStockRes || outOfStockRes) {
          setInventoryData(lowStockRes ?? outOfStockRes)
          const combinedWatchlist: AdminInventoryItem[] = []
          const seenSkuIds = new Set<string>()
          for (const item of [...(outOfStockRes?.items ?? []), ...(lowStockRes?.items ?? [])]) {
            if (seenSkuIds.has(item.skuId)) continue
            seenSkuIds.add(item.skuId)
            combinedWatchlist.push(item)
          }
          setWatchlistItems(combinedWatchlist.slice(0, 8))
        }
        if (usersOutcome.status === "fulfilled") {
          setUsersData(usersOutcome.value)
        }
        if (rbacOutcome.status === "fulfilled") {
          setRbacItems(rbacOutcome.value)
        }
      } catch (error) {
        if (signal?.aborted) return
        setErrorMessage(resolveErrorMessage(error))
      } finally {
        if (!signal?.aborted) {
          setLoading(false)
        }
      }
    },
    [canViewProducts, canViewOrders, canViewInventory, canViewUsers, canViewRbac]
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadDashboard(controller.signal)
    return () => controller.abort()
  }, [loadDashboard])

  const roleCount = rbacItems.filter((item) => item.type === "ROLE").length
  const permissionCount = rbacItems.filter((item) => item.type === "PERMISSION").length
  const stockSummary = inventoryData?.summary
  const trackedSkus = stockSummary?.totalTrackedSkus ?? stockSummary?.totalTrackedProducts ?? 0

  const stockChartData = useMemo(
    () =>
      stockSummary
        ? [
            { name: "Còn hàng", value: stockSummary.inStockCount, color: STOCK_COLORS.inStock },
            { name: "Sắp hết", value: stockSummary.lowStockCount, color: STOCK_COLORS.lowStock },
            {
              name: "Hết hàng",
              value: stockSummary.outOfStockCount,
              color: STOCK_COLORS.outOfStock,
            },
          ]
        : [],
    [stockSummary]
  )

  const orderStatusBuckets = useMemo(() => {
    const counts = ordersData?.statusCounts
    const completed = sumStatuses(counts, ["DELIVERED"])
    const processing = sumStatuses(counts, ["CONFIRMED", "PREPARING", "SHIPPING"])
    const pending = sumStatuses(counts, [
      "PENDING_STOCK",
      "PENDING_PAYMENT",
      "PENDING_CONFIRMATION",
    ])
    const cancelled = sumStatuses(counts, ["CANCELLED", "DELIVERY_FAILED"])
    return [
      { name: "Hoàn thành", value: completed, color: ORDER_BUCKET_COLORS.completed },
      { name: "Đang xử lý", value: processing, color: ORDER_BUCKET_COLORS.processing },
      { name: "Chờ xử lý", value: pending, color: ORDER_BUCKET_COLORS.pending },
      { name: "Hủy / Lỗi giao", value: cancelled, color: ORDER_BUCKET_COLORS.cancelled },
    ]
  }, [ordersData])

  const orderBucketTotal = useMemo(
    () => orderStatusBuckets.reduce((sum, item) => sum + item.value, 0),
    [orderStatusBuckets]
  )
  const totalOrders = ordersData?.totalItems ?? orderBucketTotal
  const completedOrders = orderStatusBuckets[0]?.value ?? 0
  const pendingOrders = orderStatusBuckets[2]?.value ?? 0

  const activeProducts = productsData?.activeCount ?? 0
  const totalProducts = activeProducts + (productsData?.deletedCount ?? 0)
  const activeUsers = usersData?.activeCount ?? 0
  const totalUsers = activeUsers + (usersData?.disabledCount ?? 0)
  const availableStock = stockSummary?.totalAvailable ?? 0
  const totalStock = stockSummary?.totalQuantity ?? 0

  const primaryKpiCount = [canViewProducts, canViewOrders, canViewInventory, canViewUsers].filter(
    Boolean
  ).length
  const showRbacInKpiRow = canViewRbac && primaryKpiCount < 4

  const openStockModal = (row: AdminInventoryItem) => {
    setStockTarget({
      skuId: row.skuId,
      productId: row.productId,
      name: row.name,
      variantLabel: row.variantLabel,
      brand: row.brand,
      quantity: row.quantity,
      reserved: row.reserved,
      availableQuantity: row.availableQuantity,
    })
  }

  return (
    <div className="space-y-6">
      {/* Page Greeting Header */}
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-[26px]">
            Xin chào, {user?.fullName || "Quản trị viên"}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Tổng quan vận hành hôm nay — theo dõi đơn hàng, danh mục sản phẩm, sức khỏe tồn kho và
            hệ thống tài khoản.
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void loadDashboard()}
            disabled={loading}
          >
            <RefreshCw className={loading ? "size-3.5 animate-spin" : "size-3.5"} />
            Làm mới số liệu
          </Button>
          {canViewOrders && (
            <Button asChild size="sm">
              <Link to="/orders">
                Xem đơn hàng
                <ArrowRight className="size-3.5" />
              </Link>
            </Button>
          )}
        </div>
      </div>

      {errorMessage && (
        <div
          role="alert"
          className="flex items-center gap-2.5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900"
        >
          <AlertTriangle className="size-4 shrink-0 text-red-600" />
          <span>{errorMessage}</span>
        </div>
      )}

      {/* Top KPI Cards */}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {canViewOrders && (
          <KpiCard
            label="Tổng đơn hàng"
            value={formatNumber(totalOrders)}
            sublabel={`Chờ xử lý: ${formatNumber(pendingOrders)}`}
            ratioPercent={safePercent(completedOrders, orderBucketTotal || totalOrders)}
            ratioLabel="Tỉ lệ hoàn thành"
            to="/orders"
            linkLabel="Đơn hàng"
            icon={ClipboardList}
            iconClassName="bg-orange-50 text-orange-600 ring-1 ring-orange-200/70"
            barColor="#ea580c"
            loading={loading && !ordersData}
          />
        )}

        {canViewProducts && (
          <KpiCard
            label="Sản phẩm đang bán"
            value={formatNumber(activeProducts)}
            sublabel={`Đã ẩn: ${formatNumber(productsData?.deletedCount ?? 0)}`}
            ratioPercent={safePercent(activeProducts, totalProducts)}
            ratioLabel="Đang hiển thị"
            to="/products"
            linkLabel="Danh mục"
            icon={Package}
            iconClassName="bg-teal-50 text-teal-700 ring-1 ring-teal-200/70"
            barColor="#0d9488"
            loading={loading && !productsData}
          />
        )}

        {canViewInventory && (
          <KpiCard
            label="Tồn kho khả dụng"
            value={formatNumber(availableStock)}
            sublabel={`Tổng ${formatNumber(totalStock)} · Giữ chỗ ${formatNumber(stockSummary?.totalReserved ?? 0)}`}
            ratioPercent={safePercent(availableStock, totalStock)}
            ratioLabel="Tỉ lệ khả dụng"
            to="/inventory"
            linkLabel="Kho vận"
            icon={Boxes}
            iconClassName="bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200/70"
            barColor="#10b981"
            loading={loading && !inventoryData}
          />
        )}

        {canViewUsers && (
          <KpiCard
            label="Tài khoản hoạt động"
            value={formatNumber(activeUsers)}
            sublabel={`Đã khóa: ${formatNumber(usersData?.disabledCount ?? 0)}`}
            ratioPercent={safePercent(activeUsers, totalUsers)}
            ratioLabel="Tỉ lệ hoạt động"
            to="/users"
            linkLabel="Người dùng"
            icon={Users}
            iconClassName="bg-blue-50 text-blue-600 ring-1 ring-blue-200/70"
            barColor="#2563eb"
            loading={loading && !usersData}
          />
        )}

        {showRbacInKpiRow && (
          <KpiCard
            label="Vai trò RBAC"
            value={formatNumber(roleCount)}
            sublabel={`${formatNumber(permissionCount)} quyền thực thi`}
            ratioPercent={safePercent(roleCount, roleCount + permissionCount)}
            ratioLabel="Tỉ trọng vai trò"
            to="/rbac"
            linkLabel="Phân quyền"
            icon={ShieldCheck}
            iconClassName="bg-violet-50 text-violet-700 ring-1 ring-violet-200/70"
            barColor="#7c3aed"
            loading={loading && rbacItems.length === 0}
          />
        )}
      </div>

      {/* Primary Analytics Bento Row */}
      {(canViewProducts || canViewOrders) && (
        <div className="grid gap-5 lg:grid-cols-12">
          {canViewProducts && (
            <Card className={canViewOrders ? "lg:col-span-7" : "lg:col-span-12"}>
              <CardHeader className="flex flex-row items-start justify-between gap-3">
                <div>
                  <CardTitle>Phân bổ sản phẩm theo danh mục</CardTitle>
                  <CardDescription>
                    Số lượng sản phẩm đang hiển thị trên từng ngành hàng
                  </CardDescription>
                </div>
                <Badge variant="secondary" className="font-mono tabular-nums">
                  {formatNumber(activeProducts)} sản phẩm
                </Badge>
              </CardHeader>
              <CardContent>
                {productsData && productsData.categories.length > 0 ? (
                  <div className="h-[270px] w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart
                        data={productsData.categories}
                        margin={{ top: 10, right: 12, left: -16, bottom: 4 }}
                        barCategoryGap="28%"
                      >
                        <CartesianGrid
                          strokeDasharray="3 3"
                          vertical={false}
                          stroke="#eaecf0"
                        />
                        <XAxis
                          dataKey="categoryName"
                          axisLine={false}
                          tickLine={false}
                          tick={{ fontSize: 12, fill: "#667085" }}
                          dy={6}
                        />
                        <YAxis
                          allowDecimals={false}
                          axisLine={false}
                          tickLine={false}
                          tick={{ fontSize: 12, fill: "#667085" }}
                        />
                        <Tooltip
                          cursor={{ fill: "#f4f6f9" }}
                          contentStyle={{
                            borderRadius: "12px",
                            border: "1px solid #e6e8ec",
                            boxShadow: "0 8px 20px -6px rgba(16,24,40,0.12)",
                            fontSize: "12px",
                          }}
                        />
                        <Bar
                          dataKey="activeProductCount"
                          name="Sản phẩm đang bán"
                          fill="#0d9488"
                          radius={[8, 8, 2, 2]}
                          maxBarSize={42}
                        />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                ) : (
                  <p className="py-16 text-center text-sm text-muted-foreground">
                    Chưa có số liệu phân bổ danh mục.
                  </p>
                )}
              </CardContent>
            </Card>
          )}

          {canViewOrders && (
            <Card className={canViewProducts ? "lg:col-span-5" : "lg:col-span-12"}>
              <CardHeader className="flex flex-row items-start justify-between gap-3">
                <div>
                  <CardTitle>Phân bổ đơn hàng</CardTitle>
                  <CardDescription>Tỉ trọng trạng thái xử lý đơn toàn hệ thống</CardDescription>
                </div>
                <Link
                  to="/orders"
                  className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                  Chi tiết →
                </Link>
              </CardHeader>
              <CardContent>
                {ordersData && orderBucketTotal > 0 ? (
                  <div className="grid items-center gap-5 sm:grid-cols-[172px_1fr]">
                    <div className="relative mx-auto h-44 w-44">
                      <ResponsiveContainer width="100%" height="100%">
                        <PieChart>
                          <Pie
                            data={orderStatusBuckets}
                            dataKey="value"
                            nameKey="name"
                            innerRadius={52}
                            outerRadius={76}
                            paddingAngle={3}
                            strokeWidth={0}
                          >
                            {orderStatusBuckets.map((entry) => (
                              <Cell key={entry.name} fill={entry.color} />
                            ))}
                          </Pie>
                          <Tooltip
                            contentStyle={{
                              borderRadius: "10px",
                              border: "1px solid #e6e8ec",
                              fontSize: "12px",
                            }}
                          />
                        </PieChart>
                      </ResponsiveContainer>
                      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                        <span className="font-mono text-2xl font-bold text-foreground tabular-nums">
                          {formatNumber(totalOrders)}
                        </span>
                        <span className="text-[11px] font-medium text-muted-foreground">
                          Đơn hàng
                        </span>
                      </div>
                    </div>

                    <div className="space-y-3">
                      {orderStatusBuckets.map((bucket) => {
                        const pct = safePercent(bucket.value, orderBucketTotal)
                        return (
                          <div
                            key={bucket.name}
                            className="flex items-center justify-between gap-2 text-xs"
                          >
                            <span className="flex items-center gap-2 text-foreground">
                              <span
                                className="size-2.5 shrink-0 rounded-full"
                                style={{ backgroundColor: bucket.color }}
                              />
                              <span className="font-medium">{bucket.name}</span>
                            </span>
                            <div className="flex items-center gap-2 font-mono tabular-nums">
                              <span className="text-muted-foreground">
                                {formatNumber(bucket.value)}
                              </span>
                              <span className="w-10 text-right font-semibold text-foreground">
                                {pct}%
                              </span>
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                ) : (
                  <p className="py-16 text-center text-sm text-muted-foreground">
                    Chưa có dữ liệu đơn hàng để thống kê.
                  </p>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* Stock Health + RBAC Governance Row */}
      {(canViewInventory || (canViewRbac && !showRbacInKpiRow)) && (
        <div className="grid gap-5 lg:grid-cols-12">
          {canViewInventory && (
            <Card className={canViewRbac && !showRbacInKpiRow ? "lg:col-span-8" : "lg:col-span-12"}>
              <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
                <div>
                  <CardTitle>Tỷ trọng sức khỏe tồn kho</CardTitle>
                  <CardDescription>
                    Theo dõi {formatNumber(trackedSkus)} mã SKU (
                    {formatNumber(stockSummary?.totalTrackedProducts ?? 0)} sản phẩm)
                  </CardDescription>
                </div>
                <Link
                  to="/inventory"
                  className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                  Kiểm kê kho →
                </Link>
              </CardHeader>
              <CardContent>
                {stockSummary && trackedSkus > 0 ? (
                  <div className="space-y-5">
                    <div className="flex h-3 w-full overflow-hidden rounded-full bg-muted p-0.5">
                      {stockChartData.map((bucket) => {
                        const pct = safePercent(bucket.value, trackedSkus)
                        if (pct <= 0) return null
                        return (
                          <div
                            key={bucket.name}
                            className="h-full first:rounded-l-full last:rounded-r-full transition-all duration-300"
                            style={{ width: `${pct}%`, backgroundColor: bucket.color }}
                            title={`${bucket.name}: ${bucket.value} (${pct}%)`}
                          />
                        )
                      })}
                    </div>

                    <div className="grid gap-3 sm:grid-cols-3">
                      {stockChartData.map((bucket) => {
                        const pct = safePercent(bucket.value, trackedSkus)
                        return (
                          <div
                            key={bucket.name}
                            className="rounded-xl border border-border/80 bg-background/60 p-3.5"
                          >
                            <div className="flex items-center justify-between text-xs">
                              <span className="flex items-center gap-2 font-medium text-muted-foreground">
                                <span
                                  className="size-2 rounded-full"
                                  style={{ backgroundColor: bucket.color }}
                                />
                                {bucket.name}
                              </span>
                              <span className="font-mono text-[11px] font-semibold text-foreground tabular-nums">
                                {pct}%
                              </span>
                            </div>
                            <div className="mt-2 font-mono text-xl font-semibold text-foreground tabular-nums">
                              {formatNumber(bucket.value)}
                              <span className="ml-1 font-sans text-xs font-normal text-muted-foreground">
                                SKU
                              </span>
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                ) : (
                  <p className="py-10 text-center text-sm text-muted-foreground">
                    Chưa có dữ liệu tồn kho.
                  </p>
                )}
              </CardContent>
            </Card>
          )}

          {canViewRbac && !showRbacInKpiRow && (
            <Card className={canViewInventory ? "lg:col-span-4" : "lg:col-span-12"}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle>Phân quyền RBAC</CardTitle>
                  <div className="flex size-9 items-center justify-center rounded-xl bg-violet-50 text-violet-700 ring-1 ring-violet-200/70">
                    <ShieldCheck className="size-4" />
                  </div>
                </div>
                <CardDescription>Cấu trúc vai trò và quyền thực thi hệ thống</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-xl border border-border/80 bg-background/60 p-3.5">
                    <div className="text-xs text-muted-foreground">Vai trò</div>
                    <div className="mt-1 font-mono text-2xl font-semibold text-foreground tabular-nums">
                      {formatNumber(roleCount)}
                    </div>
                  </div>
                  <div className="rounded-xl border border-border/80 bg-background/60 p-3.5">
                    <div className="text-xs text-muted-foreground">Quyền hạn</div>
                    <div className="mt-1 font-mono text-2xl font-semibold text-foreground tabular-nums">
                      {formatNumber(permissionCount)}
                    </div>
                  </div>
                </div>
                <Button asChild variant="outline" size="sm" className="w-full justify-between">
                  <Link to="/rbac">
                    Quản trị cây phân quyền
                    <ArrowRight className="size-3.5" />
                  </Link>
                </Button>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* Recent Orders Panel */}
      {canViewOrders && (
        <Card>
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3">
            <div>
              <CardTitle>Đơn hàng gần đây</CardTitle>
              <CardDescription>
                5 đơn hàng mới nhất được ghi nhận trên toàn hệ thống
              </CardDescription>
            </div>
            <Button asChild variant="outline" size="sm">
              <Link to="/orders">
                Xem tất cả đơn
                <ArrowRight className="size-3.5" />
              </Link>
            </Button>
          </CardHeader>
          <CardContent className="p-0">
            {ordersData && ordersData.items.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Mã đơn</TableHead>
                    <TableHead>Người nhận</TableHead>
                    <TableHead>Thanh toán</TableHead>
                    <TableHead>Trạng thái</TableHead>
                    <TableHead className="text-right">Tổng tiền</TableHead>
                    <TableHead className="text-right">Thời gian</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {ordersData.items.map((order) => (
                    <TableRow key={order.orderId}>
                      <TableCell className="font-mono text-xs font-semibold text-foreground">
                        {order.orderCode}
                      </TableCell>
                      <TableCell>
                        <div className="font-medium text-foreground">
                          {order.shippingAddress.recipientName}
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {order.shippingAddress.provinceName}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="font-mono text-[10px]">
                          {order.paymentMethod}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={orderStatusBadgeVariant(order.status)}>
                          {ORDER_STATUS_LABELS[order.status]}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right font-mono font-semibold text-foreground tabular-nums">
                        {formatCurrencyVnd(order.totalAmount)}
                      </TableCell>
                      <TableCell className="text-right text-xs text-muted-foreground">
                        {formatDateTime(order.createdAt)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="px-6 py-10 text-center text-sm text-muted-foreground">
                Chưa có đơn hàng nào được tạo.
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Low-Stock SKU Watchlist */}
      {canViewInventory && (
        <Card>
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3">
            <div>
              <CardTitle className="flex items-center gap-2">
                <AlertTriangle className="size-4 text-amber-600" />
                Danh sách cảnh báo SKU cần nhập bổ sung
              </CardTitle>
              <CardDescription>
                Các SKU đã hết (0) hoặc sắp hết (từ 1 đến 5 đơn vị khả dụng) cần ưu tiên xử lý
              </CardDescription>
            </div>
            <Button asChild variant="outline" size="sm">
              <Link to="/inventory">
                Xem toàn bộ kho
                <ArrowRight className="size-3.5" />
              </Link>
            </Button>
          </CardHeader>
          <CardContent className="p-0">
            {watchlistItems.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Sản phẩm & Phân loại</TableHead>
                    <TableHead>Danh mục</TableHead>
                    <TableHead className="text-right">Tổng tồn</TableHead>
                    <TableHead className="text-right">Giữ chỗ</TableHead>
                    <TableHead className="text-right">Khả dụng</TableHead>
                    {canUpdateInventory && <TableHead className="text-right">Thao tác</TableHead>}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {watchlistItems.map((row) => (
                    <TableRow key={row.skuId}>
                      <TableCell>
                        <div className="font-medium text-foreground">{row.name}</div>
                        <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-foreground/80">
                          {row.variantColorHex && (
                            <span
                              className="size-2.5 shrink-0 rounded-full border border-black/15"
                              style={{ backgroundColor: row.variantColorHex }}
                            />
                          )}
                          <span>{row.variantLabel}</span>
                        </div>
                        <div className="font-mono text-[11px] text-muted-foreground">
                          {row.skuId} · {row.productId}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{row.categoryName || row.categoryId}</Badge>
                      </TableCell>
                      <TableCell className="text-right font-mono tabular-nums">
                        {formatNumber(row.quantity)}
                      </TableCell>
                      <TableCell className="text-right font-mono text-amber-700 tabular-nums">
                        {formatNumber(row.reserved)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Badge
                          variant={row.availableQuantity <= 0 ? "destructive" : "warning"}
                          className="font-mono tabular-nums"
                        >
                          {row.availableQuantity <= 0
                            ? "Hết hàng (0)"
                            : `${formatNumber(row.availableQuantity)} đơn vị`}
                        </Badge>
                      </TableCell>
                      {canUpdateInventory && (
                        <TableCell className="text-right">
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            aria-label={`Điều chỉnh tồn ${row.name} ${row.variantLabel}`.trim()}
                            onClick={() => openStockModal(row)}
                          >
                            <SlidersHorizontal className="size-3.5" />
                            Điều chỉnh tồn
                          </Button>
                        </TableCell>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="px-6 py-10 text-center text-sm text-muted-foreground">
                Không có SKU nào đang hết hàng hoặc ở ngưỡng cảnh báo sắp hết.
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <StockDialog
        open={Boolean(stockTarget)}
        onOpenChange={(next) => {
          if (!next) setStockTarget(null)
        }}
        item={stockTarget}
        onSaved={() => void loadDashboard()}
      />
    </div>
  )
}
