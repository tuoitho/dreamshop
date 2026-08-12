import { useEffect, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"
import {
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
import { getProduct, searchProducts, softDeleteProduct, toErrorInfo } from "@/lib/api"
import type { ProductDetail, ProductSummary } from "@/lib/types"
import { formatVND } from "@/lib/utils"
import { ProductCard } from "@/components/product-card"
import { ProductImage } from "@/components/product-image"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useCart } from "@/hooks/use-cart"
import { useAuth } from "@/hooks/use-auth"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

const MAX_QUANTITY = 10

interface BuyNowRequest {
  productId: string
  variantLabel: string
  quantity: number
}

export default function ProductDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { isAuthenticated } = useAuth()
  const { addItem, items, selectOnly } = useCart()

  const [product, setProduct] = useState<ProductDetail | null>(null)
  const [error, setError] = useState<{ message: string; code?: string } | null>(null)
  const [loading, setLoading] = useState(true)
  const [reloadKey, setReloadKey] = useState(0)

  const [variantIndex, setVariantIndex] = useState(0)
  const [quantity, setQuantity] = useState(1)
  const [added, setAdded] = useState(false)
  const [activeIndex, setActiveIndex] = useState(0)
  const [galleryError, setGalleryError] = useState(false)
  const [buyNowRequest, setBuyNowRequest] = useState<BuyNowRequest | null>(null)
  const [buyNowResponseReady, setBuyNowResponseReady] = useState(false)
  const [addPending, setAddPending] = useState(false)
  const [buyNowPending, setBuyNowPending] = useState(false)
  const [softDeletePending, setSoftDeletePending] = useState(false)

  const [related, setRelated] = useState<ProductSummary[] | null>(null)

  const variantLabel = product?.variants[variantIndex]?.label ?? ""

  useEffect(() => {
    setVariantIndex(0)
    setQuantity(1)
    setAdded(false)
    setActiveIndex(0)
    setGalleryError(false)
    setBuyNowRequest(null)
    setBuyNowResponseReady(false)
    setAddPending(false)
    setBuyNowPending(false)
    setSoftDeletePending(false)
    setProduct(null)
    setError(null)
    setLoading(true)
    setRelated(null)

    if (!id) {
      setError({ message: "Không tìm thấy sản phẩm", code: "PRODUCT_NOT_FOUND" })
      setLoading(false)
      return
    }

    const controller = new AbortController()
    let active = true
    getProduct(id, controller.signal)
      .then((data) => {
        if (active) {
          setProduct(data)
          setLoading(false)
        }
      })
      .catch((err) => {
        if (active) {
          setError(toErrorInfo(err))
          setLoading(false)
        }
      })
    return () => {
      active = false
      controller.abort()
    }
  }, [id, reloadKey])

  useEffect(() => {
    if (!buyNowRequest || !buyNowResponseReady) return
    const item = items.find((entry) =>
      entry.productId === buyNowRequest.productId &&
      entry.variantLabel === buyNowRequest.variantLabel &&
      (entry.status === "AVAILABLE" || entry.status === "LIMITED")
    )
    setBuyNowRequest(null)
    setBuyNowResponseReady(false)
    setBuyNowPending(false)
    if (!item) {
      toast.error("Không tìm thấy phân loại có thể mua trong giỏ hàng")
      return
    }
    selectOnly(item.itemId)
    navigate("/gio-hang/dat-hang")
  }, [buyNowRequest, buyNowResponseReady, items, navigate, selectOnly])

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
          <Button className="mt-6" asChild>
            <Link to="/">Về trang chủ</Link>
          </Button>
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

  const discount = product.originalPrice > product.price
    ? Math.round(((product.originalPrice - product.price) / product.originalPrice) * 100)
    : 0

  const images = product.images ?? []
  const activeImage = images.length > 0 && !galleryError
    ? images[Math.min(activeIndex, images.length - 1)]
    : undefined

  const requireAuthentication = () => {
    if (isAuthenticated) return true
    navigate("/login", { state: { from: `/san-pham/${product.id}` } })
    return false
  }

  const handleAddToCart = async () => {
    if (!requireAuthentication()) return
    if (addPending || buyNowPending || softDeletePending) return
    setAddPending(true)
    try {
      await addItem(product.id, variantLabel, quantity)
      setAdded(true)
      window.setTimeout(() => setAdded(false), 1600)
    } catch (err) {
      toast.error(toErrorInfo(err).message)
    } finally {
      setAddPending(false)
    }
  }

  const handleBuyNow = async () => {
    if (!requireAuthentication()) return
    if (addPending || buyNowPending || softDeletePending) return
    const request = { productId: product.id, variantLabel, quantity }
    setBuyNowRequest(request)
    setBuyNowResponseReady(false)
    setBuyNowPending(true)
    try {
      await addItem(request.productId, request.variantLabel, request.quantity)
      setBuyNowResponseReady(true)
    } catch (err) {
      setBuyNowRequest(null)
      setBuyNowResponseReady(false)
      setBuyNowPending(false)
      toast.error(toErrorInfo(err).message)
    }
  }

  const handleSoftDelete = async () => {
    if (addPending || buyNowPending || softDeletePending) return
    setSoftDeletePending(true)
    try {
      await softDeleteProduct(product.id, !product.deleted)
      setReloadKey((key) => key + 1)
    } catch (err) {
      toast.error(toErrorInfo(err).message)
    } finally {
      setSoftDeletePending(false)
    }
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-6">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-1 text-xs text-muted-foreground">
        <Link to="/" className="hover:text-primary">Trang chủ</Link>
        <ChevronRight className="size-3.5" />
        <Link
          to={`/tim-kiem?danh-muc=${product.categoryId}`}
          className="hover:text-primary"
        >
          {product.categoryName}
        </Link>
        <ChevronRight className="size-3.5" />
        <span className="line-clamp-1 max-w-60">{product.name}</span>
      </nav>

      <div className="mt-6 grid gap-8 lg:grid-cols-2">
        {/* Gallery */}
        <div className="relative overflow-hidden rounded-3xl ring-1 ring-foreground/10">
          {activeImage ? (
            <>
              <img
                src={activeImage}
                alt={product.name}
                className="aspect-square w-full object-cover"
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
              className="aspect-square"
            />
          )}
          {discount > 0 && (
            <span className="absolute top-4 left-4 rounded-lg bg-red-600 px-2 py-1 text-xs font-bold text-white">
              -{discount}%
            </span>
          )}
        </div>

        {/* Info */}
        <div>
          <div className="flex items-center gap-2">
            <Badge variant="secondary">{product.brand}</Badge>
            {product.badge === "new" && <Badge variant="success">Mới về</Badge>}
            {product.badge === "hot" && <Badge variant="sale">🔥 Bán chạy</Badge>}
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
            <span className="font-semibold">{product.rating}</span>
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
              <span className="text-3xl font-extrabold text-red-600">{formatVND(product.price)}</span>
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
                <span>Phân loại</span>: <span className="font-normal text-muted-foreground">{variantLabel}</span>
              </p>
              <div className="mt-3 flex flex-wrap gap-2">
                {product.variants.map((variant, index) => (
                  <button
                    key={variant.label}
                    onClick={() => setVariantIndex(index)}
                    className={cn(
                      "flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors",
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
                onClick={() => setQuantity((value) => Math.max(1, value - 1))}
                aria-label="Giảm số lượng"
              >
                <Minus />
              </Button>
              <span className="w-12 text-center text-sm font-semibold" aria-live="polite">
                {quantity}
              </span>
              <Button
                variant="outline"
                size="icon"
                onClick={() => setQuantity((value) => Math.min(MAX_QUANTITY, value + 1))}
                aria-label="Tăng số lượng"
              >
                <Plus />
              </Button>
              <span className="text-xs text-muted-foreground">
                Mua tối đa {MAX_QUANTITY} sản phẩm mỗi lần
              </span>
            </div>
          </div>

          <div className="mt-6 flex flex-wrap gap-3">
            <Button
              size="lg"
              className="flex-1 min-w-44"
              disabled={addPending || buyNowPending || softDeletePending}
              onClick={() => void handleAddToCart()}
            >
              <ShoppingCart />
              {addPending ? "Đang thêm..." : added ? "Đã thêm vào giỏ" : "Thêm vào giỏ"}
            </Button>
            <Button
              size="lg"
              variant="outline"
              className="flex-1 min-w-36"
              disabled={addPending || buyNowPending || softDeletePending}
              onClick={() => void handleBuyNow()}
            >
              {buyNowPending ? "Đang xử lý..." : "Mua ngay"}
            </Button>
            <Button size="icon" variant="outline" aria-label="Thêm vào yêu thích">
              <Heart />
            </Button>
            <Button size="icon" variant="outline" aria-label="Chia sẻ sản phẩm">
              <Share2 />
            </Button>
          </div>

          {import.meta.env.DEV && (
            <div className="mt-4 flex items-center gap-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">DEV</span>
              <Button
                variant="outline"
                size="sm"
                disabled={addPending || buyNowPending || softDeletePending}
                onClick={() => void handleSoftDelete()}
              >
                {softDeletePending ? "Đang cập nhật..." : product.deleted ? "Khôi phục sản phẩm" : "Xóa mềm sản phẩm"}
              </Button>
            </div>
          )}

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

      {/* Details */}
      <div className="mt-10 grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="rounded-3xl bg-card p-6 ring-1 ring-foreground/10">
          <h2 className="text-lg font-bold">Chi tiết sản phẩm</h2>
          <Separator className="my-4" />
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

      {/* Related */}
      {related !== null && related.length > 0 ? (
        <section className="mt-12">
          <h2 className="text-xl font-bold">Sản phẩm tương tự</h2>
          <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {related.map((product) => (
              <ProductCard key={product.id} product={product} />
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
