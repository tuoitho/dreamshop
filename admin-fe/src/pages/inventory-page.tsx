import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"
import { ChevronLeft, ChevronRight, RefreshCw, Search, SlidersHorizontal } from "lucide-react"
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useAuth } from "@/hooks/use-auth"
import { listAdminInventory } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { CATEGORY_PRESETS } from "@/lib/categories"
import { formatCurrencyVnd, formatDateTime, formatNumber } from "@/lib/format"
import type { AdminInventoryItem, AdminInventoryPage } from "@/lib/types"
import { cn } from "@/lib/utils"
import { StockDialog, type StockDialogTarget } from "./stock-dialog"

type StockFilter = "all" | "in_stock" | "low_stock" | "out_of_stock"

export function InventoryPage() {
  const { can } = useAuth()
  const canUpdate = can("inventory.update")

  const [stockFilter, setStockFilter] = useState<StockFilter>("all")
  const [categoryId, setCategoryId] = useState("")
  const [includeDeleted, setIncludeDeleted] = useState(true)
  const [searchDraft, setSearchDraft] = useState("")
  const [appliedQuery, setAppliedQuery] = useState("")
  const [page, setPage] = useState(1)
  const [data, setData] = useState<AdminInventoryPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [stockTarget, setStockTarget] = useState<StockDialogTarget | null>(null)

  const loadInventory = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true)
      setErrorMessage(null)
      try {
        const res = await listAdminInventory(
          {
            stockStatus: stockFilter,
            categoryId: categoryId || undefined,
            includeDeleted,
            q: appliedQuery || undefined,
            page,
            pageSize: 15,
          },
          signal
        )
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
    [stockFilter, categoryId, includeDeleted, appliedQuery, page]
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadInventory(controller.signal)
    return () => controller.abort()
  }, [loadInventory])

  const handleSearchSubmit = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setAppliedQuery(searchDraft.trim())
  }

  const columns = useMemo<ColumnDef<AdminInventoryItem>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Sản phẩm & Phân loại SKU",
        cell: ({ row }) => {
          const item = row.original
          return (
            <div className="flex min-w-[250px] items-center gap-3">
              {item.imageUrl ? (
                <div className="flex size-11 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-background p-0.5">
                  <img
                    src={item.imageUrl}
                    alt={item.name}
                    className="max-h-full max-w-full object-contain"
                  />
                </div>
              ) : (
                <div className="flex size-11 shrink-0 items-center justify-center rounded-md border border-border bg-muted text-base">
                  {item.emoji || "📦"}
                </div>
              )}
              <div className="min-w-0 space-y-0.5">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="truncate font-medium text-foreground">{item.name}</span>
                  {item.deleted && <Badge variant="secondary">Đã ẩn</Badge>}
                </div>
                <div className="text-xs text-foreground/85">
                  <span>{item.variantLabel}</span>
                </div>
                <div className="font-mono text-[11px] text-muted-foreground">
                  {item.skuId} · {item.brand} · {item.productId}
                </div>
              </div>
            </div>
          )
        },
      },
      {
        accessorKey: "categoryName",
        header: "Danh mục",
        cell: ({ row }) => (
          <Badge variant="outline">
            {row.original.categoryName || row.original.categoryId}
          </Badge>
        ),
      },
      {
        accessorKey: "price",
        header: () => <div className="text-right">Giá bán</div>,
        cell: ({ row }) => (
          <div className="text-right font-mono tabular-nums">
            <div className="font-semibold text-foreground">
              {formatCurrencyVnd(row.original.price)}
            </div>
            {(row.original.originalPrice ?? 0) > row.original.price && (
              <div className="text-[11px] text-muted-foreground line-through">
                {formatCurrencyVnd(row.original.originalPrice ?? 0)}
              </div>
            )}
          </div>
        ),
      },
      {
        accessorKey: "quantity",
        header: () => <div className="text-right">Tổng tồn</div>,
        cell: ({ row }) => (
          <div className="text-right font-mono font-medium text-foreground tabular-nums">
            {formatNumber(row.original.quantity)}
          </div>
        ),
      },
      {
        accessorKey: "reserved",
        header: () => <div className="text-right">Đang giữ chỗ</div>,
        cell: ({ row }) => (
          <div className="text-right font-mono text-amber-800 tabular-nums">
            {formatNumber(row.original.reserved)}
          </div>
        ),
      },
      {
        accessorKey: "availableQuantity",
        header: () => <div className="text-right">Khả dụng</div>,
        cell: ({ row }) => (
          <div className="text-right font-mono font-semibold text-foreground tabular-nums">
            {formatNumber(row.original.availableQuantity)}
          </div>
        ),
      },
      {
        accessorKey: "stockStatus",
        header: "Trạng thái kho",
        cell: ({ row }) => {
          const status = row.original.stockStatus
          if (status === "OUT_OF_STOCK") {
            return <Badge variant="destructive">Hết hàng</Badge>
          }
          if (status === "LOW_STOCK") {
            return <Badge variant="warning">Sắp hết</Badge>
          }
          return <Badge variant="success">Còn hàng</Badge>
        },
      },
      {
        accessorKey: "updatedAt",
        header: "Cập nhật",
        cell: ({ row }) => (
          <span className="font-mono text-xs text-muted-foreground">
            {formatDateTime(row.original.updatedAt)}
          </span>
        ),
      },
      ...(canUpdate
        ? ([
            {
              id: "actions",
              header: () => <div className="text-right">Thao tác</div>,
              cell: ({ row }) => {
                const item = row.original
                return (
                  <div className="text-right">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      aria-label={`Điều chỉnh tồn ${item.name} ${item.variantLabel}`.trim()}
                      onClick={() =>
                        setStockTarget({
                          skuId: item.skuId,
                          productId: item.productId,
                          name: item.name,
                          variantLabel: item.variantLabel,
                          brand: item.brand,
                          quantity: item.quantity,
                          reserved: item.reserved,
                          availableQuantity: item.availableQuantity,
                        })
                      }
                    >
                      <SlidersHorizontal className="size-3.5" />
                      Điều chỉnh
                    </Button>
                  </div>
                )
              },
            },
          ] as ColumnDef<AdminInventoryItem>[])
        : []),
    ],
    [canUpdate]
  )

  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const summary = data?.summary

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Kiểm kê & Điều phối tồn kho SKU
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Theo dõi tồn vật lý, lượng giữ chỗ đơn hàng và cập nhật nguyên tử theo từng mã SKU phân
            loại.
          </p>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => void loadInventory()}
          disabled={loading}
        >
          <RefreshCw className={loading ? "size-3.5 animate-spin" : "size-3.5"} />
          Làm mới
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

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-xl border border-border bg-card px-5 py-4">
          <div className="text-xs font-medium text-muted-foreground">Tổng lượng tồn vật lý</div>
          <div className="mt-1 font-mono text-2xl font-semibold text-foreground tabular-nums">
            {formatNumber(summary?.totalQuantity ?? 0)}
          </div>
        </div>
        <div className="rounded-xl border border-border bg-card px-5 py-4">
          <div className="text-xs font-medium text-muted-foreground">Đang giữ chỗ</div>
          <div className="mt-1 font-mono text-2xl font-semibold text-amber-800 tabular-nums">
            {formatNumber(summary?.totalReserved ?? 0)}
          </div>
        </div>
        <div className="rounded-xl border border-border bg-card px-5 py-4">
          <div className="text-xs font-medium text-muted-foreground">Khả dụng mở bán</div>
          <div className="mt-1 font-mono text-2xl font-semibold text-emerald-700 tabular-nums">
            {formatNumber(summary?.totalAvailable ?? 0)}
          </div>
        </div>
        <div className="rounded-xl border border-border bg-card px-5 py-4">
          <div className="text-xs font-medium text-muted-foreground">Cảnh báo SKU</div>
          <div className="mt-1 font-mono text-2xl font-semibold text-red-700 tabular-nums">
            {formatNumber((summary?.lowStockCount ?? 0) + (summary?.outOfStockCount ?? 0))}
          </div>
        </div>
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
          <div className="inline-flex flex-wrap items-center gap-1 rounded-lg border border-border bg-muted p-1">
            {(
              [
                {
                  key: "all",
                  label: "Tất cả",
                  count: summary?.totalTrackedSkus ?? summary?.totalTrackedProducts ?? 0,
                },
                { key: "in_stock", label: "Còn hàng", count: summary?.inStockCount ?? 0 },
                { key: "low_stock", label: "Sắp hết", count: summary?.lowStockCount ?? 0 },
                { key: "out_of_stock", label: "Hết hàng", count: summary?.outOfStockCount ?? 0 },
              ] as const
            ).map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => {
                  setStockFilter(tab.key)
                  setPage(1)
                }}
                className={cn(
                  "inline-flex cursor-pointer items-center gap-2 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  stockFilter === tab.key
                    ? "border border-border bg-card font-semibold text-foreground shadow-2xs"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <span>{tab.label}</span>
                <span className="rounded-full bg-background px-1.5 py-0.5 font-mono text-[11px] tabular-nums">
                  {formatNumber(tab.count)}
                </span>
              </button>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <label className="flex cursor-pointer items-center gap-2 text-xs text-muted-foreground">
              <Switch
                checked={includeDeleted}
                onCheckedChange={(checked) => {
                  setIncludeDeleted(checked)
                  setPage(1)
                }}
              />
              <span>Gồm mã đã ẩn</span>
            </label>

            <form onSubmit={handleSearchSubmit} className="flex flex-wrap items-center gap-2">
              <select
                aria-label="Lọc danh mục kho"
                value={categoryId}
                onChange={(e) => {
                  setCategoryId(e.target.value)
                  setPage(1)
                }}
                className="h-9 rounded-lg border border-input bg-card px-3 text-xs font-medium text-foreground outline-none focus-visible:border-ring"
              >
                <option value="">Tất cả danh mục</option>
                {CATEGORY_PRESETS.map((cat) => (
                  <option key={cat.id} value={cat.id}>
                    {cat.name}
                  </option>
                ))}
              </select>

              <div className="relative">
                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={searchDraft}
                  onChange={(e) => setSearchDraft(e.target.value)}
                  placeholder="Tìm sản phẩm, phân loại hoặc SKU…"
                  className="w-48 pl-8 text-xs sm:w-56"
                />
              </div>

              <Button type="submit" variant="outline" size="sm">
                Lọc
              </Button>
            </form>
          </div>
        </div>

        <CardContent className="p-0">
          {loading && !data ? (
            <div className="space-y-3 p-6">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : table.getRowModel().rows.length > 0 ? (
            <Table>
              <TableHeader>
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <TableHead key={header.id}>
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                      </TableHead>
                    ))}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody>
                {table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id}>
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <div className="px-6 py-12 text-center text-sm text-muted-foreground">
              Không có bản ghi tồn kho SKU phù hợp.
            </div>
          )}

          {data && data.totalPages > 1 && (
            <div className="flex items-center justify-between border-t border-border px-5 py-3 text-xs text-muted-foreground">
              <span>
                Trang <span className="font-mono font-semibold text-foreground">{data.page}</span> /{" "}
                <span className="font-mono">{data.totalPages}</span> ({formatNumber(data.totalItems)}{" "}
                SKU)
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

      <StockDialog
        open={Boolean(stockTarget)}
        onOpenChange={(next) => {
          if (!next) setStockTarget(null)
        }}
        item={stockTarget}
        onSaved={() => void loadInventory()}
      />
    </div>
  )
}
