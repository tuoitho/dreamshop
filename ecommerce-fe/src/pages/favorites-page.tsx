import { useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import {
  AlertTriangle,
  ChevronRight,
  Heart,
  ShieldAlert,
  ShoppingCart,
  Star,
  Trash2,
} from "lucide-react"
import { formatRating } from "@/lib/product-availability"
import type { FavoriteItem } from "@/lib/types"
import { cn, formatVND } from "@/lib/utils"
import { ProductImage } from "@/components/product-image"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useCart } from "@/hooks/use-cart"
import { useFavorites } from "@/hooks/use-favorites"

type FavoriteFilter = "all" | "in_stock" | "unavailable"

function isPurchasableFavorite(item: FavoriteItem): boolean {
  return (
    (item.status === "AVAILABLE" || item.status === "LIMITED" || item.status === "UNKNOWN") &&
    item.variants.length > 0 &&
    Boolean(item.name.trim())
  )
}

function isInStockGroup(item: FavoriteItem): boolean {
  return item.status === "AVAILABLE" || item.status === "LIMITED" || item.status === "UNKNOWN"
}

function statusBadgeInfo(item: FavoriteItem): { label: string; className: string } {
  switch (item.status) {
    case "AVAILABLE":
      return {
        label: "Còn hàng",
        className: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300",
      }
    case "LIMITED":
      return {
        label: `Chỉ còn ${item.availableQuantity} sản phẩm`,
        className: "bg-amber-500/15 text-amber-700 dark:text-amber-300",
      }
    case "OUT_OF_STOCK":
      return {
        label: "Tạm hết hàng",
        className: "bg-destructive/15 text-destructive",
      }
    case "UNKNOWN":
      return {
        label: "Tạm chưa rõ tồn kho",
        className: "bg-amber-500/15 text-amber-700 dark:text-amber-300",
      }
    default:
      return {
        label: "Ngừng kinh doanh",
        className: "bg-muted text-muted-foreground",
      }
  }
}

function FavoriteItemCard({
  item,
  pending,
  onRemove,
  onAddToCart,
}: {
  item: FavoriteItem
  pending: boolean
  onRemove: (productId: string) => Promise<void>
  onAddToCart: (productId: string, variantLabel: string) => Promise<void>
}) {
  const [imageError, setImageError] = useState(false)
  const [addingToCart, setAddingToCart] = useState(false)

  useEffect(() => {
    setImageError(false)
  }, [item.imageUrl])

  const isOrphanDeleted = !item.name.trim()
  const displayName = item.name.trim() || "Sản phẩm không còn tồn tại"
  const purchasable = isPurchasableFavorite(item)
  const hasImage = Boolean(item.imageUrl) && !imageError
  const badge = statusBadgeInfo(item)
  const discount =
    !isOrphanDeleted && item.originalPrice > item.price
      ? Math.round(((item.originalPrice - item.price) / item.originalPrice) * 100)
      : 0

  async function handleSingleVariantAdd() {
    if (!purchasable || item.variants.length !== 1 || addingToCart) return
    setAddingToCart(true)
    try {
      await onAddToCart(item.productId, item.variants[0].label)
    } catch {
      // Mutation error notification is displayed by the HTTP response interceptor.
    } finally {
      setAddingToCart(false)
    }
  }

  async function handleRemove() {
    if (pending) return
    try {
      await onRemove(item.productId)
    } catch {
      // Mutation error notification is displayed by the HTTP response interceptor.
    }
  }

  const mediaContent = hasImage ? (
    <img
      src={item.imageUrl}
      alt={displayName}
      loading="lazy"
      className={cn(
        "h-full w-full object-cover transition-transform duration-300 group-hover:scale-105",
        !purchasable && "opacity-75 grayscale-[25%]"
      )}
      onError={() => setImageError(true)}
    />
  ) : (
    <ProductImage
      emoji={item.imageEmoji || "🛍️"}
      hue={[item.imageHue[0] ?? 200, item.imageHue[1] ?? 240]}
      seed={item.productId}
      label={displayName}
      className={cn(!purchasable && "opacity-75 grayscale-[25%]")}
    />
  )

  return (
    <article
      className={cn(
        "group flex flex-col gap-4 rounded-2xl p-4 ring-1 ring-foreground/10 transition-shadow sm:flex-row",
        purchasable ? "bg-card hover:shadow-md" : "bg-muted/30"
      )}
    >
      {isOrphanDeleted ? (
        <div className="relative block size-28 shrink-0 self-center overflow-hidden rounded-xl ring-1 ring-foreground/10 sm:self-start">
          {mediaContent}
        </div>
      ) : (
        <Link
          to={`/san-pham/${item.productId}`}
          className="relative block size-28 shrink-0 self-center overflow-hidden rounded-xl ring-1 ring-foreground/10 sm:self-start"
          aria-label={displayName}
        >
          {mediaContent}
          {discount > 0 && purchasable && (
            <span className="absolute top-1.5 left-1.5 rounded-md bg-red-600 px-1.5 py-0.5 text-[0.65rem] font-bold text-white">
              -{discount}%
            </span>
          )}
        </Link>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            {!isOrphanDeleted && (
              <div className="mb-1 flex flex-wrap items-center gap-1.5">
                {Boolean(item.brand) && <Badge variant="secondary">{item.brand}</Badge>}
                <span
                  className={cn(
                    "inline-flex items-center rounded-md px-2 py-0.5 text-[0.7rem] font-semibold",
                    badge.className
                  )}
                >
                  {badge.label}
                </span>
              </div>
            )}
            {isOrphanDeleted ? (
              <div>
                <p className="text-sm font-semibold text-muted-foreground">{displayName}</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  Sản phẩm đã bị gỡ khỏi gian hàng
                </p>
                <span
                  className={cn(
                    "mt-2 inline-flex items-center rounded-md px-2 py-0.5 text-[0.7rem] font-semibold",
                    badge.className
                  )}
                >
                  {badge.label}
                </span>
              </div>
            ) : (
              <Link
                to={`/san-pham/${item.productId}`}
                className="line-clamp-2 text-base font-semibold leading-snug hover:text-primary"
              >
                {displayName}
              </Link>
            )}
          </div>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="shrink-0 text-muted-foreground hover:text-destructive"
            aria-label={`Bỏ yêu thích ${displayName}`}
            disabled={pending}
            onClick={() => void handleRemove()}
          >
            <Trash2 className="size-4" />
          </Button>
        </div>

        {!isOrphanDeleted && (
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            {item.rating > 0 && (
              <>
                <Star className="size-3.5 fill-amber-400 text-amber-400" />
                <span className="font-semibold text-foreground">{formatRating(item.rating)}</span>
                <span>·</span>
              </>
            )}
            <span>Đã bán {item.soldCount.toLocaleString("vi-VN")}</span>
          </div>
        )}

        <div className="mt-auto flex flex-wrap items-end justify-between gap-3 pt-3">
          {!isOrphanDeleted ? (
            <div className="flex flex-col">
              {item.originalPrice > item.price && (
                <span className="text-xs text-muted-foreground line-through">
                  {formatVND(item.originalPrice)}
                </span>
              )}
              <span
                className={cn(
                  "text-base font-bold",
                  purchasable ? "text-red-600" : "text-muted-foreground"
                )}
              >
                {formatVND(item.price)}
              </span>
            </div>
          ) : (
            <div />
          )}

          <div className="flex items-center gap-2">
            {purchasable ? (
              item.variants.length === 1 ? (
                <Button
                  type="button"
                  size="sm"
                  disabled={addingToCart || pending}
                  onClick={() => void handleSingleVariantAdd()}
                >
                  <ShoppingCart className="size-4" />
                  {addingToCart ? "Đang thêm..." : "Thêm vào giỏ"}
                </Button>
              ) : (
                <Button size="sm" variant="secondary" asChild>
                  <Link to={`/san-pham/${item.productId}`}>
                    <ShoppingCart className="size-4" />
                    Chọn phân loại
                  </Link>
                </Button>
              )
            ) : (
              <Button type="button" size="sm" variant="secondary" disabled>
                {item.status === "OUT_OF_STOCK" ? "Tạm hết hàng" : "Ngừng kinh doanh"}
              </Button>
            )}
          </div>
        </div>
      </div>
    </article>
  )
}

export default function FavoritesPage() {
  const {
    items,
    isPending,
    removeFavorite,
    clearFavorites,
    loading,
    degraded,
    forbidden,
    loadError,
    retryLoad,
  } = useFavorites()
  const { addItem } = useCart()
  const [filter, setFilter] = useState<FavoriteFilter>("all")
  const [clearingAll, setClearingAll] = useState(false)

  const inStockCount = useMemo(() => items.filter(isInStockGroup).length, [items])
  const unavailableCount = items.length - inStockCount

  const filteredItems = useMemo(() => {
    if (filter === "in_stock") return items.filter(isInStockGroup)
    if (filter === "unavailable") return items.filter((item) => !isInStockGroup(item))
    return items
  }, [filter, items])

  async function handleClearAll() {
    if (clearingAll || items.length === 0) return
    setClearingAll(true)
    try {
      await clearFavorites()
    } catch {
      // Mutation error notification is displayed by the HTTP response interceptor.
    } finally {
      setClearingAll(false)
    }
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      <nav className="flex items-center gap-1 text-xs text-muted-foreground">
        <Link to="/" className="hover:text-primary">
          Trang chủ
        </Link>
        <ChevronRight className="size-3.5" />
        <span className="text-foreground">Sản phẩm yêu thích</span>
      </nav>

      <div className="mt-4 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Sản phẩm yêu thích</h1>
          {!forbidden && items.length > 0 && (
            <p className="mt-1 text-sm text-muted-foreground">
              Đã lưu {items.length} sản phẩm trong tài khoản của bạn
            </p>
          )}
        </div>

        {!forbidden && items.length > 0 && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={clearingAll}
            onClick={() => void handleClearAll()}
          >
            <Trash2 className="size-4" />
            {clearingAll ? "Đang xóa..." : "Xóa tất cả"}
          </Button>
        )}
      </div>

      {forbidden ? (
        <div className="mt-8 flex flex-col items-center justify-center rounded-3xl bg-card px-6 py-16 text-center ring-1 ring-foreground/10">
          <ShieldAlert className="size-12 text-muted-foreground" />
          <p className="mt-4 max-w-md text-base font-medium">
            Tính năng sản phẩm yêu thích chỉ dành cho tài khoản khách hàng.
          </p>
          <Button className="mt-6" asChild>
            <Link to="/">Về trang chủ</Link>
          </Button>
        </div>
      ) : loading && items.length === 0 ? (
        <div
          className="mt-6 grid gap-4 md:grid-cols-2"
          role="status"
          aria-label="Đang tải danh sách yêu thích"
        >
          {Array.from({ length: 4 }).map((_, index) => (
            <div
              key={index}
              className="flex gap-4 rounded-2xl bg-card p-4 ring-1 ring-foreground/10"
            >
              <Skeleton className="size-28 shrink-0 rounded-xl" />
              <div className="flex-1 space-y-3">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-5 w-3/4" />
                <Skeleton className="h-4 w-1/3" />
                <Skeleton className="h-8 w-32" />
              </div>
            </div>
          ))}
        </div>
      ) : loadError && items.length === 0 ? (
        <div
          role="alert"
          className="mt-8 flex flex-col items-center justify-center rounded-3xl bg-card px-6 py-16 text-center ring-1 ring-foreground/10"
        >
          <AlertTriangle className="size-12 text-amber-500" />
          <h2 className="mt-4 text-lg font-bold">Không tải được danh sách yêu thích</h2>
          <p className="mt-1 text-sm text-muted-foreground">{loadError.message}</p>
          <Button className="mt-6" onClick={() => void retryLoad()}>
            Thử lại
          </Button>
        </div>
      ) : items.length === 0 ? (
        <div className="mt-8 flex flex-col items-center justify-center rounded-3xl bg-card px-6 py-20 text-center ring-1 ring-foreground/10">
          <div className="grid size-16 place-items-center rounded-2xl bg-red-500/10 text-red-600">
            <Heart className="size-8" />
          </div>
          <h2 className="mt-4 text-xl font-bold">Chưa có sản phẩm yêu thích</h2>
          <p className="mt-1 max-w-md text-sm text-muted-foreground">
            Hãy nhấn biểu tượng trái tim trên sản phẩm bạn quan tâm để theo dõi giá ưu đãi và tình trạng còn hàng.
          </p>
          <Button className="mt-6" asChild>
            <Link to="/tim-kiem">Khám phá sản phẩm</Link>
          </Button>
        </div>
      ) : (
        <div className="mt-6 space-y-5">
          {degraded && (
            <div
              role="status"
              className="flex items-start gap-3 rounded-2xl border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-950 dark:text-amber-200"
            >
              <AlertTriangle className="mt-0.5 size-5 shrink-0 text-amber-600 dark:text-amber-400" />
              <p>
                Hệ thống tạm thời chưa thể kiểm tra đầy đủ tồn kho hoặc thông tin sản phẩm. Một số mặt hàng hiển thị trạng thái tạm thời.
              </p>
            </div>
          )}

          <div className="flex flex-wrap items-center gap-2">
            {(
              [
                { id: "all", label: `Tất cả (${items.length})` },
                { id: "in_stock", label: `Còn hàng (${inStockCount})` },
                { id: "unavailable", label: `Hết hàng / Ngừng bán (${unavailableCount})` },
              ] as const
            ).map((tab) => {
              const active = filter === tab.id
              return (
                <Button
                  key={tab.id}
                  type="button"
                  size="sm"
                  variant={active ? "default" : "outline"}
                  aria-pressed={active}
                  onClick={() => setFilter(tab.id)}
                >
                  {tab.label}
                </Button>
              )
            })}
          </div>

          {filteredItems.length === 0 ? (
            <div className="rounded-2xl bg-card p-10 text-center ring-1 ring-foreground/10">
              <p className="text-sm text-muted-foreground">
                Không có sản phẩm nào trong nhóm lọc này.
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="mt-4"
                onClick={() => setFilter("all")}
              >
                Xem tất cả
              </Button>
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              {filteredItems.map((item) => (
                <FavoriteItemCard
                  key={item.itemId || item.productId}
                  item={item}
                  pending={isPending(item.productId)}
                  onRemove={removeFavorite}
                  onAddToCart={(productId, variantLabel) => addItem(productId, variantLabel, 1)}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
