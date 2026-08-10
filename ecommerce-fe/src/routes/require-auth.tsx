import { Navigate, useLocation } from "react-router-dom"
import { useAuth } from "@/hooks/use-auth"
import type { ReactNode } from "react"

export function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated, ready } = useAuth()
  const location = useLocation()

  if (!ready) {
    return <p className="p-4">Đang khôi phục phiên...</p>
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  return <>{children}</>
}
