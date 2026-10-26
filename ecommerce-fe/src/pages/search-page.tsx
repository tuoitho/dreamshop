import { useEffect, useMemo, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { SlidersHorizontal, X } from "lucide-react"
import { categories } from "@/data/products"
import { searchProducts, toErrorInfo } from "@/lib/api"
import type { ProductSort, SearchProductsParams, SearchProductsResult } from "@/lib/types"
import { ProductCard } from "@/components/product-card"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Slider } from "@/components/ui/slider"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 20

const PRICE_RANGES = [
  { id: "0-500000", label: "Dưới 500.000đ" },
  { id: "500000-2000000", label: "500.000đ - 2 triệu" },
  { id: "2000000-10000000", label: "2 triệu - 10 triệu" },
  { id: "10000000-0", label: "Trên 10 triệu" },
]

const SORT_OPTIONS: { value: ProductSort; label: string }[] = [
  { value: "pho-bien", label: "Phổ biến nhất" },
  { value: "gia-thap", label: "Giá thấp đến cao" },
  { value: "gia-cao", label: "Giá cao đến thấp" },
  { value: "moi-nhat", label: "Hàng mới nhất" },
  { value: "danh-gia", label: "Đánh giá tốt nhất" },
]

interface PriceRange {
  min: number
  max: number
}

function parsePriceRange(value: string): PriceRange | null {
  const [min, max] = value.split("-").map(Number)
  if (Number.isNaN(min) || Number.isNaN(max)) return null
  return { min, max }
}

export default function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [showFilters, setShowFilters] = useState(false)
  const [sliderValue, setSliderValue] = useState<number[]>([0, 10000000])

  const [result, setResult] = useState<SearchProductsResult | null>(null)
  const [error, setError] = useState<{ message: string; code?: string } | null>(null)
  const [loading, setLoading] = useState(true)
  const [reloadKey, setReloadKey] = useState(0)

  const rawQuery = searchParams.get("q")?.trim() ?? ""
  const q = rawQuery.toLowerCase()
  const selectedCategory = searchParams.get("danh-muc") ?? "all"
  const giamGia = searchParams.get("giam-gia") === "1"
  const priceRangeParam = searchParams.get("gia")
  const ratingParam = Number(searchParams.get("danh-gia")) || 0
  const sort = (searchParams.get("sap-xep") ?? "pho-bien") as ProductSort
  const page = Math.max(1, Number(searchParams.get("page")) || 1)

  const priceRange = priceRangeParam ? parsePriceRange(priceRangeParam) : null
  const priceMin = priceRange?.min
  const priceMax = priceRange && priceRange.max > 0 ? priceRange.max : undefined

  useEffect(() => {
    if (!priceRangeParam) {
      setSliderValue([0, 10000000])
      return
    }
    const parsed = parsePriceRange(priceRangeParam)
    if (!parsed) {
      setSliderValue([0, 10000000])
      return
    }
    const min = Math.min(10000000, Math.max(0, parsed.min))
    const max =
      parsed.max > 0 ? Math.min(10000000, Math.max(min, parsed.max)) : 10000000
    setSliderValue([min, max])
  }, [priceRangeParam])

  const apiParams = useMemo<SearchProductsParams>(() => {
    const params: SearchProductsParams = {}
    if (q) params.q = q
    if (selectedCategory !== "all") params.categoryId = selectedCategory
    if (priceMin !== undefined) params.priceMin = priceMin
    if (priceMax !== undefined) params.priceMax = priceMax
    if (giamGia) params.onSale = true
    if (ratingParam > 0) params.ratingMin = ratingParam
    params.sort = sort
    params.page = page
    params.pageSize = PAGE_SIZE
    return params
  }, [q, selectedCategory, giamGia, priceMin, priceMax, ratingParam, sort, page])

  useEffect(() => {
    const controller = new AbortController()
    let active = true
    setLoading(true)
    setError(null)
    searchProducts(apiParams, controller.signal)
      .then((data) => {
        if (active) {
          setResult(data)
          setLoading(false)
        }
      })
      .catch((err) => {
        if (active) {
          setResult(null)
          setError(toErrorInfo(err))
          setLoading(false)
        }
      })
    return () => {
      active = false
      controller.abort()
    }
  }, [apiParams, reloadKey])

  function updateParams(updates: Record<string, string | null>) {
    const next = new URLSearchParams(searchParams)
    const onlyPaging = Object.keys(updates).every((key) => key === "page")
    if (!onlyPaging) next.delete("page")
    for (const [key, value] of Object.entries(updates)) {
      if (value === null || value === "" || value === "all" || value === "0") {
        next.delete(key)
      } else {
        next.set(key, value)
      }
    }
    setSearchParams(next, { replace: true })
  }

  const hasActiveFilters = Boolean(
    rawQuery || selectedCategory !== "all" || giamGia || priceRange || ratingParam > 0
  )
  const activeFilterCount =
    (selectedCategory !== "all" ? 1 : 0) +
    (giamGia ? 1 : 0) +
    (priceRange ? 1 : 0) +
    (ratingParam > 0 ? 1 : 0)

  const categoryName =
    selectedCategory !== "all"
      ? categories.find((c) => c.id === selectedCategory)?.name
      : null

  const headingTitle = rawQuery
    ? `Kết quả cho "${rawQuery}"`
    : categoryName
      ? categoryName
      : giamGia
        ? "Sản phẩm khuyến mãi"
        : "Tất cả sản phẩm"

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">{headingTitle}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {result ? `${result.totalItems} sản phẩm` : "\u00A0"}
            {rawQuery && (
              <button
                type="button"
                onClick={() => updateParams({ q: null })}
                className="ml-2 text-primary hover:underline"
              >
                Xóa tìm kiếm
              </button>
            )}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            className="lg:hidden"
            aria-expanded={showFilters}
            onClick={() => setShowFilters((open) => !open)}
          >
            <SlidersHorizontal />
            {activeFilterCount > 0 ? `Bộ lọc (${activeFilterCount})` : "Bộ lọc"}
          </Button>
          <Select
            value={sort}
            onValueChange={(value) => updateParams({ "sap-xep": value as ProductSort })}
          >
            <SelectTrigger className="w-44">
              <SelectValue placeholder="Sắp xếp" />
            </SelectTrigger>
            <SelectContent>
              {SORT_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="mt-6 grid gap-6 lg:grid-cols-[260px_1fr]">
        {/* Filters */}
        <aside className={cn("lg:block", showFilters ? "block" : "hidden")}>
          <div className="sticky top-24 space-y-6 rounded-2xl border bg-card p-5">
            <div className="flex items-center justify-between">
              <p className="text-sm font-bold">Bộ lọc</p>
              {hasActiveFilters && (
                <Button variant="ghost" size="xs" onClick={() => setSearchParams({})}>
                  <X />
                  Xóa tất cả
                </Button>
              )}
            </div>

            <div>
              <p className="mb-2 text-sm font-semibold">Danh mục</p>
              <div className="grid gap-2">
                {[{ id: "all", name: "Tất cả" }, ...categories].map((category) => (
                  <Label key={category.id} className="flex cursor-pointer items-center gap-2 font-normal">
                    <Checkbox
                      checked={selectedCategory === category.id}
                      onCheckedChange={() =>
                        updateParams({ "danh-muc": category.id === "all" ? null : category.id })
                      }
                    />
                    <span className="text-sm">{category.name}</span>
                  </Label>
                ))}
              </div>
            </div>

            <div className="border-t pt-4">
              <p className="mb-2 text-sm font-semibold">Khoảng giá</p>
              <div className="grid gap-2">
                {PRICE_RANGES.map((range) => {
                  const active = priceRangeParam === range.id
                  return (
                    <Label
                      key={range.id}
                      className="flex cursor-pointer items-center gap-2 font-normal"
                    >
                      <Checkbox
                        checked={active}
                        onCheckedChange={() =>
                          updateParams({ gia: active ? null : range.id })
                        }
                      />
                      <span className="text-sm">{range.label}</span>
                    </Label>
                  )
                })}
              </div>
              <div className="mt-4">
                <Slider
                  value={sliderValue}
                  min={0}
                  max={10000000}
                  step={100000}
                  onValueChange={setSliderValue}
                />
                <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                  <span>0đ</span>
                  <span>{(sliderValue[0] / 1000000).toFixed(1)}tr - {(sliderValue[1] / 1000000).toFixed(1)}tr</span>
                  <span>10tr</span>
                </div>
                <Button
                  size="sm"
                  className="mt-2 w-full"
                  onClick={() =>
                    updateParams({ gia: `${sliderValue[0]}-${sliderValue[1]}` })
                  }
                >
                  Áp dụng
                </Button>
              </div>
            </div>

            <div className="border-t pt-4">
              <p className="mb-2 text-sm font-semibold">Đánh giá</p>
              <div className="grid gap-2">
                {[4, 3, 2].map((rating) => (
                  <Label key={rating} className="flex cursor-pointer items-center gap-2 font-normal">
                    <Checkbox
                      checked={ratingParam === rating}
                      onCheckedChange={() =>
                        updateParams({ "danh-gia": ratingParam === rating ? null : String(rating) })
                      }
                    />
                    <span className="text-sm">{rating}★ trở lên</span>
                  </Label>
                ))}
              </div>
            </div>

            <div className="border-t pt-4">
              <Label className="flex cursor-pointer items-center gap-2 font-normal">
                <Checkbox
                  checked={giamGia}
                  onCheckedChange={(checked) => updateParams({ "giam-gia": checked ? "1" : null })}
                />
                <span className="text-sm">Chỉ sản phẩm giảm giá</span>
              </Label>
            </div>
          </div>
        </aside>

        {/* Results */}
        <div>
          {loading ? (
            <div
              role="status"
              aria-label="Đang tải sản phẩm"
              className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4"
            >
              {Array.from({ length: 8 }).map((_, index) => (
                <div key={index} className="overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
                  <Skeleton className="aspect-square rounded-none" />
                  <div className="space-y-2 p-3">
                    <Skeleton className="h-4 w-full" />
                    <Skeleton className="h-4 w-2/3" />
                    <Skeleton className="h-4 w-1/3" />
                  </div>
                </div>
              ))}
            </div>
          ) : error ? (
            <div
              role="alert"
              className="flex flex-col items-center justify-center rounded-2xl border bg-card py-20 text-center"
            >
              <span className="text-5xl">⚠️</span>
              <h3 className="mt-4 text-lg font-bold">Không tải được danh sách sản phẩm</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground">{error.message}</p>
              <Button className="mt-5" onClick={() => setReloadKey((key) => key + 1)}>
                Thử lại
              </Button>
            </div>
          ) : result && result.items.length > 0 ? (
            <>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
                {result.items.map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
              {result.totalPages > 1 && (
                <nav className="mt-8 flex items-center justify-center gap-2" aria-label="Phân trang">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page <= 1}
                    onClick={() => updateParams({ page: String(page - 1) })}
                  >
                    Trang trước
                  </Button>
                  <span className="text-sm text-muted-foreground">
                    Trang {page} / {result.totalPages}
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={page >= result.totalPages}
                    onClick={() => updateParams({ page: String(page + 1) })}
                  >
                    Trang sau
                  </Button>
                </nav>
              )}
            </>
          ) : (
            <div className="flex flex-col items-center justify-center rounded-2xl border bg-card py-20 text-center">
              <span className="text-5xl">🔍</span>
              <h3 className="mt-4 text-lg font-bold">Không tìm thấy sản phẩm</h3>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground">
                Thử từ khóa khác hoặc xóa bớt bộ lọc để xem thêm kết quả.
              </p>
              <Button className="mt-5" onClick={() => setSearchParams({})}>
                Xóa toàn bộ bộ lọc
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
