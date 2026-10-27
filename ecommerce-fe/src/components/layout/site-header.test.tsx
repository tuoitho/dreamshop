import { describe, expect, it, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation, type InitialEntry } from "react-router-dom"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import { FavoritesContext, type FavoritesContextValue } from "@/context/favorites-context"
import type { CurrentUser } from "@/lib/types"
import { SiteHeader } from "./site-header"

const USER: CurrentUser = {
  userId: "user-1",
  email: "vana@example.com",
  fullName: "Nguyễn Văn A",
  roles: ["USER"],
  phone: null,
  dateOfBirth: null,
  gender: null,
  provinceCode: null,
  wardCode: null,
  addressLine: null,
}

function makeCartValue(overrides: Partial<CartContextValue> = {}): CartContextValue {
  return {
    items: [],
    addItem: vi.fn().mockResolvedValue(undefined),
    updateQuantity: vi.fn().mockResolvedValue(undefined),
    removeItem: vi.fn().mockResolvedValue(undefined),
    removeSelected: vi.fn().mockResolvedValue(undefined),
    clearCart: vi.fn().mockResolvedValue(undefined),
    toggleSelected: vi.fn(),
    selectOnly: vi.fn(),
    toggleAllSelected: vi.fn(),
    selectedItems: [],
    selectedCount: 0,
    totalQuantity: 0,
    selectedTotal: 0,
    allSelected: false,
    loading: false,
    degraded: false,
    loadError: null,
    retryLoad: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function LocationProbe() {
  const location = useLocation()
  return <p data-testid="current-location">{location.pathname}</p>
}

function renderHeader(
  authOverrides: Partial<AuthContextValue> = {},
  initialEntries: InitialEntry[] = ["/"],
  favoritesOverrides: Partial<FavoritesContextValue> = {},
  cartOverrides: Partial<CartContextValue> = {},
) {
  const authValue: AuthContextValue = {
    token: null,
    user: null,
    ready: true,
    isAuthenticated: false,
    login: vi.fn().mockResolvedValue(undefined),
    register: vi.fn().mockResolvedValue(undefined),
    updateProfile: vi.fn().mockResolvedValue(undefined),
    logout: vi.fn().mockResolvedValue(undefined),
    ...authOverrides,
  }

  const favoritesValue: FavoritesContextValue = {
    items: [],
    totalCount: 0,
    isFavorite: () => false,
    isPending: () => false,
    addFavorite: vi.fn().mockResolvedValue(undefined),
    removeFavorite: vi.fn().mockResolvedValue(undefined),
    toggleFavorite: vi.fn().mockResolvedValue(undefined),
    clearFavorites: vi.fn().mockResolvedValue(undefined),
    loading: false,
    degraded: false,
    forbidden: false,
    loadError: null,
    retryLoad: vi.fn().mockResolvedValue(undefined),
    ...favoritesOverrides,
  }

  return {
    authValue,
    favoritesValue,
    ...render(
      <AuthContext value={authValue}>
        <CartContext value={makeCartValue(cartOverrides)}>
          <FavoritesContext value={favoritesValue}>
            <MemoryRouter initialEntries={initialEntries}>
              <SiteHeader />
              <Routes>
                <Route path="*" element={<LocationProbe />} />
              </Routes>
            </MemoryRouter>
          </FavoritesContext>
        </CartContext>
      </AuthContext>
    ),
  }
}

describe("SiteHeader", () => {
  it("renders login link when unauthenticated", () => {
    renderHeader({ isAuthenticated: false, user: null })

    const loginLink = screen.getByRole("link", { name: "Đăng nhập" })
    expect(loginLink).toHaveAttribute("href", "/login")
    expect(loginLink.querySelector("svg")).not.toBeNull()
    expect(screen.queryByText("Tài khoản")).not.toBeInTheDocument()
  })

  it("renders user icon and full name instead of 'Tài khoản' when authenticated", () => {
    renderHeader({ token: "access-token", isAuthenticated: true, user: USER })

    const trigger = screen.getByRole("button", { name: "Nguyễn Văn A" })
    expect(trigger).toBeInTheDocument()
    expect(trigger).toHaveTextContent("Nguyễn Văn A")
    expect(trigger.querySelector("svg")).not.toBeNull()
    expect(screen.queryByText("Tài khoản")).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Đăng nhập" })).not.toBeInTheDocument()
  })

  it("falls back to user email when fullName is blank", () => {
    renderHeader({
      token: "access-token",
      isAuthenticated: true,
      user: { ...USER, fullName: "   " },
    })

    const trigger = screen.getByRole("button", { name: "vana@example.com" })
    expect(trigger).toHaveTextContent("vana@example.com")
  })

  it("opens dropdown menu and navigates to /profile when selecting 'Hồ sơ'", async () => {
    renderHeader({ token: "access-token", isAuthenticated: true, user: USER })

    await userEvent.click(screen.getByRole("button", { name: "Nguyễn Văn A" }))

    const profileItem = await screen.findByRole("menuitem", { name: "Hồ sơ" })
    expect(screen.getByRole("menuitem", { name: "Đăng xuất" })).toBeInTheDocument()

    await userEvent.click(profileItem)

    await waitFor(() => expect(screen.getByTestId("current-location")).toHaveTextContent("/profile"))
  })

  it("calls logout and navigates to home when selecting 'Đăng xuất'", async () => {
    const logoutMock = vi.fn().mockResolvedValue(undefined)
    renderHeader(
      { token: "access-token", isAuthenticated: true, user: USER, logout: logoutMock },
      ["/gio-hang"],
    )

    await userEvent.click(screen.getByRole("button", { name: "Nguyễn Văn A" }))
    await userEvent.click(await screen.findByRole("menuitem", { name: "Đăng xuất" }))

    expect(logoutMock).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.getByTestId("current-location")).toHaveTextContent("/"))
  })

  it("still navigates to home without unhandled rejection when logout request fails", async () => {
    const logoutMock = vi.fn().mockRejectedValue(new Error("Network Error"))
    renderHeader(
      { token: "access-token", isAuthenticated: true, user: USER, logout: logoutMock },
      ["/gio-hang"],
    )

    await userEvent.click(screen.getByRole("button", { name: "Nguyễn Văn A" }))
    await userEvent.click(await screen.findByRole("menuitem", { name: "Đăng xuất" }))

    expect(logoutMock).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.getByTestId("current-location")).toHaveTextContent("/"))
  })

  it("supports keyboard navigation to open and dismiss the user menu", async () => {
    renderHeader({ token: "access-token", isAuthenticated: true, user: USER })

    const trigger = screen.getByRole("button", { name: "Nguyễn Văn A" })
    trigger.focus()
    await userEvent.keyboard("{Enter}")

    expect(await screen.findByRole("menuitem", { name: "Hồ sơ" })).toBeInTheDocument()
    await userEvent.keyboard("{Escape}")
    expect(screen.queryByRole("menuitem", { name: "Hồ sơ" })).not.toBeInTheDocument()
  })

  it("keeps account controls visible on mobile viewports without hidden class", () => {
    const { unmount } = renderHeader({ isAuthenticated: false, user: null })
    expect(screen.getByRole("link", { name: "Đăng nhập" })).not.toHaveClass("hidden")
    unmount()

    renderHeader({ token: "access-token", isAuthenticated: true, user: USER })
    expect(screen.getByRole("button", { name: "Nguyễn Văn A" })).not.toHaveClass("hidden")
  })

  it("toggles mobile drawer open and closed cleanly on repeated clicks and exposes mobile account actions", async () => {
    renderHeader({ isAuthenticated: false, user: null })

    const menuButton = screen.getByRole("button", { name: "Mở menu" })
    await userEvent.click(menuButton)

    expect(screen.getByText("Danh mục")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Đăng nhập tài khoản" })).toHaveAttribute("href", "/login")
    expect(screen.getByRole("link", { name: "Tạo tài khoản mới" })).toHaveAttribute("href", "/register")

    await userEvent.click(menuButton)
    expect(screen.queryByText("Danh mục")).not.toBeInTheDocument()
  })

  it("syncs search input with URL ?q= and marks active category link with aria-current", () => {
    renderHeader(
      { isAuthenticated: false, user: null },
      ["/tim-kiem?danh-muc=dien-thoai&q=iPhone%2015"],
    )

    expect(screen.getByRole("searchbox", { name: "Tìm kiếm sản phẩm" })).toHaveValue("iPhone 15")
    expect(screen.getByRole("link", { name: "Điện thoại" })).toHaveAttribute("aria-current", "page")
    expect(screen.getByRole("link", { name: "Laptop" })).not.toHaveAttribute("aria-current")
  })

  it("renders favorites icon badge and menu links for customers and hides them when forbidden", async () => {
    const { unmount } = renderHeader(
      { token: "access-token", isAuthenticated: true, user: USER },
      ["/"],
      { totalCount: 120, forbidden: false },
      { totalQuantity: 3, forbidden: false },
    )

    const headerFavLink = screen.getByRole("link", { name: "Sản phẩm yêu thích" })
    expect(headerFavLink).toHaveAttribute("href", "/yeu-thich")
    expect(headerFavLink).toHaveTextContent("99+")

    const headerCartLink = screen.getByRole("link", { name: "Giỏ hàng" })
    expect(headerCartLink).toHaveAttribute("href", "/gio-hang")
    expect(headerCartLink).toHaveTextContent("3")

    await userEvent.click(screen.getByRole("button", { name: "Nguyễn Văn A" }))
    expect(await screen.findByRole("menuitem", { name: "Sản phẩm yêu thích" })).toHaveAttribute("href", "/yeu-thich")
    await userEvent.keyboard("{Escape}")

    await userEvent.click(screen.getByRole("button", { name: "Mở menu" }))
    const favLinks = screen.getAllByRole("link", { name: /Sản phẩm yêu thích/ })
    expect(favLinks.length).toBeGreaterThanOrEqual(2)
    unmount()

    renderHeader(
      { token: "admin-token", isAuthenticated: true, user: { ...USER, roles: ["ADMIN"] } },
      ["/"],
      { totalCount: 0, forbidden: true },
      { totalQuantity: 0, forbidden: true },
    )
    expect(screen.queryByRole("link", { name: "Sản phẩm yêu thích" })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Giỏ hàng" })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: "Nguyễn Văn A" }))
    expect(screen.queryByRole("menuitem", { name: "Sản phẩm yêu thích" })).not.toBeInTheDocument()
  })
})
