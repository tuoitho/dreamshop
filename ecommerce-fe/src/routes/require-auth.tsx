import { Navigate, useLocation } from "react-router-dom"
import { useAuth } from "@/hooks/use-auth"
import { isCustomerOnlyRoleSet } from "@/lib/auth-session"
import type { ReactNode } from "react"

export function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated, user, ready } = useAuth()
  const location = useLocation()

  if (!ready) {
    return (
      <div className="mx-auto flex max-w-md flex-col items-center justify-center gap-3 px-4 py-16 text-center" role="status">
        <span
          aria-hidden="true"
          className="size-7 animate-spin rounded-full border-2 border-primary border-t-transparent"
        />
        <p className="text-sm font-medium text-muted-foreground">Đang khôi phục phiên...</p>
      </div>
    )
  }

  const hasCustomerAccess = isAuthenticated && (user === null || isCustomerOnlyRoleSet(user.roles))
  if (!hasCustomerAccess) {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  return <>{children}</>
}
