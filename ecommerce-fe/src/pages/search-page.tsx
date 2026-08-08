import { useMemo, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { SlidersHorizontal, X } from "lucide-react"
import { categories, products } from "@/data/products"
import { ProductCard } from "@/components/product-card"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { Slider } from "@/components/ui/slider"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

const PRICE_RANGES = [
  { id: "0-500000", label: "Dưới 500.000đ" },
  { id: "500000-2000000", label: "500.000đ - 2 triệu" },
  { id: "2000000-10000000", label: "2 triệu - 10 triệu" },
  { id: "10000000-0", label: "Trên 10 triệu" },
]

type SortKey = "pho-bien" | "gia-thap" | "gia-cao" | "moi-nhat" | "danh-gia"

const SORT_OPTIONS: { value: SortKey; label: string }[] = [
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

  const q = searchParams.get("q")?.toLowerCase() ?? ""
  const selectedCategory = searchParams.get("danh-muc") ?? "all"
  const giamGia = searchParams.get("giam-gia") === "1"
  const priceRangeParam = searchParams.get("gia")
  const ratingParam = Number(searchParams.get("danh-gia")) || 0
  const sort = (searchParams.get("sap-xep") ?? "pho-bien") as SortKey

  const priceRange = priceRangeParam ? parsePriceRange(priceRangeParam) : null

  const filteredProducts = useMemo(() => {
    let result = [...products]

    if (q) {
      result = result.filter(
        (p) =>
          p.name.toLowerCase().includes(q) ||
          p.brand.toLowerCase().includes(q) ||
          p.categoryName.toLowerCase().includes(q)
      )
    }

    if (selectedCategory !== "all") {
      result = result.filter((p) => p.categoryId === selectedCategory)
    }

    if (giamGia) {
      result = result.filter((p) => p.originalPrice > p.price)
    }

    if (priceRange) {
      result = result.filter((p) => {
        if (priceRange.max === 0) return p.price >= priceRange.min
        return p.price >= priceRange.min && p.price <= priceRange.max
      })
    }

    if (ratingParam > 0) {
      result = result.filter((p) => p.rating >= ratingParam)
    }

    switch (sort) {
      case "gia-thap":
        result.sort((a, b) => a.price - b.price)
        break
      case "gia-cao":
        result.sort((a, b) => b.price - a.price)
        break
      case "moi-nhat":
        result.sort((a, b) => Number(b.isNew ?? false) - Number(a.isNew ?? false))
        break
      case "danh-gia":
        result.sort((a, b) => b.rating - a.rating)
        break
      default:
        result.sort((a, b) => b.soldCount - a.soldCount)
    }

    return result
  }, [q, selectedCategory, giamGia, priceRange, ratingParam, sort])

  function updateParams(updates: Record<string, string | null>) {
    const next = new URLSearchParams(searchParams)
    for (const [key, value] of Object.entries(updates)) {
      if (value === null || value === "" || value === "all" || value === "0") {
        next.delete(key)
      } else {
        next.set(key, value)
      }
    }
    setSearchParams(next, { replace: true })
  }

  const hasActiveFilters = q || selectedCategory !== "all" || giamGia || priceRange || ratingParam > 0

  const categoryName = selectedCategory !== "all"
    ? categories.find((c) => c.id === selectedCategory)?.name
    : null

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">
            {q ? `Kết quả cho "${q}"` : categoryName ? categoryName : "Tất cả sản phẩm"}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {filteredProducts.length} sản phẩm
            {q && (
              <button
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
            onClick={() => setShowFilters((open) => !open)}
          >
            <SlidersHorizontal />
            Bộ lọc
          </Button>
          <Select
            value={sort}
            onValueChange={(value) => updateParams({ "sap-xep": value as SortKey })}
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
          {filteredProducts.length > 0 ? (
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
              {filteredProducts.map((product) => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>
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
