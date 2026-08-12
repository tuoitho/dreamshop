import { useState } from "react"
import { Link } from "react-router-dom"
import { ShoppingCart, Star } from "lucide-react"
import type { ProductSummary } from "@/lib/types"
import { formatVND } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { ProductImage } from "@/components/product-image"
import { useCart } from "@/hooks/use-cart"
import { cn } from "@/lib/utils"

interface ProductCardProps {
  product: ProductSummary
  className?: string
}

export function ProductCard({ product, className }: ProductCardProps) {
  const { addItem } = useCart()
  const [imageError, setImageError] = useState(false)

  const image = product.images?.[0]
  const hasImage = Boolean(image) && !imageError

  const discount = product.originalPrice > product.price
    ? Math.round(((product.originalPrice - product.price) / product.originalPrice) * 100)
    : 0

  return (
    <div
      className={cn(
        "group relative flex flex-col overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10 transition-shadow hover:shadow-lg",
        className
      )}
    >
      <Link
        to={`/san-pham/${product.id}`}
        className="relative block aspect-square overflow-hidden"
        aria-label={product.name}
      >
        {hasImage && image ? (
          <img
            src={image}
            alt={product.name}
            loading="lazy"
            className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
            onError={() => setImageError(true)}
          />
        ) : (
          <ProductImage
            emoji={product.emoji ?? "🛍️"}
            hue={product.imageHue ?? [200, 240]}
            seed={product.id}
            label={product.name}
            className="transition-transform duration-300 group-hover:scale-105"
          />
        )}
        {discount > 0 && (
          <span className="absolute top-2 left-2 rounded-md bg-red-600 px-1.5 py-0.5 text-[0.65rem] font-bold text-white">
            -{discount}%
          </span>
        )}
        {product.badge === "hot" && (
          <Badge variant="sale" className="absolute top-2 right-2">🔥 HOT</Badge>
        )}
        {product.badge === "new" && (
          <Badge variant="success" className="absolute top-2 right-2">Mới</Badge>
        )}
      </Link>

      <div className="flex flex-1 flex-col gap-1.5 p-3">
        <Link to={`/san-pham/${product.id}`}>
          <h3 className="line-clamp-2 min-h-10 text-sm font-medium leading-snug hover:text-primary">
            {product.name}
          </h3>
        </Link>

        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <Star className="size-3.5 fill-amber-400 text-amber-400" />
          <span className="font-semibold text-foreground">{product.rating}</span>
          <span>·</span>
          <span>Đã bán {product.soldCount.toLocaleString("vi-VN")}</span>
        </div>

        <div className="mt-auto flex items-end justify-between gap-2 pt-1">
          <div className="flex flex-col">
            <span className="text-[0.7rem] text-muted-foreground line-through">
              {product.originalPrice > product.price ? formatVND(product.originalPrice) : "\u00A0"}
            </span>
            <span className="text-sm font-bold text-red-600">{formatVND(product.price)}</span>
          </div>
          <Button
            size="icon"
            variant="secondary"
            className="shrink-0"
            aria-label={`Thêm ${product.name} vào giỏ`}
            onClick={() => addItem(product.id, 0, 1)}
          >
            <ShoppingCart />
          </Button>
        </div>
      </div>
    </div>
  )
}
