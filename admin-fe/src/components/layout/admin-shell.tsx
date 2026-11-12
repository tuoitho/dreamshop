import {
  Boxes,
  ChevronRight,
  ClipboardList,
  LayoutDashboard,
  LogOut,
  Menu,
  Package,
  ShieldCheck,
  Users,
  X,
} from "lucide-react"
import { useState, type ComponentType } from "react"
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"
import { cn } from "@/lib/utils"

interface NavItemConfig {
  to: string
  label: string
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
    title: "Điều hành chung",
    items: [
      {
        to: "/",
        label: "Tổng quan",
        icon: LayoutDashboard,
        end: true,
      },
    ],
  },
  {
    title: "Thương mại & Kho vận",
    items: [
      {
        to: "/products",
        label: "Sản phẩm",
        icon: Package,
        permission: "product.view",
      },
      {
        to: "/inventory",
        label: "Tồn kho",
        icon: Boxes,
        permission: "inventory.view",
      },
      {
        to: "/orders",
        label: "Đơn hàng",
        icon: ClipboardList,
        permission: "order.admin.view",
      },
    ],
  },
  {
    title: "Định danh & Bảo mật",
    items: [
      {
        to: "/users",
        label: "Người dùng",
        icon: Users,
        permission: "user.view",
      },
      {
        to: "/rbac",
        label: "Phân quyền RBAC",
        icon: ShieldCheck,
        permission: "rbac.view",
      },
    ],
  },
]

const ROUTE_TITLES: Record<string, string> = {
  "/": "Tổng quan vận hành",
  "/products": "Danh mục sản phẩm & Media",
  "/inventory": "Kiểm kê & Điều phối tồn kho",
  "/orders": "Quản lý đơn hàng",
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

  const currentTitle = ROUTE_TITLES[location.pathname] ?? "Trung tâm điều hành"
  const primaryRole = user?.roles[0] ?? "OPERATOR"

  const visibleGroups = NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => !item.permission || can(item.permission)),
  })).filter((group) => group.items.length > 0)

  const handleLogout = async () => {
    await logout()
    navigate("/login", { replace: true })
  }

  return (
    <div className="flex min-h-screen w-full bg-background text-foreground">
      {mobileOpen && (
        <div
          aria-hidden="true"
          onClick={() => setMobileOpen(false)}
          className="fixed inset-0 z-30 bg-black/50 lg:hidden"
        />
      )}

      <aside
        aria-label="Điều hướng quản trị"
        className={cn(
          "fixed inset-y-0 left-0 z-40 flex w-[264px] flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-transform duration-200 lg:translate-x-0",
          mobileOpen ? "translate-x-0" : "-translate-x-full"
        )}
      >
        <div className="flex h-16 items-center justify-between border-b border-sidebar-border px-5">
          <div className="flex items-center gap-3">
            <div className="flex size-9 items-center justify-center rounded-lg border border-emerald-500/30 bg-emerald-500/15 font-mono text-sm font-semibold text-emerald-300">
              T
            </div>
            <div>
              <div className="text-sm font-semibold leading-tight text-white">TechShop Admin</div>
              <div className="font-mono text-[10px] font-medium tracking-widest text-emerald-400 uppercase">
                Control Center
              </div>
            </div>
          </div>
          <button
            type="button"
            aria-label="Đóng menu"
            onClick={() => setMobileOpen(false)}
            className="rounded-md p-1 text-sidebar-muted hover:bg-sidebar-hover hover:text-white lg:hidden"
          >
            <X className="size-4" />
          </button>
        </div>

        <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-5">
          {visibleGroups.map((group) => (
            <div key={group.title} className="space-y-1">
              <div className="px-3 pb-1.5 text-[11px] font-semibold tracking-wider text-sidebar-muted uppercase">
                {group.title}
              </div>
              {group.items.map((item) => {
                const Icon = item.icon
                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={item.end}
                    onClick={() => setMobileOpen(false)}
                    className={({ isActive }) =>
                      cn(
                        "flex min-h-10 items-center gap-3 rounded-lg border px-3 py-2 text-sm font-medium transition-colors",
                        isActive
                          ? "border-[#2a5244] bg-sidebar-surface font-semibold text-white [&_svg]:text-emerald-400"
                          : "border-transparent text-sidebar-foreground hover:bg-sidebar-hover hover:text-white [&_svg]:text-sidebar-muted"
                      )
                    }
                  >
                    <Icon className="size-4 shrink-0" />
                    <span>{item.label}</span>
                  </NavLink>
                )
              })}
            </div>
          ))}
        </nav>

        <div className="border-t border-sidebar-border bg-black/15 px-5 py-4">
          <div className="flex items-center gap-2 font-mono text-xs text-sidebar-foreground">
            <span className="size-2 rounded-full bg-emerald-400" />
            <span>Gateway Connected</span>
          </div>
          <div className="mt-1 text-[11px] text-sidebar-muted">
            RBAC · Catalog · Inventory · MinIO S3
          </div>
        </div>
      </aside>

      <div className="flex min-h-screen min-w-0 flex-1 flex-col lg:pl-[264px]">
        <header className="sticky top-0 z-20 flex h-16 items-center justify-between gap-4 border-b border-border bg-card px-4 lg:px-8">
          <div className="flex min-w-0 items-center gap-3">
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
            <nav aria-label="Breadcrumb" className="flex min-w-0 items-center gap-1.5 text-sm">
              <span className="hidden font-medium text-muted-foreground sm:inline">TechShop Admin</span>
              <ChevronRight className="hidden size-3.5 text-muted-foreground sm:inline" />
              <span className="truncate font-semibold text-foreground">{currentTitle}</span>
            </nav>
          </div>

          <div className="flex shrink-0 items-center gap-2.5">
            {user && (
              <div className="flex items-center gap-2.5 rounded-full border border-border bg-background py-1 pr-3 pl-1.5">
                <div className="flex size-7 items-center justify-center rounded-full bg-sidebar font-mono text-xs font-semibold text-white">
                  {getInitials(user.fullName, user.email)}
                </div>
                <div className="hidden text-left md:block">
                  <div className="max-w-[160px] truncate text-xs font-semibold text-foreground">
                    {user.fullName || user.email}
                  </div>
                  <div className="max-w-[160px] truncate font-mono text-[11px] text-muted-foreground">
                    {user.email}
                  </div>
                </div>
                <span className="rounded border border-emerald-200 bg-emerald-50 px-2 py-0.5 font-mono text-[11px] font-semibold tracking-wider text-emerald-800 uppercase">
                  {primaryRole}
                </span>
              </div>
            )}

            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => void handleLogout()}
              aria-label="Đăng xuất"
            >
              <LogOut className="size-3.5" />
              <span className="hidden sm:inline">Đăng xuất</span>
            </Button>
          </div>
        </header>

        <main className="mx-auto w-full max-w-[1520px] flex-1 px-4 py-6 lg:px-10 lg:py-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
