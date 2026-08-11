import { useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { ArrowRight, BadgePercent, Flame, Sparkles } from "lucide-react"
import { categories, products } from "@/data/products"
import { ProductCard } from "@/components/product-card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ProductImage } from "@/components/product-image"
import { cn } from "@/lib/utils"

export default function HomePage() {
  const [activeCategory, setActiveCategory] = useState<string>("all")

  const hotProducts = useMemo(
    () => [...products].sort((a, b) => b.soldCount - a.soldCount).slice(0, 8),
    []
  )

  const saleProducts = useMemo(
    () =>
      [...products]
        .filter((p) => p.originalPrice > p.price)
        .sort((a, b) => (b.originalPrice - b.price) - (a.originalPrice - a.price))
        .slice(0, 4),
    []
  )

  const featured = products.find((p) => p.id === "lt-1") ?? products[0]
  const featuredProducts = useMemo(
    () =>
      activeCategory === "all"
        ? products
        : products.filter((p) => p.categoryId === activeCategory),
    [activeCategory]
  )

  return (
    <div className="pb-12">
      {/* Hero */}
      <section className="relative overflow-hidden bg-gradient-to-br from-primary via-emerald-700 to-primary text-primary-foreground">
        <div className="pointer-events-none absolute inset-0 opacity-20 [background-image:radial-gradient(circle_at_20%_20%,white_1px,transparent_1px),radial-gradient(circle_at_80%_60%,white_1px,transparent_1px)] [background-size:48px_48px]" />
        <div className="relative mx-auto grid max-w-7xl gap-8 px-4 py-12 sm:py-16 lg:grid-cols-[1.1fr_1fr] lg:items-center">
          <div>
            <Badge variant="accent">
              <Sparkles className="size-3" /> Sự kiện mua sắm mùa hè
            </Badge>
            <h1 className="mt-4 max-w-xl text-3xl leading-tight font-extrabold sm:text-4xl lg:text-5xl">
              Mua sắm thông minh,
              <br />
              nhận ngàn ưu đãi
            </h1>
            <p className="mt-4 max-w-lg text-sm text-primary-foreground/80 sm:text-base">
              Hàng nghìn sản phẩm chính hãng từ các thương hiệu lớn với giá tốt nhất thị
              trường. Giao nhanh 2h nội thành, đổi trả miễn phí 30 ngày.
            </p>
            <div className="mt-6 flex flex-wrap gap-3">
              <Button
                variant="secondary"
                size="lg"
                className="bg-white text-primary hover:bg-white/90"
                asChild
              >
                <Link to="/tim-kiem">
                  Mua sắm ngay
                  <ArrowRight />
                </Link>
              </Button>
              <Button
                variant="outline"
                size="lg"
                className="border-white/40 bg-transparent text-white hover:bg-white/10 hover:text-white"
                asChild
              >
                <Link to="/tim-kiem?giam-gia=1">
                  <BadgePercent />
                  Xem khuyến mãi
                </Link>
              </Button>
            </div>
          </div>

          <div className="relative mx-auto w-full max-w-md">
            <div className="overflow-hidden rounded-3xl shadow-2xl ring-4 ring-white/20">
              <ProductImage
                emoji={featured.emoji}
                hue={featured.imageHue}
                seed={featured.id}
                label={featured.name}
              />
            </div>
            <div className="absolute -bottom-4 -left-4 hidden rounded-2xl bg-white p-3 shadow-lg sm:block">
              <p className="text-[0.65rem] text-muted-foreground">Giá chỉ từ</p>
              <p className="text-lg font-extrabold text-primary">3.450.000đ</p>
            </div>
          </div>
        </div>
      </section>

      {/* Categories */}
      <section className="mx-auto max-w-7xl px-4 pt-12">
        <div className="flex items-end justify-between">
          <div>
            <h2 className="text-xl font-bold sm:text-2xl">Danh mục nổi bật</h2>
            <p className="mt-1 text-sm text-muted-foreground">Khám phá hàng trăm sản phẩm mới mỗi ngày</p>
          </div>
          <Link to="/tim-kiem" className="hidden items-center gap-1 text-sm font-medium text-primary hover:underline sm:flex">
            Xem tất cả <ArrowRight className="size-4" />
          </Link>
        </div>
        <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8">
          {categories.map((category) => (
            <Link
              key={category.id}
              to={`/tim-kiem?danh-muc=${category.id}`}
              className="group flex flex-col items-center gap-2 rounded-2xl border border-transparent bg-card p-4 ring-1 ring-foreground/10 transition-all hover:-translate-y-1 hover:shadow-md"
            >
              <span className="grid size-12 place-items-center rounded-xl bg-muted text-2xl transition-colors group-hover:bg-primary/10">
                {category.emoji}
              </span>
              <span className="text-center text-xs font-medium">{category.name}</span>
            </Link>
          ))}
        </div>
      </section>

      {/* Flash sale */}
      {saleProducts.length > 0 && (
        <section className="mx-auto max-w-7xl px-4 pt-12">
          <div className="rounded-3xl bg-gradient-to-r from-red-600 to-rose-500 p-6 sm:p-8">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Flame className="size-6 text-white" />
                <h2 className="text-xl font-extrabold text-white sm:text-2xl">Flash Sale</h2>
              </div>
              <div className="flex items-center gap-1 text-sm font-semibold text-white">
                Kết thúc trong
                <div className="ml-1 flex items-center gap-1">
                  {["02", "14", "36"].map((value, i) => (
                    <span key={i} className="grid size-7 place-items-center rounded-md bg-white/20 backdrop-blur">
                      {value}
                    </span>
                  ))}
                </div>
              </div>
            </div>
            <div className="mt-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
              {saleProducts.map((product) => (
                <ProductCard key={product.id} product={product} className="bg-white/95" />
              ))}
            </div>
          </div>
        </section>
      )}

      {/* Featured by category */}
      <section className="mx-auto max-w-7xl px-4 pt-12">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-xl font-bold sm:text-2xl">Sản phẩm nổi bật</h2>
            <p className="mt-1 text-sm text-muted-foreground">Lựa chọn hàng đầu theo từng danh mục</p>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {[{ id: "all", name: "Tất cả" }, ...categories.map((c) => ({ id: c.id, name: c.name }))].map((category) => (
              <button
                key={category.id}
                onClick={() => setActiveCategory(category.id)}
                className={cn(
                  "rounded-full px-3 py-1.5 text-xs font-medium transition-colors",
                  activeCategory === category.id
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-muted-foreground hover:bg-secondary hover:text-foreground"
                )}
              >
                {category.name}
              </button>
            ))}
          </div>
        </div>
        <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {featuredProducts.slice(0, 8).map((product) => (
            <ProductCard key={product.id} product={product} />
          ))}
        </div>
      </section>

      {/* Best sellers */}
      <section className="mx-auto max-w-7xl px-4 pt-12">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-bold sm:text-2xl">Bán chạy nhất</h2>
            <p className="mt-1 text-sm text-muted-foreground">Được mua nhiều nhất trong tuần</p>
          </div>
          <Link to="/tim-kiem" className="hidden items-center gap-1 text-sm font-medium text-primary hover:underline sm:flex">
            Xem tất cả <ArrowRight className="size-4" />
          </Link>
        </div>
        <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {hotProducts.map((product) => (
            <ProductCard key={product.id} product={product} />
          ))}
        </div>
      </section>

      {/* CTA banner */}
      <section className="mx-auto max-w-7xl px-4 pt-12">
        <div className="relative overflow-hidden rounded-3xl bg-gradient-to-r from-primary to-teal-600 px-6 py-10 text-center text-primary-foreground sm:px-10">
          <div className="pointer-events-none absolute inset-0 opacity-20 [background-image:radial-gradient(circle_at_80%_20%,white_1px,transparent_1px)] [background-size:36px_36px]" />
          <div className="relative">
            <h2 className="text-2xl font-extrabold sm:text-3xl">Đăng ký nhận ưu đãi hằng ngày</h2>
            <p className="mx-auto mt-2 max-w-xl text-sm text-primary-foreground/80">
              Nhận mã giảm giá lên đến 200.000đ và cập nhật các chương trình khuyến mãi sớm nhất.
            </p>
            <Button
              variant="secondary"
              size="lg"
              className="mt-6 bg-white text-primary hover:bg-white/90"
              asChild
            >
              <Link to="/register">Đăng ký miễn phí</Link>
            </Button>
          </div>
        </div>
      </section>
    </div>
  )
}
