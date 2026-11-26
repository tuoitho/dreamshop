import { LogOut, ShieldAlert } from "lucide-react"
import { Navigate, Outlet, useLocation, useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/hooks/use-auth"

export function RequireAdminAuth() {
  const { status, user, hasAnyAdminAccess, logout } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background px-4">
        <div className="flex items-center gap-3 rounded-xl border border-border bg-card px-5 py-4 shadow-xs">
          <span className="size-2.5 animate-pulse rounded-full bg-emerald-500" />
          <span className="text-sm font-medium text-muted-foreground">
            Đang khôi phục phiên điều hành và quyền truy cập…
          </span>
        </div>
      </div>
    )
  }

  if (status === "unauthenticated" || !user) {
    return (
      <Navigate
        to="/login"
        replace
        state={{ from: `${location.pathname}${location.search}` }}
      />
    )
  }

  if (!hasAnyAdminAccess) {
    const handleSwitchAccount = async () => {
      await logout()
      navigate("/login", { replace: true })
    }

    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-6">
        <div className="w-full max-w-md rounded-xl border border-border bg-card p-7 shadow-sm">
          <div className="mb-4 flex size-11 items-center justify-center rounded-lg border border-amber-200 bg-amber-50 text-amber-800">
            <ShieldAlert className="size-5" />
          </div>
          <h1 className="text-lg font-semibold tracking-tight text-foreground">
            Tài khoản chưa được cấp quyền điều hành
          </h1>
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
            Tài khoản <span className="font-mono font-medium text-foreground">{user.email}</span> hiện
            không sở hữu quyền truy cập trung tâm quản trị DreamShop. Vui lòng liên hệ quản trị viên cấp
            cao hoặc chuyển sang tài khoản vận hành khác.
          </p>
          {user.roles.length > 0 && (
            <div className="mt-4 flex flex-wrap items-center gap-1.5 rounded-lg border border-border bg-muted/50 px-3 py-2 text-xs text-muted-foreground">
              <span>Vai trò hiện tại:</span>
              {user.roles.map((role) => (
                <span
                  key={role}
                  className="rounded border border-border bg-card px-1.5 py-0.5 font-mono text-[11px] font-medium text-foreground"
                >
                  {role}
                </span>
              ))}
            </div>
          )}
          <div className="mt-6 flex justify-end">
            <Button type="button" variant="default" onClick={() => void handleSwitchAccount()}>
              <LogOut className="size-4" />
              Đăng xuất / Đổi tài khoản
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return <Outlet />
}
