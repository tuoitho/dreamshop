import { useContext, useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import {
  ArrowRight,
  BadgePercent,
  Headphones,
  RotateCcw,
  ShieldCheck,
  Sparkles,
  Truck,
} from "lucide-react"
import { categories } from "@/data/products"
import { AuthContext } from "@/context/auth-context"
import { searchProducts, toErrorInfo } from "@/lib/api"
import type { ProductSummary } from "@/lib/types"
import { ProductCard } from "@/components/product-card"
import { Button } from "@/components/ui/button"
import { ProductImage } from "@/components/product-image"
import { Skeleton } from "@/components/ui/skeleton"
import { cn, formatVND } from "@/lib/utils"

const TRUST_PILLARS = [
  { icon: Truck, title: "Giao hàng nhanh", desc: "Miễn phí từ 299.000đ" },
  { icon: ShieldCheck, title: "Chính hãng 100%", desc: "Bảo hành toàn quốc" },
  { icon: RotateCcw, title: "Đổi trả 30 ngày", desc: "Hoàn tiền nhanh chóng" },
  { icon: Headphones, title: "Hỗ trợ 24/7", desc: "Hotline 1900 6789" },
] as const

function SectionOrnament() {
  return (
    <div className="mt-3 flex items-center justify-center gap-2" aria-hidden="true">
      <span className="h-px w-8 bg-border" />
      <svg viewBox="0 0 16 16" className="size-3.5 text-primary" fill="currentColor">
        <path d="M8 1L9.7 6.3L15 8L9.7 9.7L8 15L6.3 9.7L1 8L6.3 6.3L8 1Z" />
      </svg>
      <span className="h-px w-8 bg-border" />
    </div>
  )
}

export default function HomePage() {
  const auth = useContext(AuthContext)
  const isAuthenticated = auth?.isAuthenticated ?? false

  const [activeCategory, setActiveCategory] = useState<string>("all")
  const [items, setItems] = useState<ProductSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<{ message: string; code?: string } | null>(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [heroImageError, setHeroImageError] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    let active = true
    setLoading(true)
    setError(null)
    setHeroImageError(false)

    searchProducts({ sort: "pho-bien", pageSize: 20 }, controller.signal)
      .then((result) => {
        if (active) {
          setItems(result.items)
          setLoading(false)
        }
      })
      .catch((err) => {
        if (active) {
          setItems([])
          setError(toErrorInfo(err))
          setLoading(false)
        }
      })

    return () => {
      active = false
      controller.abort()
    }
  }, [reloadKey])

  const hotProducts = useMemo(
    () => [...items].sort((a, b) => b.soldCount - a.soldCount).slice(0, 8),
    [items]
  )

  const saleProducts = useMemo(
    () =>
      [...items]
        .filter((p) => p.originalPrice > p.price)
        .sort((a, b) => (b.originalPrice - b.price) - (a.originalPrice - a.price))
        .slice(0, 4),
    [items]
  )

  const maxDiscountPercent = useMemo(() => {
    if (saleProducts.length === 0) return 0
    return Math.max(
      ...saleProducts.map((p) => Math.round(((p.originalPrice - p.price) / p.originalPrice) * 100))
    )
  }, [saleProducts])

  const featuredHero = useMemo(
    () => items.find((p) => p.featured) ?? items[0] ?? null,
    [items]
  )

  const minPrice = useMemo(
    () => (items.length > 0 ? Math.min(...items.map((p) => p.price)) : null),
    [items]
  )

  const featuredProducts = useMemo(
    () =>
      activeCategory === "all"
        ? items
        : items.filter((p) => p.categoryId === activeCategory),
    [activeCategory, items]
  )

  const heroImageUrl = featuredHero?.images?.[0]
  const primaryCategories = categories.slice(0, 4)
  const secondaryCategories = categories.slice(4)

  return (
    <div className="bg-background">
      {/* Editorial Split Hero */}
      <section className="relative overflow-hidden bg-surface-warm">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -top-24 -right-24 size-96 rounded-full bg-[#65a30d]/14 blur-3xl"
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -bottom-24 -left-24 size-80 rounded-full bg-primary/10 blur-3xl"
        />
        <div className="relative mx-auto grid max-w-7xl gap-10 px-4 py-14 sm:py-20 lg:grid-cols-[1fr_1.1fr] lg:items-center lg:gap-16 lg:py-24">
          <div className="order-2 mx-auto w-full max-w-md lg:order-1">
            <div className="relative rounded-3xl bg-card p-3 shadow-xl shadow-foreground/5 ring-1 ring-border/70">
              <div className="overflow-hidden rounded-2xl bg-surface-warm">
                {heroImageUrl && !heroImageError ? (
                  <img
                    src={heroImageUrl}
                    alt={featuredHero?.name ?? "Sản phẩm nổi bật"}
                    className="aspect-square w-full object-cover"
                    onError={() => setHeroImageError(true)}
                  />
                ) : (
                  <ProductImage
                    emoji={featuredHero?.emoji ?? "🛍️"}
                    hue={featuredHero?.imageHue ?? [160, 200]}
                    seed={featuredHero?.id ?? "dreamshop-hero"}
                    label={featuredHero?.name ?? "DreamShop"}
                  />
                )}
              </div>
              {minPrice !== null ? (
                <div className="absolute -right-3 -bottom-4 rounded-2xl bg-surface-ink px-4 py-3 text-white shadow-lg sm:-right-5">
                  <p className="text-[11px] font-medium tracking-wide text-[#a3b89f] uppercase">
                    Giá chỉ từ
                  </p>
                  <p className="mt-0.5 text-lg font-bold tracking-tight text-[#bef264]">
                    {formatVND(minPrice)}
                  </p>
                </div>
              ) : null}
            </div>
          </div>

          <div className="order-1 lg:order-2">
            <div className="inline-flex items-center gap-2 text-xs font-semibold tracking-wider text-primary uppercase">
              <Sparkles className="size-3.5 text-gold" />
              <span>Tuyển chọn chính hãng tại DreamShop</span>
            </div>
            <h1 className="mt-4 max-w-xl font-serif text-3xl leading-[1.15] font-bold tracking-tight text-foreground sm:text-5xl">
              Nâng tầm cuộc sống,
              <br />
              mua sắm đầy cảm hứng.
            </h1>
            <p className="mt-5 max-w-lg text-sm leading-relaxed text-muted-foreground sm:text-base">
              Khám phá bộ sưu tập sản phẩm chính hãng được tuyển chọn kỹ lưỡng từ các thương hiệu
              uy tín. Giao nhanh nội thành, bảo hành rõ ràng và đổi trả dễ dàng trong 30 ngày.
            </p>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <Button
                size="lg"
                className="h-11 rounded-full bg-primary px-7 text-primary-foreground shadow-sm hover:bg-primary/90"
                asChild
              >
                <Link to="/tim-kiem">
                  Mua sắm ngay
                  <ArrowRight className="size-4" />
                </Link>
              </Button>
              <Button
                variant="outline"
                size="lg"
                className="h-11 rounded-full border-foreground/20 bg-card/70 px-6 text-foreground hover:bg-card"
                asChild
              >
                <Link to="/tim-kiem?giam-gia=1">
                  <BadgePercent className="size-4 text-primary" />
                  Xem khuyến mãi
                </Link>
              </Button>
            </div>
          </div>
        </div>
      </section>

      {/* Dark Trust Bar */}
      <section className="bg-surface-ink py-8 text-white">
        <div className="mx-auto grid max-w-7xl gap-4 px-4 sm:grid-cols-2 lg:grid-cols-4">
          {TRUST_PILLARS.map((item) => (
            <div
              key={item.title}
              className="flex items-start gap-3.5 rounded-xl border border-white/8 bg-surface-ink-elevated px-5 py-4"
            >
              <span className="mt-0.5 grid size-10 shrink-0 place-items-center rounded-lg bg-[#65a30d]/25 text-[#a3e635]">
                <item.icon className="size-5" />
              </span>
              <div>
                <p className="font-serif text-sm font-semibold text-white">{item.title}</p>
                <p className="mt-0.5 text-xs text-[#a3b89f]">{item.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {loading ? (
        <section
          className="mx-auto max-w-7xl px-4 py-16"
          role="status"
          aria-label="Đang tải sản phẩm trang chủ"
        >
          <div className="mx-auto flex flex-col items-center">
            <Skeleton className="h-8 w-52" />
            <Skeleton className="mt-3 h-3 w-24" />
          </div>
          <div className="mt-10 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 8 }).map((_, index) => (
              <div
                key={index}
                className="overflow-hidden rounded-2xl border border-border/70 bg-card"
              >
                <Skeleton className="aspect-square rounded-none" />
                <div className="space-y-2 p-4">
                  <Skeleton className="h-3 w-1/3" />
                  <Skeleton className="h-4 w-full" />
                  <Skeleton className="h-4 w-2/3" />
                  <Skeleton className="h-4 w-1/2" />
                </div>
              </div>
            ))}
          </div>
        </section>
      ) : error ? (
        <section className="mx-auto max-w-7xl px-4 py-16">
          <div
            role="alert"
            className="flex flex-col items-center justify-center rounded-3xl border bg-surface-warm py-16 text-center"
          >
            <span className="text-4xl">⚠️</span>
            <h2 className="mt-3 font-serif text-xl font-bold">
              Không tải được sản phẩm trang chủ
            </h2>
            <p className="mt-1 max-w-sm text-sm text-muted-foreground">{error.message}</p>
            <Button className="mt-6 rounded-full px-6" onClick={() => setReloadKey((k) => k + 1)}>
              Thử lại
            </Button>
          </div>
        </section>
      ) : (
        <>
          {/* Best Sellers */}
          {hotProducts.length > 0 && (
            <section className="bg-background py-16 sm:py-20">
              <div className="mx-auto max-w-7xl px-4">
                <div className="text-center">
                  <h2 className="font-serif text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
                    Bán chạy nhất
                  </h2>
                  <p className="mt-1.5 text-sm text-muted-foreground">
                    Những sản phẩm được khách hàng tin chọn nhiều nhất trong tuần
                  </p>
                  <SectionOrnament />
                </div>
                <div className="mt-10 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
                  {hotProducts.map((product) => (
                    <ProductCard key={product.id} product={product} />
                  ))}
                </div>
                <div className="mt-8 text-center">
                  <Button
                    variant="outline"
                    className="rounded-full border-foreground/20 px-6 hover:border-primary hover:text-primary"
                    asChild
                  >
                    <Link to="/tim-kiem">
                      Xem tất cả sản phẩm
                      <ArrowRight className="size-4" />
                    </Link>
                  </Button>
                </div>
              </div>
            </section>
          )}

          {/* Editorial Category Highlights */}
          <section className="bg-surface-warm py-16 sm:py-20">
            <div className="mx-auto max-w-7xl px-4">
              <div className="text-center">
                <h2 className="font-serif text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
                  Danh mục nổi bật
                </h2>
                <p className="mt-1.5 text-sm text-muted-foreground">
                  Khám phá nhanh theo nhóm sản phẩm bạn quan tâm
                </p>
                <SectionOrnament />
              </div>

              <div className="mt-10 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
                {primaryCategories.map((category) => (
                  <Link
                    key={category.id}
                    to={`/tim-kiem?danh-muc=${category.id}`}
                    className="group flex flex-col justify-between rounded-2xl border border-border/70 bg-card p-6 transition-all duration-200 hover:-translate-y-1 hover:shadow-md"
                  >
                    <div>
                      <span className="grid size-12 place-items-center rounded-xl bg-surface-warm text-2xl transition-colors group-hover:bg-primary/10">
                        {category.emoji}
                      </span>
                      <h3 className="mt-5 font-serif text-lg font-bold text-foreground group-hover:text-primary">
                        {category.name}
                      </h3>
                      <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
                        {category.description}
                      </p>
                    </div>
                    <span className="mt-5 inline-flex items-center gap-1.5 text-xs font-semibold tracking-wide text-primary uppercase">
                      Khám phá
                      <ArrowRight className="size-3.5 transition-transform group-hover:translate-x-1" />
                    </span>
                  </Link>
                ))}
              </div>

              {secondaryCategories.length > 0 && (
                <div className="mt-6 flex flex-wrap items-center justify-center gap-2.5">
                  {secondaryCategories.map((category) => (
                    <Link
                      key={category.id}
                      to={`/tim-kiem?danh-muc=${category.id}`}
                      className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-card px-4 py-2 text-xs font-medium text-foreground transition-colors hover:border-primary hover:text-primary"
                    >
                      <span aria-hidden="true">{category.emoji}</span>
                      <span>{category.name}</span>
                    </Link>
                  ))}
                </div>
              )}
            </div>
          </section>

          {/* Mid-page Dark Promo Breakout Strip + Sale Products */}
          {saleProducts.length > 0 && (
            <>
              <section className="bg-surface-ink py-12 text-white sm:py-14">
                <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-6 px-4 text-center md:flex-row md:text-left">
                  <div>
                    <p className="text-xs font-semibold tracking-widest text-[#bef264] uppercase">
                      Ưu đãi đặc biệt hôm nay
                    </p>
                    <h2 className="mt-2 font-serif text-2xl font-bold tracking-tight sm:text-3xl">
                      {maxDiscountPercent > 0
                        ? `Tiết kiệm đến ${maxDiscountPercent}% cho các sản phẩm chọn lọc`
                        : "Ưu đãi giá tốt cho các sản phẩm chọn lọc"}
                    </h2>
                  </div>
                  <Button
                    size="lg"
                    className="h-11 shrink-0 rounded-full bg-primary px-7 text-primary-foreground hover:bg-primary/90"
                    asChild
                  >
                    <Link to="/tim-kiem?giam-gia=1">
                      Xem tất cả ưu đãi
                      <ArrowRight className="size-4" />
                    </Link>
                  </Button>
                </div>
              </section>

              <section className="bg-background py-14 sm:py-16">
                <div className="mx-auto max-w-7xl px-4">
                  <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    {saleProducts.map((product) => (
                      <ProductCard key={product.id} product={product} />
                    ))}
                  </div>
                </div>
              </section>
            </>
          )}

          {/* Featured by Category */}
          <section className="bg-surface-warm py-16 sm:py-20">
            <div className="mx-auto max-w-7xl px-4">
              <div className="text-center">
                <h2 className="font-serif text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
                  Sản phẩm nổi bật
                </h2>
                <p className="mt-1.5 text-sm text-muted-foreground">
                  Lựa chọn hàng đầu theo từng danh mục
                </p>
                <SectionOrnament />
              </div>

              <div className="mt-8 flex flex-wrap items-center justify-center gap-2">
                {[
                  { id: "all", name: "Tất cả" },
                  ...categories.map((c) => ({ id: c.id, name: c.name })),
                ].map((category) => (
                  <button
                    key={category.id}
                    type="button"
                    aria-pressed={activeCategory === category.id}
                    onClick={() => setActiveCategory(category.id)}
                    className={cn(
                      "rounded-full px-4 py-2 text-xs font-medium transition-colors",
                      activeCategory === category.id
                        ? "bg-primary text-primary-foreground shadow-xs"
                        : "border border-border/80 bg-card text-muted-foreground hover:border-primary/40 hover:text-foreground"
                    )}
                  >
                    {category.name}
                  </button>
                ))}
              </div>

              {featuredProducts.length > 0 ? (
                <div className="mt-10 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
                  {featuredProducts.slice(0, 8).map((product) => (
                    <ProductCard key={product.id} product={product} />
                  ))}
                </div>
              ) : (
                <div className="mt-10 rounded-2xl border border-border/70 bg-card py-14 text-center text-sm text-muted-foreground">
                  Chưa có sản phẩm trong danh mục này.
                </div>
              )}
            </div>
          </section>
        </>
      )}

      {/* Bottom Editorial CTA */}
      <section className="bg-background py-16 sm:py-20">
        <div className="mx-auto max-w-5xl px-4">
          <div className="relative overflow-hidden rounded-3xl border border-border/70 bg-surface-warm px-6 py-12 text-center sm:px-12 sm:py-16">
            <div
              aria-hidden="true"
              className="pointer-events-none absolute -bottom-20 left-1/2 h-48 w-96 -translate-x-1/2 rounded-full bg-primary/10 blur-3xl"
            />
            <div className="relative">
              {isAuthenticated ? (
                <>
                  <p className="text-xs font-semibold tracking-widest text-primary uppercase">
                    Đặc quyền thành viên DreamShop
                  </p>
                  <h2 className="mt-3 font-serif text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
                    Ưu đãi dành riêng cho thành viên
                  </h2>
                  <p className="mx-auto mt-3 max-w-xl text-sm leading-relaxed text-muted-foreground">
                    Khám phá ngay danh sách sản phẩm đang giảm giá sâu và miễn phí giao hàng từ
                    299.000đ.
                  </p>
                  <Button
                    size="lg"
                    className="mt-7 h-11 rounded-full bg-primary px-7 text-primary-foreground hover:bg-primary/90"
                    asChild
                  >
                    <Link to="/tim-kiem?giam-gia=1">Khám phá ưu đãi hôm nay</Link>
                  </Button>
                </>
              ) : (
                <>
                  <p className="text-xs font-semibold tracking-widest text-primary uppercase">
                    Thành viên mới
                  </p>
                  <h2 className="mt-3 font-serif text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
                    Đăng ký nhận ưu đãi hằng ngày
                  </h2>
                  <p className="mx-auto mt-3 max-w-xl text-sm leading-relaxed text-muted-foreground">
                    Nhận thông báo sớm về các chương trình khuyến mãi chính hãng và ưu đãi dành
                    riêng cho tài khoản DreamShop.
                  </p>
                  <Button
                    size="lg"
                    className="mt-7 h-11 rounded-full bg-primary px-7 text-primary-foreground hover:bg-primary/90"
                    asChild
                  >
                    <Link to="/register">Đăng ký miễn phí</Link>
                  </Button>
                </>
              )}
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}
