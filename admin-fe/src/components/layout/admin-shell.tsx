import {
  Boxes,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  LayoutDashboard,
  LogOut,
  Menu,
  Package,
  Plus,
  Search,
  ShieldCheck,
  Users,
  X,
} from "lucide-react"
import { useEffect, useMemo, useRef, useState, type ComponentType } from "react"
import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom"
import { BrandLogo } from "@/components/layout/brand-logo"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { cn } from "@/lib/utils"

interface NavItemConfig {
  to: string
  label: string
  keywords: string
  icon: ComponentType<{ className?: string }>
  permission?: string
  end?: boolean
}

interface NavGroupConfig {
  title: string
  items: NavItemConfig[]
}

const NAV_GROUPS: NavGroupConfig[] = [
  {
    title: "Tổng quan",
    items: [
      {
        to: "/",
        label: "Tổng quan",
        keywords: "dashboard tong quan kpi",
        icon: LayoutDashboard,
        end: true,
      },
    ],
  },
  {
    title: "Thương mại & Kho",
    items: [
      {
        to: "/orders",
        label: "Đơn hàng",
        keywords: "don hang order giao hang thanh toan",
        icon: ClipboardList,
        permission: "order.admin.view",
      },
      {
        to: "/products",
        label: "Sản phẩm",
        keywords: "san pham product catalog media",
        icon: Package,
        permission: "product.view",
      },
      {
        to: "/inventory",
        label: "Tồn kho",
        keywords: "ton kho inventory sku nhap hang",
        icon: Boxes,
        permission: "inventory.view",
      },
    ],
  },
  {
    title: "Hệ thống",
    items: [
      {
        to: "/users",
        label: "Người dùng",
        keywords: "nguoi dung user tai khoan khach hang",
        icon: Users,
        permission: "user.view",
      },
      {
        to: "/rbac",
        label: "Phân quyền RBAC",
        keywords: "phan quyen rbac role permission vai tro",
        icon: ShieldCheck,
        permission: "rbac.view",
      },
    ],
  },
]

const ROUTE_TITLES: Record<string, string> = {
  "/": "Tổng quan vận hành",
  "/orders": "Quản lý đơn hàng",
  "/products": "Danh mục sản phẩm & Media",
  "/inventory": "Kiểm kê & Điều phối tồn kho",
  "/users": "Quản trị tài khoản người dùng",
  "/rbac": "Cây phân quyền RBAC",
}

function getInitials(fullName?: string, email?: string): string {
  const source = (fullName || email || "AD").trim()
  const parts = source.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) {
    return `${parts[0][0] ?? ""}${parts[parts.length - 1][0] ?? ""}`.toUpperCase()
  }
  return source.slice(0, 2).toUpperCase()
}

export function AdminShell() {
  const { user, can, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(false)
  const [jumpQuery, setJumpQuery] = useState("")
  const [jumpOpen, setJumpOpen] = useState(false)
  const jumpContainerRef = useRef<HTMLDivElement>(null)
  const jumpInputRef = useRef<HTMLInputElement>(null)

  const currentTitle = ROUTE_TITLES[location.pathname] ?? "Trung tâm điều hành"
  const primaryRole = user?.roles[0] ?? "OPERATOR"

  const visibleGroups = useMemo(
    () =>
      NAV_GROUPS.map((group) => ({
        ...group,
        items: group.items.filter((item) => !item.permission || can(item.permission)),
      })).filter((group) => group.items.length > 0),
    [can]
  )

  const flatVisibleItems = useMemo(
    () => visibleGroups.flatMap((group) => group.items),
    [visibleGroups]
  )

  const filteredJumpItems = useMemo(() => {
    const normalized = jumpQuery.trim().toLowerCase()
    if (!normalized) return flatVisibleItems
    return flatVisibleItems.filter(
      (item) =>
        item.label.toLowerCase().includes(normalized) ||
        item.keywords.includes(normalized) ||
        item.to.toLowerCase().includes(normalized)
    )
  }, [flatVisibleItems, jumpQuery])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault()
        setJumpOpen(true)
        jumpInputRef.current?.focus()
      } else if (event.key === "Escape") {
        setJumpOpen(false)
      }
    }
    const onPointerDown = (event: MouseEvent) => {
      if (
        jumpContainerRef.current &&
        !jumpContainerRef.current.contains(event.target as Node)
      ) {
        setJumpOpen(false)
      }
    }
    window.addEventListener("keydown", onKeyDown)
    document.addEventListener("mousedown", onPointerDown)
    return () => {
      window.removeEventListener("keydown", onKeyDown)
      document.removeEventListener("mousedown", onPointerDown)
    }
  }, [])

  const handleLogout = async () => {
    await logout()
    navigate("/login", { replace: true })
  }

  const handleJumpSelect = (to: string) => {
    setJumpQuery("")
    setJumpOpen(false)
    navigate(to)
  }

  const quickAction = can("product.view")
    ? { to: "/products", label: "Sản phẩm" }
    : can("order.admin.view")
      ? { to: "/orders", label: "Đơn hàng" }
      : null

  return (
    <div className="flex min-h-screen w-full bg-background text-foreground">
      {mobileOpen && (
        <div
          aria-hidden="true"
          onClick={() => setMobileOpen(false)}
          className="fixed inset-0 z-30 bg-zinc-950/35 backdrop-blur-[1px] lg:hidden"
        />
      )}

      <aside
        aria-label="Điều hướng quản trị"
        data-collapsed={collapsed ? "true" : "false"}
        className={cn(
          "fixed inset-y-0 left-0 z-40 flex flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-all duration-200 lg:translate-x-0",
          collapsed ? "w-[256px] lg:w-[76px]" : "w-[256px]",
          mobileOpen ? "translate-x-0" : "-translate-x-full"
        )}
      >
        <div
          className={cn(
            "relative flex h-16 items-center border-b border-sidebar-border px-4",
            collapsed ? "justify-between lg:justify-center" : "justify-between"
          )}
        >
          <Link
            to="/"
            onClick={() => setMobileOpen(false)}
            className="flex min-w-0 items-center gap-3 focus-visible:outline-none"
          >
            <BrandLogo className="size-8" />
            <div className={cn("min-w-0", collapsed && "lg:hidden")}>
              <div className="truncate font-display text-sm font-bold tracking-tight text-foreground">
                DreamShop
              </div>
              <div className="font-mono text-[10px] font-medium tracking-[0.14em] text-sidebar-muted uppercase">
                Admin Console
              </div>
            </div>
          </Link>

          <button
            type="button"
            aria-label="Đóng menu"
            onClick={() => setMobileOpen(false)}
            className="rounded-lg p-1.5 text-sidebar-muted hover:bg-sidebar-hover hover:text-foreground lg:hidden"
          >
            <X className="size-4" />
          </button>

          <button
            type="button"
            aria-label={collapsed ? "Mở rộng menu" : "Thu gọn menu"}
            aria-expanded={!collapsed}
            onClick={() => setCollapsed((prev) => !prev)}
            className="absolute -right-3 top-1/2 hidden size-6 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-xs transition-colors hover:bg-muted hover:text-foreground lg:flex"
          >
            {collapsed ? <ChevronRight className="size-3.5" /> : <ChevronLeft className="size-3.5" />}
          </button>
        </div>

        <nav className="flex-1 space-y-5 overflow-y-auto px-3 py-5">
          {visibleGroups.map((group) => (
            <div key={group.title} className="space-y-1">
              <div
                className={cn(
                  "px-3 pb-1 text-[10px] font-semibold tracking-[0.12em] text-sidebar-muted uppercase",
                  collapsed && "lg:sr-only"
                )}
              >
                {group.title}
              </div>
              {group.items.map((item) => {
                const Icon = item.icon
                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={item.end}
                    title={collapsed ? item.label : undefined}
                    onClick={() => setMobileOpen(false)}
                    className={({ isActive }) =>
                      cn(
                        "group flex min-h-10 items-center gap-3 rounded-xl px-3 py-2 text-[13.5px] font-medium transition-all duration-150",
                        collapsed && "lg:justify-center lg:px-0",
                        isActive
                          ? "bg-sidebar-surface font-semibold text-foreground shadow-2xs ring-1 ring-border/70 [&_svg]:text-foreground"
                          : "text-sidebar-foreground hover:bg-sidebar-hover hover:text-foreground [&_svg]:text-sidebar-muted group-hover:[&_svg]:text-foreground"
                      )
                    }
                  >
                    <Icon className="size-[17px] shrink-0 transition-colors" />
                    <span className={cn("truncate", collapsed && "lg:sr-only")}>{item.label}</span>
                  </NavLink>
                )
              })}
            </div>
          ))}
        </nav>

        <div className="border-t border-sidebar-border p-3">
          {user && (
            <div
              className={cn(
                "flex items-center gap-2.5 rounded-xl bg-muted/55 p-2",
                collapsed && "lg:flex-col lg:gap-2 lg:bg-transparent lg:p-1"
              )}
            >
              <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary font-mono text-xs font-semibold text-primary-foreground">
                {getInitials(user.fullName, user.email)}
              </div>
              <div className={cn("min-w-0 flex-1", collapsed && "lg:hidden")}>
                <div className="truncate text-xs font-semibold text-foreground">
                  {user.fullName || user.email}
                </div>
                <div className="flex items-center gap-1.5">
                  <span className="truncate text-[11px] text-muted-foreground">{primaryRole}</span>
                </div>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={() => void handleLogout()}
                aria-label="Đăng xuất"
                title="Đăng xuất"
                className="size-8 shrink-0 text-muted-foreground hover:bg-card hover:text-destructive"
              >
                <LogOut className="size-3.5" />
              </Button>
            </div>
          )}
        </div>
      </aside>

      <div
        className={cn(
          "flex min-h-screen min-w-0 flex-1 flex-col transition-all duration-200",
          collapsed ? "lg:pl-[76px]" : "lg:pl-[256px]"
        )}
      >
        <header className="sticky top-0 z-20 flex h-16 items-center justify-between gap-4 border-b border-border/80 bg-card/90 px-4 backdrop-blur-md lg:px-8">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label="Mở menu điều hướng"
              onClick={() => setMobileOpen(true)}
              className="lg:hidden"
            >
              <Menu className="size-4" />
            </Button>

            <div ref={jumpContainerRef} className="relative w-full max-w-[300px]">
              <Search className="pointer-events-none absolute top-1/2 left-3 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <input
                ref={jumpInputRef}
                type="search"
                value={jumpQuery}
                onFocus={() => setJumpOpen(true)}
                onChange={(event) => {
                  setJumpQuery(event.target.value)
                  setJumpOpen(true)
                }}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && filteredJumpItems.length > 0) {
                    event.preventDefault()
                    handleJumpSelect(filteredJumpItems[0]!.to)
                  }
                }}
                placeholder="Tìm nhanh trang..."
                aria-label="Tìm nhanh trang"
                className="h-9 w-full rounded-xl border border-border bg-background/80 pr-12 pl-9 text-xs text-foreground placeholder:text-muted-foreground transition-colors outline-none focus:border-ring focus:bg-card"
              />
              <kbd className="pointer-events-none absolute top-1/2 right-2.5 hidden -translate-y-1/2 rounded border border-border bg-card px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground sm:inline-block">
                ⌘K
              </kbd>

              {jumpOpen && (
                <div
                  role="listbox"
                  aria-label="Kết quả chuyển trang nhanh"
                  className="absolute top-[calc(100%+6px)] right-0 left-0 z-50 overflow-hidden rounded-xl border border-border bg-card p-1.5 shadow-lg"
                >
                  {filteredJumpItems.length > 0 ? (
                    filteredJumpItems.map((item) => {
                      const Icon = item.icon
                      return (
                        <button
                          key={item.to}
                          type="button"
                          onClick={() => handleJumpSelect(item.to)}
                          className="flex w-full cursor-pointer items-center justify-between rounded-lg px-2.5 py-2 text-left text-xs font-medium text-foreground transition-colors hover:bg-muted"
                        >
                          <span className="flex items-center gap-2.5">
                            <Icon className="size-3.5 text-muted-foreground" />
                            <span>{item.label}</span>
                          </span>
                          <span className="font-mono text-[10px] text-muted-foreground">
                            {item.to}
                          </span>
                        </button>
                      )
                    })
                  ) : (
                    <div className="px-3 py-2 text-xs text-muted-foreground">
                      Không tìm thấy trang phù hợp.
                    </div>
                  )}
                </div>
              )}
            </div>

            <nav
              aria-label="Breadcrumb"
              className="hidden min-w-0 items-center gap-1.5 pl-2 text-xs xl:flex"
            >
              <span className="font-medium text-muted-foreground">DreamShop</span>
              <ChevronRight className="size-3.5 text-muted-foreground/70" />
              <span className="truncate font-semibold text-foreground">{currentTitle}</span>
            </nav>
          </div>

          <div className="flex shrink-0 items-center gap-2.5">
            <div className="hidden items-center gap-1.5 rounded-full border border-emerald-200/80 bg-emerald-50/70 px-2.5 py-1 text-[11px] font-medium text-emerald-700 md:flex">
              <span className="size-1.5 rounded-full bg-emerald-500" />
              <span>Gateway Live</span>
            </div>

            {quickAction && (
              <Button asChild size="sm">
                <Link to={quickAction.to}>
                  <Plus className="size-3.5" />
                  <span className="hidden sm:inline">{quickAction.label}</span>
                </Link>
              </Button>
            )}
          </div>
        </header>

        <main className="mx-auto w-full max-w-[1440px] flex-1 px-4 py-6 lg:px-8 lg:py-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
