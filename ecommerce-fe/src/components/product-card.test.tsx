import { describe, expect, it, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { FavoritesContext, DEFAULT_FAVORITES_CONTEXT, type FavoritesContextValue } from "@/context/favorites-context"
import type { ProductSummary } from "@/lib/types"
import { ProductCard } from "./product-card"

const BASE_PRODUCT: ProductSummary = {
  id: "dt-1",
  name: "Điện thoại Nova X15",
  brand: "Nova",
  categoryId: "dien-thoai",
  categoryName: "Điện thoại",
  price: 18990000,
  originalPrice: 21990000,
  rating: 4.900000095367432,
  reviewCount: 1200,
  soldCount: 2300,
  emoji: "📱",
  imageHue: [220, 260],
}

function renderCard(overrides: Partial<ProductSummary> = {}) {
  return render(
    <MemoryRouter>
      <ProductCard product={{ ...BASE_PRODUCT, ...overrides }} />
    </MemoryRouter>
  )
}

describe("ProductCard", () => {
  it("formats float32 rating precision to one decimal place", () => {
    renderCard()

    expect(screen.getByText("4.9")).toBeInTheDocument()
    expect(screen.queryByText("4.900000095367432")).not.toBeInTheDocument()
  })

  it("displays 'Tạm hết hàng' badge while keeping product link navigable when OUT_OF_STOCK", () => {
    renderCard({ status: "OUT_OF_STOCK", availableQuantity: 0 })

    expect(screen.getByText("Tạm hết hàng")).toBeInTheDocument()
    const links = screen.getAllByRole("link", { name: BASE_PRODUCT.name })
    expect(links.length).toBeGreaterThan(0)
    expect(links[0]).toHaveAttribute("href", "/san-pham/dt-1")
  })

  it("displays 'Ngừng kinh doanh' badge when UNAVAILABLE", () => {
    renderCard({ status: "UNAVAILABLE", availableQuantity: 0 })

    expect(screen.getByText("Ngừng kinh doanh")).toBeInTheDocument()
  })

  it("displays low-stock note when status is LIMITED", () => {
    renderCard({ status: "LIMITED", availableQuantity: 2 })

    expect(screen.getByText("Chỉ còn 2")).toBeInTheDocument()
  })

  it("redirects unauthenticated clicks on favorite button to /login with return location", async () => {
    function LoginProbe() {
      const location = useLocation()
      return <output data-testid="login-from">{(location.state as { from?: string } | null)?.from ?? ""}</output>
    }

    render(
      <MemoryRouter initialEntries={["/tim-kiem?q=nova"]}>
        <Routes>
          <Route path="/tim-kiem" element={<ProductCard product={BASE_PRODUCT} />} />
          <Route path="/login" element={<LoginProbe />} />
        </Routes>
      </MemoryRouter>
    )

    const favButton = screen.getByRole("button", { name: `Thêm ${BASE_PRODUCT.name} vào yêu thích` })
    expect(favButton).toHaveAttribute("aria-pressed", "false")
    await userEvent.click(favButton)

    await waitFor(() => expect(screen.getByTestId("login-from")).toHaveTextContent("/tim-kiem?q=nova"))
  })

  it("toggles favorite for authenticated customers without navigating to product detail and hides when forbidden", async () => {
    const toggleFavorite = vi.fn().mockResolvedValue(undefined)
    const auth: AuthContextValue = {
      token: "token",
      user: {
        userId: "u-1",
        email: "c@example.com",
        fullName: "Customer",
        roles: ["CUSTOMER"],
        phone: null,
        dateOfBirth: null,
        gender: null,
        provinceCode: null,
        wardCode: null,
        addressLine: null,
      },
      ready: true,
      isAuthenticated: true,
      login: vi.fn(),
      register: vi.fn(),
      updateProfile: vi.fn(),
      logout: vi.fn(),
    }
    const favValue: FavoritesContextValue = {
      ...DEFAULT_FAVORITES_CONTEXT,
      isFavorite: (id) => id === BASE_PRODUCT.id,
      toggleFavorite,
    }

    const { unmount } = render(
      <AuthContext value={auth}>
        <FavoritesContext value={favValue}>
          <MemoryRouter initialEntries={["/"]}>
            <Routes>
              <Route path="/" element={<ProductCard product={BASE_PRODUCT} />} />
              <Route path="/san-pham/:id" element={<div data-testid="detail-page">Detail</div>} />
            </Routes>
          </MemoryRouter>
        </FavoritesContext>
      </AuthContext>
    )

    const favButton = screen.getByRole("button", { name: `Bỏ ${BASE_PRODUCT.name} khỏi yêu thích` })
    expect(favButton).toHaveAttribute("aria-pressed", "true")
    await userEvent.click(favButton)

    expect(toggleFavorite).toHaveBeenCalledWith(BASE_PRODUCT.id)
    expect(screen.queryByTestId("detail-page")).not.toBeInTheDocument()
    unmount()

    render(
      <AuthContext value={auth}>
        <FavoritesContext value={{ ...DEFAULT_FAVORITES_CONTEXT, forbidden: true }}>
          <MemoryRouter>
            <ProductCard product={BASE_PRODUCT} />
          </MemoryRouter>
        </FavoritesContext>
      </AuthContext>
    )
    expect(screen.queryByRole("button", { name: /yêu thích/i })).not.toBeInTheDocument()
  })
})
