import { useEffect, useRef, useState, type FormEvent } from "react"
import { Link, useLocation, useNavigate } from "react-router-dom"
import { DropdownMenu } from "radix-ui"
import { Heart, LogOut, Menu, Search, ShoppingCart, User, UserPlus, X } from "lucide-react"
import { useCart } from "@/hooks/use-cart"
import { useFavorites } from "@/hooks/use-favorites"
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

function isNavLinkActive(linkTo: string, pathname: string, search: string): boolean {
  if (linkTo === "/") {
    return pathname === "/"
  }
  const [targetPath, targetSearch = ""] = linkTo.split("?")
  if (pathname !== targetPath) {
    return false
  }
  const currentParams = new URLSearchParams(search)
  const targetParams = new URLSearchParams(targetSearch)
  const targetCategory = targetParams.get("danh-muc")
  if (targetCategory) {
    return currentParams.get("danh-muc") === targetCategory
  }
  if (targetParams.get("giam-gia") === "1") {
    return currentParams.get("giam-gia") === "1" && !currentParams.get("danh-muc")
  }
  return false
}

export function SiteHeader() {
  const { totalQuantity, forbidden: cartForbidden } = useCart()
  const { totalCount: favoriteCount, forbidden: favoritesForbidden } = useFavorites()
  const { isAuthenticated, user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const urlQuery =
    location.pathname === "/tim-kiem"
      ? new URLSearchParams(location.search).get("q") ?? ""
      : ""
  const [query, setQuery] = useState(urlQuery)
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const toggleRef = useRef<HTMLButtonElement>(null)
  const displayName = user?.fullName.trim() || user?.email || "Hồ sơ"

  useEffect(() => {
    setQuery(urlQuery)
  }, [urlQuery])

  useEffect(() => {
    setMenuOpen(false)
  }, [location.pathname, location.search])

  useEffect(() => {
    function onPointerDown(event: PointerEvent) {
      const target = event.target as Node
      if (toggleRef.current?.contains(target)) {
        return
      }
      if (menuRef.current && !menuRef.current.contains(target)) {
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

  async function handleLogout() {
    try {
      await logout()
    } catch {
      // Mutation error toast is shown by the HTTP interceptor; AuthProvider clears local session state in finally.
    } finally {
      setMenuOpen(false)
      navigate("/", { replace: true })
    }
  }

  return (
    <header className="sticky top-0 z-40 border-b bg-background/90 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-7xl items-center gap-3 px-4 sm:gap-6">
        <div className="flex items-center gap-1">
          <Button
            ref={toggleRef}
            variant="ghost"
            size="icon"
            className="lg:hidden"
            aria-label="Mở menu"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((open) => !open)}
          >
            <Menu />
          </Button>
          <Link to="/" className="flex items-center gap-2">
            <span className="grid size-8 place-items-center rounded-lg bg-primary text-base font-extrabold text-primary-foreground">
              T
            </span>
            <span className="hidden text-lg font-extrabold tracking-tight sm:inline">
              TechShop
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
          {isAuthenticated ? (
            <DropdownMenu.Root>
              <DropdownMenu.Trigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  aria-label={displayName}
                >
                  <User />
                  <span className="sr-only max-w-40 truncate md:not-sr-only md:inline">
                    {displayName}
                  </span>
                </Button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Portal>
                <DropdownMenu.Content
                  align="end"
                  sideOffset={6}
                  className="z-50 min-w-44 rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-md"
                >
                  <DropdownMenu.Item
                    asChild
                    className="flex cursor-pointer select-none items-center gap-2 rounded-md px-2.5 py-1.5 text-sm outline-none focus:bg-muted focus:text-foreground data-[highlighted]:bg-muted data-[highlighted]:text-foreground"
                  >
                    <Link to="/profile">
                      <User className="size-4 text-muted-foreground" />
                      <span>Hồ sơ</span>
                    </Link>
                  </DropdownMenu.Item>
                  {!favoritesForbidden && (
                    <DropdownMenu.Item
                      asChild
                      className="flex cursor-pointer select-none items-center gap-2 rounded-md px-2.5 py-1.5 text-sm outline-none focus:bg-muted focus:text-foreground data-[highlighted]:bg-muted data-[highlighted]:text-foreground"
                    >
                      <Link to="/yeu-thich">
                        <Heart className="size-4 text-muted-foreground" />
                        <span>Sản phẩm yêu thích</span>
                      </Link>
                    </DropdownMenu.Item>
                  )}
                  <DropdownMenu.Separator className="my-1 h-px bg-border" />
                  <DropdownMenu.Item
                    onSelect={() => {
                      void handleLogout()
                    }}
                    className="flex cursor-pointer select-none items-center gap-2 rounded-md px-2.5 py-1.5 text-sm text-destructive outline-none focus:bg-destructive/10 focus:text-destructive data-[highlighted]:bg-destructive/10 data-[highlighted]:text-destructive"
                  >
                    <LogOut className="size-4" />
                    <span>Đăng xuất</span>
                  </DropdownMenu.Item>
                </DropdownMenu.Content>
              </DropdownMenu.Portal>
            </DropdownMenu.Root>
          ) : (
            <Button variant="ghost" size="sm" asChild>
              <Link to="/login">
                <User />
                <span className="sr-only md:not-sr-only md:inline">Đăng nhập</span>
              </Link>
            </Button>
          )}
          {!favoritesForbidden && (
            <Button variant="ghost" size="icon" className="relative" asChild aria-label="Sản phẩm yêu thích">
              <Link to="/yeu-thich">
                <Heart />
                {favoriteCount > 0 && (
                  <span className="absolute -top-0.5 -right-0.5 grid min-w-4 place-items-center rounded-full bg-red-600 px-1 text-[0.6rem] font-bold text-white">
                    {favoriteCount > 99 ? "99+" : favoriteCount}
                  </span>
                )}
              </Link>
            </Button>
          )}
          {!cartForbidden && (
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
          )}
        </nav>
      </div>

      <nav className="hidden border-t lg:block">
        <div className="mx-auto flex max-w-7xl items-center gap-1 px-4">
          {NAV_LINKS.map((link) => {
            const active = isNavLinkActive(link.to, location.pathname, location.search)
            return (
              <Link
                key={link.to}
                to={link.to}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "px-3 py-2 text-sm font-medium transition-colors hover:text-primary",
                  active ? "text-primary font-semibold" : "text-muted-foreground"
                )}
              >
                {link.label}
              </Link>
            )
          })}
        </div>
      </nav>

      {menuOpen && (
        <div ref={menuRef} className="border-t bg-background lg:hidden">
          <div className="flex items-center justify-between px-4 py-2">
            <span className="text-sm font-semibold">Danh mục</span>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Đóng menu"
              onClick={() => setMenuOpen(false)}
            >
              <X />
            </Button>
          </div>
          <div className="grid gap-1 px-2 pb-2">
            {NAV_LINKS.map((link) => {
              const active = isNavLinkActive(link.to, location.pathname, location.search)
              return (
                <Link
                  key={link.to}
                  to={link.to}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "rounded-lg px-3 py-2 text-sm font-medium hover:bg-muted hover:text-foreground",
                    active ? "bg-primary/10 text-primary font-semibold" : "text-muted-foreground"
                  )}
                  onClick={() => setMenuOpen(false)}
                >
                  {link.label}
                </Link>
              )
            })}
          </div>
          <div className="grid gap-1 border-t px-2 py-3">
            {isAuthenticated ? (
              <>
                <Link
                  to="/profile"
                  className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted"
                  onClick={() => setMenuOpen(false)}
                >
                  <User className="size-4 text-muted-foreground" />
                  <span>Hồ sơ cá nhân</span>
                </Link>
                {!favoritesForbidden && (
                  <Link
                    to="/yeu-thich"
                    className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted"
                    onClick={() => setMenuOpen(false)}
                  >
                    <Heart className="size-4 text-muted-foreground" />
                    <span>Sản phẩm yêu thích</span>
                  </Link>
                )}
                <button
                  type="button"
                  onClick={() => void handleLogout()}
                  className="flex items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium text-destructive hover:bg-destructive/10"
                >
                  <LogOut className="size-4" />
                  <span>Đăng xuất tài khoản</span>
                </button>
              </>
            ) : (
              <>
                <Link
                  to="/login"
                  className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted"
                  onClick={() => setMenuOpen(false)}
                >
                  <User className="size-4 text-muted-foreground" />
                  <span>Đăng nhập tài khoản</span>
                </Link>
                <Link
                  to="/register"
                  className="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground"
                  onClick={() => setMenuOpen(false)}
                >
                  <UserPlus className="size-4" />
                  <span>Tạo tài khoản mới</span>
                </Link>
              </>
            )}
          </div>
        </div>
      )}
    </header>
  )
}
