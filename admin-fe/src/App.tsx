import type { ReactNode } from "react"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { Toaster } from "sonner"
import { AdminShell } from "@/components/layout/admin-shell"
import { AuthProvider } from "@/context/auth-provider"
import { useAuth } from "@/hooks/use-auth"
import { InventoryPage } from "@/pages/inventory-page"
import { LoginPage } from "@/pages/login-page"
import { OrdersPage } from "@/pages/orders-page"
import { OverviewPage } from "@/pages/overview-page"
import { ProductsPage } from "@/pages/products-page"
import { RbacPage } from "@/pages/rbac-page"
import { UsersPage } from "@/pages/users-page"
import { RequireAdminAuth } from "@/routes/require-admin-auth"

function PermissionGate({
  permission,
  children,
}: {
  permission: string
  children: ReactNode
}) {
  const { can } = useAuth()
  if (!can(permission)) {
    return (
      <div className="rounded-xl border border-border bg-card p-8 text-center">
        <h2 className="text-base font-semibold text-foreground">
          Bạn không có quyền truy cập phân hệ này
        </h2>
        <p className="mt-1.5 text-sm text-muted-foreground">
          Quyền yêu cầu: <span className="font-mono text-foreground">{permission}</span>
        </p>
      </div>
    )
  }
  return <>{children}</>
}

export function App() {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL}>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<RequireAdminAuth />}>
            <Route element={<AdminShell />}>
              <Route index element={<OverviewPage />} />
              <Route
                path="products"
                element={
                  <PermissionGate permission="product.view">
                    <ProductsPage />
                  </PermissionGate>
                }
              />
              <Route
                path="inventory"
                element={
                  <PermissionGate permission="inventory.view">
                    <InventoryPage />
                  </PermissionGate>
                }
              />
              <Route
                path="orders"
                element={
                  <PermissionGate permission="order.admin.view">
                    <OrdersPage />
                  </PermissionGate>
                }
              />
              <Route
                path="users"
                element={
                  <PermissionGate permission="user.view">
                    <UsersPage />
                  </PermissionGate>
                }
              />
              <Route
                path="rbac"
                element={
                  <PermissionGate permission="rbac.view">
                    <RbacPage />
                  </PermissionGate>
                }
              />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Route>
          </Route>
        </Routes>
        <Toaster position="bottom-right" richColors closeButton />
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App
