import { useContext, useState } from "react"
import { Link, useLocation, useNavigate } from "react-router-dom"
import { Heart, ShoppingCart, Star } from "lucide-react"
import { AuthContext } from "@/context/auth-context"
import { useFavorites } from "@/hooks/use-favorites"
import type { ProductSummary } from "@/lib/types"
import { formatRating, resolveProductAvailability } from "@/lib/product-availability"
import { cn, formatVND } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ProductImage } from "@/components/product-image"

interface ProductCardProps {
  product: ProductSummary
  className?: string
}

export function ProductCard({ product, className }: ProductCardProps) {
  const [imageError, setImageError] = useState(false)
  const auth = useContext(AuthContext)
  const isAuthenticated = auth?.isAuthenticated ?? false
  const navigate = useNavigate()
  const location = useLocation()
  const { isFavorite, isPending, toggleFavorite, forbidden: favoritesForbidden } = useFavorites()

  const image = product.images?.[0]
  const hasImage = Boolean(image) && !imageError
  const availability = resolveProductAvailability(product)
  const favorited = isFavorite(product.id)
  const favoritePending = isPending(product.id)
  const favoriteDisabled = favoritePending || (!favorited && availability.status === "UNAVAILABLE")

  async function handleToggleFavorite() {
    if (!isAuthenticated) {
      navigate("/login", { state: { from: `${location.pathname}${location.search}` } })
      return
    }
    if (favoriteDisabled) return
    try {
      await toggleFavorite(product.id)
    } catch {
      // Mutation error notification is displayed by the HTTP response interceptor.
    }
  }

  const discount =
    product.originalPrice > product.price
      ? Math.round(((product.originalPrice - product.price) / product.originalPrice) * 100)
      : 0

  return (
    <div
      className={cn(
        "group relative flex flex-col overflow-hidden rounded-2xl border border-border/70 bg-card transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md",
        className
      )}
    >
      <Link
        to={`/san-pham/${product.id}`}
        className="relative block aspect-square overflow-hidden bg-surface-warm"
        aria-label={product.name}
      >
        {hasImage && image ? (
          <img
            src={image}
            alt={product.name}
            loading="lazy"
            className={cn(
              "h-full w-full object-cover transition-transform duration-300 group-hover:scale-105",
              !availability.purchasable && "opacity-75 grayscale-[25%]"
            )}
            onError={() => setImageError(true)}
          />
        ) : (
          <ProductImage
            emoji={product.emoji ?? "🛍️"}
            hue={product.imageHue ?? [200, 240]}
            seed={product.id}
            label={product.name}
            className={cn(
              "transition-transform duration-300 group-hover:scale-105",
              !availability.purchasable && "opacity-75 grayscale-[25%]"
            )}
          />
        )}
        {discount > 0 && availability.purchasable && (
          <span className="absolute top-3 left-3 rounded-full bg-primary px-2.5 py-0.5 text-[0.68rem] font-semibold tracking-wide text-primary-foreground shadow-xs">
            -{discount}%
          </span>
        )}
        {!availability.purchasable && availability.badgeLabel ? (
          <span className="absolute inset-x-3 bottom-3 rounded-lg bg-foreground/85 px-2 py-1 text-center text-[0.7rem] font-semibold text-background backdrop-blur-xs">
            {availability.badgeLabel}
          </span>
        ) : null}
        {availability.purchasable && product.badge === "hot" && (
          <Badge variant="sale" className="absolute top-3 right-3 rounded-full">
            🔥 HOT
          </Badge>
        )}
        {availability.purchasable && product.badge === "new" && (
          <Badge variant="success" className="absolute top-3 right-3 rounded-full">
            Mới
          </Badge>
        )}
      </Link>

      <div className="flex flex-1 flex-col gap-1.5 p-4">
        {product.categoryName && (
          <span className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
            {product.categoryName}
          </span>
        )}
        <Link to={`/san-pham/${product.id}`}>
          <h3 className="line-clamp-2 min-h-10 text-sm font-semibold leading-snug text-foreground transition-colors hover:text-primary">
            {product.name}
          </h3>
        </Link>

        <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
          <Star className="size-3.5 fill-gold text-gold" />
          <span className="font-semibold text-foreground">{formatRating(product.rating)}</span>
          <span>·</span>
          <span>Đã bán {product.soldCount.toLocaleString("vi-VN")}</span>
          {availability.status === "LIMITED" && availability.badgeLabel ? (
            <span className="ml-auto rounded-full bg-amber-500/15 px-2 py-0.5 text-[0.65rem] font-semibold text-amber-800 dark:text-amber-300">
              {availability.badgeLabel}
            </span>
          ) : null}
        </div>

        <div className="mt-auto flex items-end justify-between gap-2 pt-2">
          <div className="flex flex-col">
            <span className="text-[0.72rem] text-muted-foreground line-through">
              {product.originalPrice > product.price ? formatVND(product.originalPrice) : "\u00A0"}
            </span>
            <span
              className={cn(
                "text-[0.95rem] font-bold tracking-tight",
                availability.purchasable ? "text-primary" : "text-muted-foreground"
              )}
            >
              {product.maxPrice && product.maxPrice > product.price && (
                <span className="mr-1 text-xs font-normal text-muted-foreground">Từ</span>
              )}
              {formatVND(product.price)}
            </span>
          </div>
          <div className="flex shrink-0 items-center gap-1.5">
            {!favoritesForbidden && (
              <Button
                type="button"
                size="icon"
                variant="outline"
                className="size-8 shrink-0 rounded-full border-border/80"
                aria-label={
                  favorited
                    ? `Bỏ ${product.name} khỏi yêu thích`
                    : `Thêm ${product.name} vào yêu thích`
                }
                aria-pressed={favorited}
                disabled={favoriteDisabled}
                onClick={() => void handleToggleFavorite()}
              >
                <Heart className={cn("size-4", favorited && "fill-red-500 text-red-500")} />
              </Button>
            )}
            <Button
              size="icon"
              variant="secondary"
              className="size-8 shrink-0 rounded-full bg-primary/10 text-primary hover:bg-primary hover:text-primary-foreground"
              aria-label={`Chọn phân loại của ${product.name}`}
              asChild
            >
              <Link to={`/san-pham/${product.id}`} aria-label={`Chọn phân loại của ${product.name}`}>
                <ShoppingCart className="size-4" />
              </Link>
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
