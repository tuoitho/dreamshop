import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { RequireAuth } from "@/routes/require-auth"
import { StoreLayout } from "@/components/layout/store-layout"
import LoginPage from "@/pages/login-page"
import RegisterPage from "@/pages/register-page"
import ProfilePage from "@/pages/profile-page"
import HomePage from "@/pages/home-page"
import SearchPage from "@/pages/search-page"
import ProductDetailPage from "@/pages/product-detail-page"
import CartPage from "@/pages/cart-page"
import FavoritesPage from "@/pages/favorites-page"
import OrderPreviewPage from "@/pages/order-preview-page"
import PaymentResultPage from "@/pages/payment-result-page"
import OrdersPage from "@/pages/orders-page"
import OrderDetailPage from "@/pages/order-detail-page"

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<StoreLayout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/tim-kiem" element={<SearchPage />} />
          <Route path="/san-pham/:id" element={<ProductDetailPage />} />
          <Route path="/yeu-thich" element={<RequireAuth><FavoritesPage /></RequireAuth>} />
          <Route path="/gio-hang" element={<RequireAuth><CartPage /></RequireAuth>} />
          <Route path="/gio-hang/dat-hang" element={<RequireAuth><OrderPreviewPage /></RequireAuth>} />
          <Route path="/thanh-toan/ket-qua" element={<RequireAuth><PaymentResultPage /></RequireAuth>} />
          <Route path="/don-hang" element={<RequireAuth><OrdersPage /></RequireAuth>} />
          <Route path="/don-hang/:orderId" element={<RequireAuth><OrderDetailPage /></RequireAuth>} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route
            path="/profile"
            element={
              <RequireAuth>
                <ProfilePage />
              </RequireAuth>
            }
          />
        </Route>
        <Route path="/dashboard" element={<Navigate to="/profile" replace />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
