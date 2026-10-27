import { useEffect, useRef, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"
import {
  AlertTriangle,
  ChevronRight,
  Heart,
  Minus,
  Plus,
  RotateCcw,
  Share2,
  ShieldCheck,
  ShoppingCart,
  Star,
  Truck,
} from "lucide-react"
import { getProduct, searchProducts, toErrorInfo } from "@/lib/api"
import { formatRating, resolveProductAvailability } from "@/lib/product-availability"
import type { ProductDetail, ProductSummary } from "@/lib/types"
import { cn, formatVND } from "@/lib/utils"
import { ProductCard } from "@/components/product-card"
import { ProductImage } from "@/components/product-image"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useCart } from "@/hooks/use-cart"
import { useFavorites } from "@/hooks/use-favorites"
import { useAuth } from "@/hooks/use-auth"
import { toast } from "sonner"

interface PendingCartMutation {
  productId: string
  variantLabel: string
  quantity: number
  generation: number
}

export default function ProductDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { isAuthenticated } = useAuth()
  const { addItem, degraded, items, selectOnly } = useCart()
  const {
    isFavorite,
    isPending: isFavoritePending,
    toggleFavorite,
    forbidden: favoritesForbidden,
  } = useFavorites()

  const [product, setProduct] = useState<ProductDetail | null>(null)
  const [error, setError] = useState<{ message: string; code?: string } | null>(null)
  const [loading, setLoading] = useState(true)
  const [reloadKey, setReloadKey] = useState(0)

  const [variantIndex, setVariantIndex] = useState(0)
  const [quantity, setQuantity] = useState(1)
  const [added, setAdded] = useState(false)
  const [activeIndex, setActiveIndex] = useState(0)
  const [galleryError, setGalleryError] = useState(false)
  const [addRequest, setAddRequest] = useState<PendingCartMutation | null>(null)
  const [addResponseReady, setAddResponseReady] = useState(false)
  const [buyNowRequest, setBuyNowRequest] = useState<PendingCartMutation | null>(null)
  const [buyNowResponseReady, setBuyNowResponseReady] = useState(false)
  const [addPending, setAddPending] = useState(false)
  const [buyNowPending, setBuyNowPending] = useState(false)

  const [related, setRelated] = useState<ProductSummary[] | null>(null)
  const mountedRef = useRef(false)
  const productGenerationRef = useRef(0)
  const addedTimerRef = useRef<number | null>(null)

  const variantLabel = product?.variants[variantIndex]?.label ?? ""

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      productGenerationRef.current += 1
      if (addedTimerRef.current !== null) {
        window.clearTimeout(addedTimerRef.current)
        addedTimerRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    setVariantIndex(0)
    setQuantity(1)
    setAdded(false)
    setActiveIndex(0)
    setGalleryError(false)
    setAddRequest(null)
    setAddResponseReady(false)
    setBuyNowRequest(null)
    setBuyNowResponseReady(false)
    setAddPending(false)
    setBuyNowPending(false)
    setProduct(null)
    setError(null)
    setLoading(true)
    setRelated(null)

    const generation = ++productGenerationRef.current
    if (addedTimerRef.current !== null) {
      window.clearTimeout(addedTimerRef.current)
      addedTimerRef.current = null
    }

    if (!id) {
      setError({ message: "Không tìm thấy sản phẩm", code: "PRODUCT_NOT_FOUND" })
      setLoading(false)
      return
    }

    const controller = new AbortController()
    let active = true
    getProduct(id, controller.signal)
      .then((data) => {
        if (active && mountedRef.current && productGenerationRef.current === generation) {
          const nextAvailability = resolveProductAvailability(data)
          if (nextAvailability.purchasable) {
            setQuantity((current) =>
              Math.max(1, Math.min(current, nextAvailability.maxQuantity))
            )
          }
          setProduct(data)
          setLoading(false)
        }
      })
      .catch((err) => {
        if (active && mountedRef.current && productGenerationRef.current === generation) {
          setError(toErrorInfo(err))
          setLoading(false)
        }
      })
    return () => {
      active = false
      if (productGenerationRef.current === generation) {
        productGenerationRef.current += 1
      }
      controller.abort()
    }
  }, [id, reloadKey])

  useEffect(() => {
    if (!addRequest || !addResponseReady) return
    if (
      !mountedRef.current ||
      productGenerationRef.current !== addRequest.generation ||
      id !== addRequest.productId
    ) {
      return
    }
    const { generation, productId, variantLabel: targetVariant } = addRequest
    setAddRequest(null)
    setAddResponseReady(false)
    setAddPending(false)

    if (!degraded) {
      const matching = items.find(
        (entry) => entry.productId === productId && entry.variantLabel === targetVariant
      )
      if (matching?.status === "OUT_OF_STOCK") {
        toast.error("Sản phẩm vừa tạm hết hàng")
        setReloadKey((key) => key + 1)
        return
      }
      if (matching?.status === "UNAVAILABLE") {
        toast.error("Sản phẩm đã ngừng kinh doanh")
        setReloadKey((key) => key + 1)
        return
      }
    }

    setAdded(true)
    if (addedTimerRef.current !== null) window.clearTimeout(addedTimerRef.current)
    addedTimerRef.current = window.setTimeout(() => {
      if (mountedRef.current && productGenerationRef.current === generation && id === productId) {
        setAdded(false)
      }
      addedTimerRef.current = null
    }, 1600)
  }, [addRequest, addResponseReady, degraded, id, items])

  useEffect(() => {
    if (!buyNowRequest || !buyNowResponseReady) return
    if (
      !mountedRef.current ||
      productGenerationRef.current !== buyNowRequest.generation ||
      id !== buyNowRequest.productId
    ) {
      return
    }
    const { productId, variantLabel: targetVariant } = buyNowRequest
    setBuyNowRequest(null)
    setBuyNowResponseReady(false)
    setBuyNowPending(false)

    if (degraded) {
      toast.error("Tạm thời chưa kiểm tra được tồn kho, vui lòng kiểm tra giỏ hàng")
      return
    }

    const matching = items.find(
      (entry) => entry.productId === productId && entry.variantLabel === targetVariant
    )
    if (matching?.status === "OUT_OF_STOCK") {
      toast.error("Sản phẩm vừa tạm hết hàng")
      setReloadKey((key) => key + 1)
      return
    }
    if (matching?.status === "UNAVAILABLE") {
      toast.error("Sản phẩm đã ngừng kinh doanh")
      setReloadKey((key) => key + 1)
      return
    }
    if (!matching) {
      toast.error("Không tìm thấy phân loại có thể mua trong giỏ hàng")
      return
    }
    selectOnly(matching.itemId)
    navigate("/gio-hang/dat-hang")
  }, [buyNowRequest, buyNowResponseReady, degraded, id, items, navigate, selectOnly])

  const productCategoryId = product?.categoryId
  const productId = product?.id

  useEffect(() => {
    if (!productCategoryId || !productId) return
    let active = true
    setRelated(null)
    searchProducts({ categoryId: productCategoryId, pageSize: 4, sort: "pho-bien" })
      .then((data) => {
        if (active) {
          setRelated(data.items.filter((item) => item.id !== productId))
        }
      })
      .catch(() => {
        if (active) setRelated([])
      })
    return () => {
      active = false
    }
  }, [productCategoryId, productId])

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-6" role="status" aria-label="Đang tải sản phẩm">
        <Skeleton className="h-4 w-64" />
        <div className="mt-6 grid gap-8 lg:grid-cols-2">
          <Skeleton className="aspect-square rounded-3xl" />
          <div className="space-y-4">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-8 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-24 w-full rounded-2xl" />
            <Skeleton className="h-24 w-full rounded-2xl" />
          </div>
        </div>
      </div>
    )
  }

  if (error) {
    if (error.code === "PRODUCT_NOT_FOUND") {
      return (
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-center px-4 py-24 text-center">
          <span className="text-5xl">🤷</span>
          <h1 className="mt-4 text-xl font-bold">Không tìm thấy sản phẩm</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Sản phẩm không tồn tại hoặc đã bị gỡ khỏi gian hàng.
          </p>
          <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
            <Button asChild>
              <Link to="/">Về trang chủ</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/tim-kiem">Xem tất cả sản phẩm</Link>
            </Button>
          </div>
        </div>
      )
    }
    return (
      <div
        role="alert"
        className="mx-auto flex max-w-7xl flex-col items-center justify-center px-4 py-24 text-center"
      >
        <span className="text-5xl">⚠️</span>
        <h1 className="mt-4 text-xl font-bold">Không tải được sản phẩm</h1>
        <p className="mt-1 text-sm text-muted-foreground">{error.message}</p>
        <Button className="mt-6" onClick={() => setReloadKey((key) => key + 1)}>
          Thử lại
        </Button>
      </div>
    )
  }

  if (!product) return null

  const availability = resolveProductAvailability(product)
  const effectiveQuantity = availability.purchasable
    ? Math.max(1, Math.min(quantity, availability.maxQuantity))
    : 1

  const discount =
    product.originalPrice > product.price
      ? Math.round(((product.originalPrice - product.price) / product.originalPrice) * 100)
      : 0

  const images = product.images ?? []
  const activeImage =
    images.length > 0 && !galleryError
      ? images[Math.min(activeIndex, images.length - 1)]
      : undefined

  const requireAuthentication = () => {
    if (isAuthenticated) return true
    navigate("/login", { state: { from: `/san-pham/${product.id}` } })
    return false
  }

  const handleAddToCart = async () => {
    if (!availability.purchasable) return
    if (!requireAuthentication()) return
    if (addPending || buyNowPending) return
    const request: PendingCartMutation = {
      productId: product.id,
      variantLabel,
      quantity: effectiveQuantity,
      generation: productGenerationRef.current,
    }
    setAddRequest(request)
    setAddResponseReady(false)
    setAddPending(true)
    try {
      await addItem(request.productId, request.variantLabel, request.quantity)
      if (
        !mountedRef.current ||
        productGenerationRef.current !== request.generation ||
        id !== request.productId
      ) {
        return
      }
      setAddResponseReady(true)
    } catch (err) {
      if (
        !mountedRef.current ||
        productGenerationRef.current !== request.generation ||
        id !== request.productId
      ) {
        return
      }
      setAddRequest(null)
      setAddResponseReady(false)
      setAddPending(false)
      toast.error(toErrorInfo(err).message)
    }
  }

  const handleBuyNow = async () => {
    if (!availability.purchasable) return
    if (!requireAuthentication()) return
    if (addPending || buyNowPending) return
    const request: PendingCartMutation = {
      productId: product.id,
      variantLabel,
      quantity: effectiveQuantity,
      generation: productGenerationRef.current,
    }
    setBuyNowRequest(request)
    setBuyNowResponseReady(false)
    setBuyNowPending(true)
    try {
      await addItem(request.productId, request.variantLabel, request.quantity)
      if (
        !mountedRef.current ||
        productGenerationRef.current !== request.generation ||
        id !== request.productId
      ) {
        return
      }
      setBuyNowResponseReady(true)
    } catch (err) {
      if (
        !mountedRef.current ||
        productGenerationRef.current !== request.generation ||
        id !== request.productId
      ) {
        return
      }
      setBuyNowRequest(null)
      setBuyNowResponseReady(false)
      setBuyNowPending(false)
      toast.error(toErrorInfo(err).message)
    }
  }

  const favorited = isFavorite(product.id)
  const favoritePending = isFavoritePending(product.id)
  const favoriteDisabled = favoritePending || (!favorited && availability.status === "UNAVAILABLE")

  const handleToggleFavorite = async () => {
    if (!requireAuthentication()) return
    if (favoriteDisabled) return
    try {
      await toggleFavorite(product.id)
    } catch {
      // Mutation error notification is displayed by the HTTP response interceptor.
    }
  }

  const handleShare = async () => {
    try {
      if (!navigator.clipboard?.writeText) {
        toast.error("Không thể sao chép liên kết sản phẩm")
        return
      }
      await navigator.clipboard.writeText(window.location.href)
      toast.success("Đã sao chép liên kết sản phẩm")
    } catch {
      toast.error("Không thể sao chép liên kết sản phẩm")
    }
  }

  const hasSeparateFullDescription = Boolean(
    product.shortDescription &&
      product.description &&
      product.shortDescription !== product.description
  )

  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      <nav className="flex items-center gap-1 text-xs text-muted-foreground">
        <Link to="/" className="hover:text-primary">
          Trang chủ
        </Link>
        <ChevronRight className="size-3.5" />
        <Link to={`/tim-kiem?danh-muc=${product.categoryId}`} className="hover:text-primary">
          {product.categoryName}
        </Link>
        <ChevronRight className="size-3.5" />
        <span className="line-clamp-1 max-w-60">{product.name}</span>
      </nav>

      {availability.status === "UNAVAILABLE" ? (
        <div
          role="status"
          className="mt-4 flex items-start gap-3 rounded-2xl border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-950 dark:text-amber-200"
        >
          <AlertTriangle className="mt-0.5 size-5 shrink-0 text-amber-600 dark:text-amber-400" />
          <div>
            <p className="font-semibold">Sản phẩm ngừng kinh doanh</p>
            <p className="mt-0.5 text-xs text-amber-900/80 dark:text-amber-200/80">
              Sản phẩm này hiện không còn mở bán. Bạn có thể tham khảo các sản phẩm tương tự bên dưới.
            </p>
          </div>
        </div>
      ) : null}

      <div className="mt-6 grid gap-8 lg:grid-cols-2">
        <div className="relative overflow-hidden rounded-3xl ring-1 ring-foreground/10">
          {activeImage ? (
            <>
              <img
                src={activeImage}
                alt={product.name}
                className={cn(
                  "aspect-square w-full object-cover",
                  !availability.purchasable && "opacity-75 grayscale-[25%]"
                )}
                onError={() => setGalleryError(true)}
              />
              {images.length > 1 && (
                <div className="absolute inset-x-0 bottom-0 flex justify-center gap-2 bg-gradient-to-t from-black/40 to-transparent p-3">
                  {images.map((src, index) => (
                    <button
                      key={src}
                      type="button"
                      onClick={() => setActiveIndex(index)}
                      aria-label={`Xem ảnh ${index + 1}`}
                      aria-pressed={index === activeIndex}
                      className={cn(
                        "size-14 overflow-hidden rounded-lg ring-2 transition-opacity",
                        index === activeIndex
                          ? "ring-white opacity-100"
                          : "ring-white/50 opacity-70 hover:opacity-100"
                      )}
                    >
                      <img src={src} alt="" className="h-full w-full object-cover" />
                    </button>
                  ))}
                </div>
              )}
            </>
          ) : (
            <ProductImage
              emoji={product.emoji ?? "🛍️"}
              hue={product.imageHue ?? [200, 240]}
              seed={product.id}
              label={product.name}
              className={cn("aspect-square", !availability.purchasable && "opacity-75 grayscale-[25%]")}
            />
          )}
          {discount > 0 && availability.purchasable && (
            <span className="absolute top-4 left-4 rounded-lg bg-red-600 px-2 py-1 text-xs font-bold text-white">
              -{discount}%
            </span>
          )}
        </div>

        <div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">{product.brand}</Badge>
            {availability.purchasable && product.badge === "new" && (
              <Badge variant="success">Mới về</Badge>
            )}
            {availability.purchasable && product.badge === "hot" && (
              <Badge variant="sale">🔥 Bán chạy</Badge>
            )}
            {availability.status === "LIMITED" && (
              <span className="rounded-md bg-amber-500/15 px-2 py-0.5 text-xs font-semibold text-amber-700 dark:text-amber-300">
                {availability.detailLabel}
              </span>
            )}
            {availability.status === "AVAILABLE" && (
              <span className="rounded-md bg-emerald-500/15 px-2 py-0.5 text-xs font-semibold text-emerald-700 dark:text-emerald-300">
                {availability.detailLabel}
              </span>
            )}
            {availability.status === "OUT_OF_STOCK" && (
              <span className="rounded-md bg-destructive/10 px-2 py-0.5 text-xs font-semibold text-destructive">
                Tạm hết hàng
              </span>
            )}
            {availability.status === "UNKNOWN" && availability.inventoryDegraded && (
              <span className="rounded-md bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                {availability.detailLabel}
              </span>
            )}
          </div>

          <h1 className="mt-3 text-2xl leading-snug font-bold sm:text-3xl">{product.name}</h1>

          <div className="mt-3 flex flex-wrap items-center gap-2 text-sm">
            <div className="flex items-center gap-1">
              {Array.from({ length: 5 }).map((_, i) => (
                <Star
                  key={i}
                  className={cn(
                    "size-4",
                    i < Math.round(product.rating)
                      ? "fill-amber-400 text-amber-400"
                      : "text-muted"
                  )}
                />
              ))}
            </div>
            <span className="font-semibold">{formatRating(product.rating)}</span>
            <span className="text-muted-foreground">
              ({product.reviewCount.toLocaleString("vi-VN")} đánh giá)
            </span>
            <span className="text-muted-foreground">·</span>
            <span className="text-muted-foreground">
              Đã bán {product.soldCount.toLocaleString("vi-VN")}
            </span>
          </div>

          <div className="mt-5 rounded-2xl bg-card p-5 ring-1 ring-foreground/10">
            <div className="flex items-end gap-3">
              <span
                className={cn(
                  "text-3xl font-extrabold",
                  availability.purchasable ? "text-red-600" : "text-muted-foreground"
                )}
              >
                {formatVND(product.price)}
              </span>
              {product.originalPrice > product.price && (
                <span className="mb-1 text-sm text-muted-foreground line-through">
                  {formatVND(product.originalPrice)}
                </span>
              )}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              Giá đã bao gồm VAT. Miễn phí vận chuyển cho đơn từ 299.000đ.
            </p>
          </div>

          <div className="mt-5 rounded-2xl bg-card p-5 ring-1 ring-foreground/10">
            <p className="text-sm font-semibold">Mô tả ngắn</p>
            <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
              {product.shortDescription ?? product.description}
            </p>
          </div>

          {product.variants.length > 0 && (
            <div className="mt-5 rounded-2xl bg-card p-5 ring-1 ring-foreground/10">
              <p className="text-sm font-semibold">
                <span>Phân loại</span>:{" "}
                <span className="font-normal text-muted-foreground">{variantLabel}</span>
              </p>
              <div className="mt-3 flex flex-wrap gap-2">
                {product.variants.map((variant, index) => (
                  <button
                    key={variant.label}
                    type="button"
                    disabled={!availability.purchasable}
                    onClick={() => setVariantIndex(index)}
                    className={cn(
                      "flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50",
                      variantIndex === index
                        ? "border-primary bg-primary/5 text-primary"
                        : "border-border hover:border-primary/40"
                    )}
                    aria-pressed={variantIndex === index}
                  >
                    {variant.colorHex && (
                      <span
                        className="size-4 rounded-full border border-black/10"
                        style={{ backgroundColor: variant.colorHex }}
                      />
                    )}
                    {variant.label}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="mt-5 flex flex-wrap items-center gap-6">
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="icon"
                disabled={!availability.purchasable || effectiveQuantity <= 1}
                onClick={() =>
                  setQuantity((value) =>
                    Math.max(1, Math.min(availability.maxQuantity, value) - 1)
                  )
                }
                aria-label="Giảm số lượng"
              >
                <Minus />
              </Button>
              <span className="w-12 text-center text-sm font-semibold" aria-live="polite">
                {effectiveQuantity}
              </span>
              <Button
                variant="outline"
                size="icon"
                disabled={!availability.purchasable || effectiveQuantity >= availability.maxQuantity}
                onClick={() =>
                  setQuantity((value) =>
                    Math.min(availability.maxQuantity, Math.max(1, value) + 1)
                  )
                }
                aria-label="Tăng số lượng"
              >
                <Plus />
              </Button>
              {availability.purchasable ? (
                <span className="text-xs text-muted-foreground">
                  Mua tối đa {availability.maxQuantity} sản phẩm mỗi lần
                </span>
              ) : (
                <span className="text-xs text-muted-foreground">
                  {availability.detailLabel}
                </span>
              )}
            </div>
          </div>

          <div className="mt-6 flex flex-wrap gap-3">
            <Button
              size="lg"
              className="flex-1 min-w-44"
              disabled={!availability.purchasable || addPending || buyNowPending}
              onClick={() => void handleAddToCart()}
            >
              <ShoppingCart />
              {!availability.purchasable
                ? availability.ctaLabel
                : addPending
                  ? "Đang thêm..."
                  : added
                    ? "Đã thêm vào giỏ"
                    : "Thêm vào giỏ"}
            </Button>
            <Button
              size="lg"
              variant="outline"
              className="flex-1 min-w-36"
              disabled={!availability.purchasable || addPending || buyNowPending}
              onClick={() => void handleBuyNow()}
            >
              {buyNowPending ? "Đang xử lý..." : "Mua ngay"}
            </Button>
            {!favoritesForbidden && (
              <Button
                type="button"
                size="icon"
                variant="outline"
                aria-label={favorited ? "Bỏ khỏi yêu thích" : "Thêm vào yêu thích"}
                aria-pressed={favorited}
                disabled={favoriteDisabled}
                onClick={() => void handleToggleFavorite()}
              >
                <Heart className={cn(favorited && "fill-red-500 text-red-500")} />
              </Button>
            )}
            <Button
              size="icon"
              variant="outline"
              aria-label="Chia sẻ sản phẩm"
              onClick={() => void handleShare()}
            >
              <Share2 />
            </Button>
          </div>

          <div className="mt-6 grid gap-3 rounded-2xl bg-card p-5 ring-1 ring-foreground/10 sm:grid-cols-3">
            {[
              { icon: Truck, title: "Giao nhanh 2h", desc: "Nội thành HCM & HN" },
              { icon: ShieldCheck, title: "Chính hãng", desc: "Bảo hành 12 tháng" },
              { icon: RotateCcw, title: "Đổi trả 30 ngày", desc: "Hoàn tiền dễ dàng" },
            ].map((item) => (
              <div key={item.title} className="flex items-center gap-2">
                <item.icon className="size-5 shrink-0 text-primary" />
                <div>
                  <p className="text-xs font-semibold">{item.title}</p>
                  <p className="text-[0.7rem] text-muted-foreground">{item.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="mt-10 grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="rounded-3xl bg-card p-6 ring-1 ring-foreground/10">
          <h2 className="text-lg font-bold">Chi tiết sản phẩm</h2>
          <Separator className="my-4" />
          {hasSeparateFullDescription ? (
            <p className="mb-4 text-sm leading-relaxed text-muted-foreground">
              {product.description}
            </p>
          ) : null}
          <ul className="grid gap-3 sm:grid-cols-2">
            {product.details.map((detail) => (
              <li key={detail} className="flex items-start gap-2 text-sm">
                <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-primary" />
                <span className="text-muted-foreground">{detail}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="rounded-3xl bg-card p-6 ring-1 ring-foreground/10">
          <h2 className="text-lg font-bold">Chính sách bán hàng</h2>
          <Separator className="my-4" />
          <ul className="grid gap-3 text-sm text-muted-foreground">
            <li>• Miễn phí vận chuyển đơn từ 299.000đ</li>
            <li>• Đổi trả miễn phí trong 30 ngày</li>
            <li>• Bảo hành chính hãng toàn quốc</li>
            <li>• Thanh toán khi nhận hàng (COD)</li>
            <li>• Trả góp 0% qua thẻ tín dụng</li>
          </ul>
        </div>
      </div>

      {related !== null && related.length > 0 ? (
        <section className="mt-12">
          <h2 className="text-xl font-bold">Sản phẩm tương tự</h2>
          <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {related.map((item) => (
              <ProductCard key={item.id} product={item} />
            ))}
          </div>
        </section>
      ) : related === null ? (
        <section className="mt-12" role="status" aria-label="Đang tải sản phẩm tương tự">
          <h2 className="text-xl font-bold">Sản phẩm tương tự</h2>
          <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {Array.from({ length: 4 }).map((_, index) => (
              <div key={index} className="overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
                <Skeleton className="aspect-square rounded-none" />
                <div className="space-y-2 p-3">
                  <Skeleton className="h-4 w-full" />
                  <Skeleton className="h-4 w-1/3" />
                </div>
              </div>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  )
}
