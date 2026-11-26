import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"
import {
  AlertTriangle,
  Archive,
  ChevronLeft,
  ChevronRight,
  Eye,
  Film,
  Pencil,
  Plus,
  RotateCcw,
  Search,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react"
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
import { listAdminProducts, toggleAdminProductDeleted } from "@/lib/api"
import { resolveErrorMessage } from "@/lib/error-messages"
import { CATEGORY_PRESETS } from "@/lib/categories"
import { formatCurrencyVnd, formatNumber } from "@/lib/format"
import type { AdminProductSummary, AdminProductsPage } from "@/lib/types"
import { cn } from "@/lib/utils"
import { ProductDetailSheet } from "./product-detail-sheet"
import { ProductDrawer } from "./product-drawer"

type ProductStatusFilter = "all" | "active" | "deleted"

export function ProductsPage() {
  const { can } = useAuth()
  const canCreate = can("product.create")
  const canUpdate = can("product.update")
  const canDelete = can("product.delete")
  const canUpdateInventory = can("inventory.update")

  const [statusFilter, setStatusFilter] = useState<ProductStatusFilter>("all")
  const [categoryId, setCategoryId] = useState<string>("")
  const [sort, setSort] = useState<string>("updated-desc")
  const [searchDraft, setSearchDraft] = useState("")
  const [appliedQuery, setAppliedQuery] = useState("")
  const [page, setPage] = useState(1)
  const [data, setData] = useState<AdminProductsPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingProductId, setEditingProductId] = useState<string | null>(null)
  const [detailSheetOpen, setDetailSheetOpen] = useState(false)
  const [viewingProductId, setViewingProductId] = useState<string | null>(null)
  const [mutatingId, setMutatingId] = useState<string | null>(null)

  const filterCategories = useMemo(() => {
    const map = new Map<string, string>()
    for (const preset of CATEGORY_PRESETS) {
      map.set(preset.id, preset.name)
    }
    for (const cat of data?.categories ?? []) {
      if (!map.has(cat.categoryId)) {
        map.set(cat.categoryId, cat.categoryName || cat.categoryId)
      }
    }
    return Array.from(map.entries()).map(([id, name]) => ({ id, name }))
  }, [data?.categories])

  const loadProducts = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true)
      setErrorMessage(null)
      try {
        const res = await listAdminProducts(
          {
            status: statusFilter,
            categoryId: categoryId || undefined,
            sort,
            q: appliedQuery || undefined,
            page,
            pageSize: 12,
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
    [statusFilter, categoryId, sort, appliedQuery, page]
  )

  useEffect(() => {
    const controller = new AbortController()
    void loadProducts(controller.signal)
    return () => controller.abort()
  }, [loadProducts])

  const handleToggleDeleted = useCallback(
    async (row: AdminProductSummary) => {
      setMutatingId(row.id)
      setErrorMessage(null)
      try {
        await toggleAdminProductDeleted(row.id, !row.deleted)
        await loadProducts()
      } catch (error) {
        setErrorMessage(resolveErrorMessage(error))
      } finally {
        setMutatingId(null)
      }
    },
    [loadProducts]
  )

  const handleSearchSubmit = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setAppliedQuery(searchDraft.trim())
  }

  const columns = useMemo<ColumnDef<AdminProductSummary>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Sản phẩm",
        cell: ({ row }) => {
          const item = row.original
          const cover = item.images?.[0]
          return (
            <button
              type="button"
              onClick={() => {
                setViewingProductId(item.id)
                setDetailSheetOpen(true)
              }}
              className="group flex min-w-[240px] cursor-pointer items-center gap-3 text-left focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
            >
              {cover ? (
                <div className="flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-background p-0.5 transition-colors group-hover:border-primary">
                  <img
                    src={cover}
                    alt={item.name}
                    className="max-h-full max-w-full object-contain"
                  />
                </div>
              ) : (
                <div className="flex size-12 shrink-0 items-center justify-center rounded-md border border-border bg-muted text-lg transition-colors group-hover:border-primary">
                  {item.emoji || "📦"}
                </div>
              )}
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="truncate font-medium text-foreground group-hover:text-primary group-hover:underline">
                    {item.name}
                  </span>
                  {item.videoCount > 0 && (
                    <Badge variant="default" className="gap-1 px-1.5 py-0 text-[10px]">
                      <Film className="size-3" />
                      {item.videoCount}
                    </Badge>
                  )}
                </div>
                <div className="font-mono text-xs text-muted-foreground">
                  {item.brand} · {item.id}
                </div>
              </div>
            </button>
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
        cell: ({ row }) => {
          const item = row.original
          const maxPrice = item.maxPrice ?? item.price
          const hasRange = maxPrice > item.price
          return (
            <div className="text-right">
              <div className="font-mono font-semibold text-foreground tabular-nums">
                {hasRange ? `Từ ${formatCurrencyVnd(item.price)}` : formatCurrencyVnd(item.price)}
              </div>
              {hasRange ? (
                <div className="font-mono text-[11px] text-muted-foreground tabular-nums">
                  đến {formatCurrencyVnd(maxPrice)}
                </div>
              ) : (
                item.originalPrice > item.price && (
                  <div className="font-mono text-[11px] text-muted-foreground line-through tabular-nums">
                    {formatCurrencyVnd(item.originalPrice)}
                  </div>
                )
              )}
            </div>
          )
        },
      },
      {
        id: "stock",
        header: () => <div className="text-right">Khả dụng</div>,
        cell: ({ row }) => {
          const stock = row.original.stock
          if (!stock) {
            return <div className="text-right font-mono text-xs text-muted-foreground">—</div>
          }
          const variant =
            stock.availableQuantity <= 0
              ? "destructive"
              : stock.availableQuantity <= 5
                ? "warning"
                : "success"
          return (
            <div className="text-right">
              <Badge variant={variant} className="font-mono tabular-nums">
                {formatNumber(stock.availableQuantity)} khả dụng
              </Badge>
            </div>
          )
        },
      },
      {
        accessorKey: "deleted",
        header: "Trạng thái",
        cell: ({ row }) =>
          row.original.deleted ? (
            <Badge variant="secondary">Đã ẩn</Badge>
          ) : (
            <Badge variant="success">Đang bán</Badge>
          ),
      },
      {
        id: "actions",
        header: () => <div className="text-right">Thao tác</div>,
        cell: ({ row }) => {
          const item = row.original
          return (
            <div className="flex items-center justify-end gap-1.5">
              <Button
                type="button"
                variant="outline"
                size="sm"
                aria-label={`Xem chi tiết ${item.name}`}
                onClick={() => {
                  setViewingProductId(item.id)
                  setDetailSheetOpen(true)
                }}
              >
                <Eye className="size-3.5" />
                Xem
              </Button>
              {canUpdate && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  aria-label={`Chỉnh sửa ${item.name}`}
                  onClick={() => {
                    setEditingProductId(item.id)
                    setDrawerOpen(true)
                  }}
                >
                  <Pencil className="size-3.5" />
                  Sửa
                </Button>
              )}
              {canDelete && (
                <Button
                  type="button"
                  variant={item.deleted ? "secondary" : "outline"}
                  size="sm"
                  disabled={mutatingId === item.id}
                  aria-label={item.deleted ? `Khôi phục ${item.name}` : `Ẩn ${item.name}`}
                  onClick={() => void handleToggleDeleted(item)}
                >
                  {item.deleted ? (
                    <>
                      <RotateCcw className="size-3.5" />
                      Khôi phục
                    </>
                  ) : (
                    <>
                      <Archive className="size-3.5" />
                      Ẩn
                    </>
                  )}
                </Button>
              )}
            </div>
          )
        },
      },
    ],
    [canUpdate, canDelete, mutatingId, handleToggleDeleted]
  )

  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const totalAll = (data?.activeCount ?? 0) + (data?.deletedCount ?? 0)

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Danh mục sản phẩm & Media
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Quản lý thông tin thương mại, biến thể và tệp hình ảnh/video lưu trữ trên S3/MinIO.
          </p>
        </div>

        {canCreate && (
          <Button
            type="button"
            onClick={() => {
              setEditingProductId(null)
              setDrawerOpen(true)
            }}
          >
            <Plus className="size-4" />
            Thêm sản phẩm
          </Button>
        )}
      </div>

      {data?.stockDegraded && (
        <div className="flex items-center gap-2.5 rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-xs font-medium text-amber-950">
          <AlertTriangle className="size-4 shrink-0 text-amber-700" />
          <span>
            Dịch vụ tồn kho tạm thời phản hồi chậm; số liệu tồn trên bảng sản phẩm đang ở chế độ dự
            phòng.
          </span>
        </div>
      )}

      {errorMessage && (
        <div
          role="alert"
          className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900"
        >
          {errorMessage}
        </div>
      )}

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
          <div className="inline-flex flex-wrap items-center gap-1 rounded-lg border border-border bg-muted p-1">
            {(
              [
                { key: "all", label: "Tất cả", count: totalAll },
                { key: "active", label: "Đang bán", count: data?.activeCount ?? 0 },
                { key: "deleted", label: "Đã ẩn", count: data?.deletedCount ?? 0 },
              ] as const
            ).map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => {
                  setStatusFilter(tab.key)
                  setPage(1)
                }}
                className={cn(
                  "inline-flex cursor-pointer items-center gap-2 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                  statusFilter === tab.key
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

          <form onSubmit={handleSearchSubmit} className="flex flex-wrap items-center gap-2">
            <select
              aria-label="Lọc danh mục"
              value={categoryId}
              onChange={(e) => {
                setCategoryId(e.target.value)
                setPage(1)
              }}
              className="h-9 rounded-lg border border-input bg-card px-3 text-xs font-medium text-foreground outline-none focus-visible:border-ring"
            >
              <option value="">Tất cả danh mục</option>
              {filterCategories.map((cat) => (
                <option key={cat.id} value={cat.id}>
                  {cat.name}
                </option>
              ))}
            </select>

            <select
              aria-label="Sắp xếp sản phẩm"
              value={sort}
              onChange={(e) => {
                setSort(e.target.value)
                setPage(1)
              }}
              className="h-9 rounded-lg border border-input bg-card px-3 text-xs font-medium text-foreground outline-none focus-visible:border-ring"
            >
              <option value="updated-desc">Mới cập nhật</option>
              <option value="pho-bien">Bán chạy</option>
              <option value="gia-thap">Giá thấp → cao</option>
              <option value="gia-cao">Giá cao → thấp</option>
              <option value="moi-nhat">Sản phẩm mới</option>
              <option value="danh-gia">Đánh giá cao</option>
            </select>

            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={searchDraft}
                onChange={(e) => setSearchDraft(e.target.value)}
                placeholder="Tìm theo tên hoặc thương hiệu…"
                className="w-56 pl-8 text-xs sm:w-64"
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
              Không tìm thấy sản phẩm phù hợp với bộ lọc hiện tại.
            </div>
          )}

          {data && data.totalPages > 1 && (
            <div className="flex items-center justify-between border-t border-border px-5 py-3 text-xs text-muted-foreground">
              <span>
                Trang <span className="font-mono font-semibold text-foreground">{data.page}</span> /{" "}
                <span className="font-mono">{data.totalPages}</span> ({formatNumber(data.totalItems)}{" "}
                mã hàng)
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

      <ProductDetailSheet
        open={detailSheetOpen}
        onOpenChange={setDetailSheetOpen}
        productId={viewingProductId}
        canUpdate={canUpdate}
        onEdit={(id) => {
          setEditingProductId(id)
          setDrawerOpen(true)
        }}
      />

      <ProductDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        productId={editingProductId}
        canUpdateInventory={canUpdateInventory}
        categories={data?.categories}
        onSaved={() => void loadProducts()}
      />
    </div>
  )
}
