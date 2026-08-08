import { useEffect, useRef, useState, type FormEvent } from "react"
import { Link, useNavigate } from "react-router-dom"
import { Menu, Search, ShoppingCart, User, X } from "lucide-react"
import { useCart } from "@/hooks/use-cart"
import { useAuth } from "@/hooks/use-auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

const NAV_LINKS = [
  { label: "Trang chủ", to: "/" },
  { label: "Điện thoại", to: "/tim-kiem?danh-muc=dien-thoai" },
  { label: "Laptop", to: "/tim-kiem?danh-muc=laptop" },
  { label: "Tai nghe", to: "/tim-kiem?danh-muc=tai-nghe" },
  { label: "Đồng hồ", to: "/tim-kiem?danh-muc=dong-ho" },
  { label: "Khuyến mãi", to: "/tim-kiem?giam-gia=1" },
]

export function SiteHeader() {
  const { totalQuantity } = useCart()
  const { isAuthenticated } = useAuth()
  const navigate = useNavigate()
  const [query, setQuery] = useState("")
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onPointerDown(event: PointerEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false)
      }
    }
    document.addEventListener("pointerdown", onPointerDown)
    return () => document.removeEventListener("pointerdown", onPointerDown)
  }, [])

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const q = query.trim()
    navigate(q ? `/tim-kiem?q=${encodeURIComponent(q)}` : "/tim-kiem")
  }

  return (
    <header className="sticky top-0 z-40 border-b bg-background/90 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-7xl items-center gap-3 px-4 sm:gap-6">
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="lg:hidden"
            aria-label="Mở menu"
            onClick={() => setMenuOpen((open) => !open)}
          >
            <Menu />
          </Button>
          <Link to="/" className="flex items-center gap-2">
            <span className="grid size-8 place-items-center rounded-lg bg-primary text-base font-extrabold text-primary-foreground">
              S
            </span>
            <span className="hidden text-lg font-extrabold tracking-tight sm:inline">
              S-Shop
            </span>
          </Link>
        </div>

        <form onSubmit={handleSubmit} className="relative flex-1 max-w-xl">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Tìm sản phẩm, thương hiệu..."
            className="pl-8"
            aria-label="Tìm kiếm sản phẩm"
          />
        </form>

        <nav className="ml-auto flex items-center gap-1">
          <Button variant="ghost" size="sm" className="hidden sm:inline-flex" asChild>
            <Link to={isAuthenticated ? "/dashboard" : "/login"}>
              <User />
              <span className="hidden md:inline">{isAuthenticated ? "Tài khoản" : "Đăng nhập"}</span>
            </Link>
          </Button>
          <Button variant="ghost" size="icon" className="relative" asChild aria-label="Giỏ hàng">
            <Link to="/gio-hang">
              <ShoppingCart />
              {totalQuantity > 0 && (
                <span className="absolute -top-0.5 -right-0.5 grid min-w-4 place-items-center rounded-full bg-red-600 px-1 text-[0.6rem] font-bold text-white">
                  {totalQuantity > 99 ? "99+" : totalQuantity}
                </span>
              )}
            </Link>
          </Button>
        </nav>
      </div>

      <nav className="hidden border-t lg:block">
        <div className="mx-auto flex max-w-7xl items-center gap-1 px-4">
          {NAV_LINKS.map((link) => (
            <Link
              key={link.to}
              to={link.to}
              className="px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-primary"
            >
              {link.label}
            </Link>
          ))}
        </div>
      </nav>

      {menuOpen && (
        <div ref={menuRef} className="border-t bg-background lg:hidden">
          <div className="flex items-center justify-between px-4 py-2">
            <span className="text-sm font-semibold">Danh mục</span>
            <Button variant="ghost" size="icon" aria-label="Đóng menu" onClick={() => setMenuOpen(false)}>
              <X />
            </Button>
          </div>
          <div className="grid gap-1 px-2 pb-3">
            {NAV_LINKS.map((link) => (
              <Link
                key={link.to}
                to={link.to}
                className={cn(
                  "rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
                )}
                onClick={() => setMenuOpen(false)}
              >
                {link.label}
              </Link>
            ))}
          </div>
        </div>
      )}
    </header>
  )
}
