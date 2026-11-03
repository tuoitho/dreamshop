import {
  AlertTriangle,
  ArrowRight,
  Boxes,
  Package,
  RefreshCw,
  ShieldCheck,
  SlidersHorizontal,
  Users,
} from "lucide-react"
import { useCallback, useEffect, useState } from "react"
import { Link } from "react-router-dom"
import {
  Bar,
  BarChart,
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
  listAdminProducts,
  listAdminUsers,
  listRbacItems,
  toClientError,
} from "@/lib/api"
import { formatNumber } from "@/lib/format"
import type {
  AdminInventoryItem,
  AdminInventoryPage,
  AdminProductsPage,
  AdminUserPage,
  RbacItem,
} from "@/lib/types"
import { StockDialog, type StockDialogTarget } from "./stock-dialog"

const STOCK_PIE_COLORS = {
  inStock: "#0f766e",
  lowStock: "#d97706",
  outOfStock: "#c8322b",
}

export function OverviewPage() {
  const { can } = useAuth()
  const canViewProducts = can("product.view")
  const canViewInventory = can("inventory.view")
  const canUpdateInventory = can("inventory.update")
  const canViewUsers = can("user.view")
  const canViewRbac = can("rbac.view")

  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [productsData, setProductsData] = useState<AdminProductsPage | null>(null)
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
        const [productsOutcome, outOfStockOutcome, lowStockOutcome, usersOutcome, rbacOutcome] =
          await Promise.allSettled([
            canViewProducts
              ? listAdminProducts({ page: 1, pageSize: 1 }, signal)
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
          outOfStockOutcome,
          lowStockOutcome,
          usersOutcome,
          rbacOutcome,
        ]
        const firstRejected = outcomes.find(
          (outcome): outcome is PromiseRejectedResult => outcome.status === "rejected"
        )
        if (firstRejected) {
          setErrorMessage(toClientError(firstRejected.reason).message)
        }

        if (productsOutcome.status === "fulfilled") {
          setProductsData(productsOutcome.value)
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
        setErrorMessage(toClientError(error).message)
      } finally {
        if (!signal?.aborted) {
          setLoading(false)
        }
      }
    },
    [canViewProducts, canViewInventory, canViewUsers, canViewRbac]
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

  const stockChartData = stockSummary
    ? [
        { name: "Còn hàng", value: stockSummary.inStockCount, color: STOCK_PIE_COLORS.inStock },
        { name: "Sắp hết", value: stockSummary.lowStockCount, color: STOCK_PIE_COLORS.lowStock },
        { name: "Hết hàng", value: stockSummary.outOfStockCount, color: STOCK_PIE_COLORS.outOfStock },
      ]
    : []

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
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Tổng quan vận hành
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Giám sát trực tiếp sức khỏe danh mục, tồn kho SKU khả dụng, tài khoản và phân quyền hệ
            thống.
          </p>
        </div>
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
      </div>

      {errorMessage && (
        <div
          role="alert"
          className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900"
        >
          {errorMessage}
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {canViewProducts && (
          <Card>
            <CardContent className="flex flex-col justify-between gap-4 p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-xs font-medium text-muted-foreground">Sản phẩm đang bán</div>
                  {loading && !productsData ? (
                    <Skeleton className="mt-2 h-8 w-20" />
                  ) : (
                    <div className="mt-1.5 font-mono text-3xl font-semibold tracking-tight text-foreground tabular-nums">
                      {formatNumber(productsData?.activeCount ?? 0)}
                    </div>
                  )}
                </div>
                <div className="flex size-9 items-center justify-center rounded-lg bg-muted text-primary">
                  <Package className="size-4" />
                </div>
              </div>
              <div className="flex items-center justify-between border-t border-border pt-3 text-xs text-muted-foreground">
                <span>Đã ẩn: {formatNumber(productsData?.deletedCount ?? 0)}</span>
                <Link to="/products" className="font-medium text-primary hover:underline">
                  Danh mục →
                </Link>
              </div>
            </CardContent>
          </Card>
        )}

        {canViewInventory && (
          <Card>
            <CardContent className="flex flex-col justify-between gap-4 p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-xs font-medium text-muted-foreground">Tồn kho khả dụng</div>
                  {loading && !inventoryData ? (
                    <Skeleton className="mt-2 h-8 w-24" />
                  ) : (
                    <div className="mt-1.5 font-mono text-3xl font-semibold tracking-tight text-foreground tabular-nums">
                      {formatNumber(stockSummary?.totalAvailable ?? 0)}
                    </div>
                  )}
                </div>
                <div className="flex size-9 items-center justify-center rounded-lg bg-muted text-primary">
                  <Boxes className="size-4" />
                </div>
              </div>
              <div className="flex items-center justify-between border-t border-border pt-3 text-xs text-muted-foreground">
                <span>
                  Tổng {formatNumber(stockSummary?.totalQuantity ?? 0)} · Giữ chỗ{" "}
                  {formatNumber(stockSummary?.totalReserved ?? 0)}
                </span>
                <Link to="/inventory" className="font-medium text-primary hover:underline">
                  Kho vận →
                </Link>
              </div>
            </CardContent>
          </Card>
        )}

        {canViewUsers && (
          <Card>
            <CardContent className="flex flex-col justify-between gap-4 p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-xs font-medium text-muted-foreground">
                    Tài khoản hoạt động
                  </div>
                  {loading && !usersData ? (
                    <Skeleton className="mt-2 h-8 w-20" />
                  ) : (
                    <div className="mt-1.5 font-mono text-3xl font-semibold tracking-tight text-foreground tabular-nums">
                      {formatNumber(usersData?.activeCount ?? 0)}
                    </div>
                  )}
                </div>
                <div className="flex size-9 items-center justify-center rounded-lg bg-muted text-primary">
                  <Users className="size-4" />
                </div>
              </div>
              <div className="flex items-center justify-between border-t border-border pt-3 text-xs text-muted-foreground">
                <span>Đã khóa: {formatNumber(usersData?.disabledCount ?? 0)}</span>
                <Link to="/users" className="font-medium text-primary hover:underline">
                  Người dùng →
                </Link>
              </div>
            </CardContent>
          </Card>
        )}

        {canViewRbac && (
          <Card>
            <CardContent className="flex flex-col justify-between gap-4 p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-xs font-medium text-muted-foreground">Vai trò RBAC</div>
                  {loading && rbacItems.length === 0 ? (
                    <Skeleton className="mt-2 h-8 w-16" />
                  ) : (
                    <div className="mt-1.5 font-mono text-3xl font-semibold tracking-tight text-foreground tabular-nums">
                      {formatNumber(roleCount)}
                    </div>
                  )}
                </div>
                <div className="flex size-9 items-center justify-center rounded-lg bg-muted text-primary">
                  <ShieldCheck className="size-4" />
                </div>
              </div>
              <div className="flex items-center justify-between border-t border-border pt-3 text-xs text-muted-foreground">
                <span>{formatNumber(permissionCount)} quyền thực thi</span>
                <Link to="/rbac" className="font-medium text-primary hover:underline">
                  Phân quyền →
                </Link>
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {(canViewProducts || canViewInventory) && (
        <div className="grid gap-4 lg:grid-cols-[1.35fr_1fr]">
          {canViewProducts && (
            <Card>
              <CardHeader>
                <CardTitle>Phân bổ sản phẩm theo danh mục</CardTitle>
                <CardDescription>Số lượng mã hàng đang hiển thị trên từng ngành hàng</CardDescription>
              </CardHeader>
              <CardContent>
                {productsData && productsData.categories.length > 0 ? (
                  <div className="h-64 w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart
                        data={productsData.categories}
                        margin={{ top: 8, right: 12, left: -12, bottom: 4 }}
                      >
                        <XAxis dataKey="categoryName" tick={{ fontSize: 12 }} />
                        <YAxis allowDecimals={false} tick={{ fontSize: 12 }} />
                        <Tooltip />
                        <Bar
                          dataKey="activeProductCount"
                          name="Sản phẩm đang bán"
                          fill="#0f766e"
                          radius={[6, 6, 0, 0]}
                        />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                ) : (
                  <p className="py-10 text-center text-sm text-muted-foreground">
                    Chưa có số liệu phân bổ danh mục.
                  </p>
                )}
              </CardContent>
            </Card>
          )}

          {canViewInventory && (
            <Card>
              <CardHeader>
                <CardTitle>Tỷ trọng sức khỏe tồn kho</CardTitle>
                <CardDescription>
                  Theo dõi {formatNumber(trackedSkus)} mã SKU (
                  {formatNumber(stockSummary?.totalTrackedProducts ?? 0)} sản phẩm)
                </CardDescription>
              </CardHeader>
              <CardContent>
                {stockSummary && trackedSkus > 0 ? (
                  <div className="grid items-center gap-4 sm:grid-cols-[170px_1fr]">
                    <div className="mx-auto h-40 w-40">
                      <ResponsiveContainer width="100%" height="100%">
                        <PieChart>
                          <Pie
                            data={stockChartData}
                            dataKey="value"
                            nameKey="name"
                            innerRadius={44}
                            outerRadius={68}
                            paddingAngle={2}
                          >
                            {stockChartData.map((entry) => (
                              <Cell key={entry.name} fill={entry.color} />
                            ))}
                          </Pie>
                          <Tooltip />
                        </PieChart>
                      </ResponsiveContainer>
                    </div>
                    <div className="space-y-3">
                      {stockChartData.map((bucket) => {
                        const pct =
                          trackedSkus > 0 ? Math.round((bucket.value / trackedSkus) * 100) : 0
                        return (
                          <div key={bucket.name} className="space-y-1">
                            <div className="flex items-center justify-between text-xs">
                              <span className="flex items-center gap-2 font-medium text-foreground">
                                <span
                                  className="size-2.5 rounded-full"
                                  style={{ backgroundColor: bucket.color }}
                                />
                                {bucket.name}
                              </span>
                              <span className="font-mono font-semibold text-foreground tabular-nums">
                                {formatNumber(bucket.value)} ({pct}%)
                              </span>
                            </div>
                            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                              <div
                                className="h-full rounded-full"
                                style={{
                                  width: `${pct}%`,
                                  backgroundColor: bucket.color,
                                }}
                              />
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
        </div>
      )}

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
                        <div className="flex flex-wrap items-center gap-1.5 text-xs text-foreground/85">
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
                      <TableCell className="text-right font-mono text-amber-800 tabular-nums">
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
