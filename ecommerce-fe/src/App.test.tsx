import { afterEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import type { CurrentUser } from "@/lib/types"
import App from "./App"

vi.mock("@/lib/locations", () => ({
  fetchProvinces: vi.fn().mockResolvedValue([]),
  fetchWards: vi.fn().mockResolvedValue([]),
}))

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    searchProducts: vi.fn().mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 20,
      totalItems: 0,
      totalPages: 0,
    }),
  }
})

const USER: CurrentUser = {
  userId: "user-1",
  email: "vana@example.com",
  fullName: "Nguyễn Văn A",
  roles: ["USER"],
  phone: "0901234567",
  dateOfBirth: null,
  gender: null,
  provinceCode: null,
  wardCode: null,
  addressLine: "123 Lê Lợi",
}

function makeCartValue(): CartContextValue {
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
  }
}

function renderAppAt(path: string, authOverrides: Partial<AuthContextValue> = {}) {
  window.history.pushState({}, "", path)
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
  return render(
    <AuthContext value={authValue}>
      <CartContext value={makeCartValue()}>
        <App />
      </CartContext>
    </AuthContext>
  )
}

describe("App StoreLayout shell", () => {
  afterEach(() => {
    window.history.pushState({}, "", "/")
  })

  it("wraps /profile inside StoreLayout with SiteHeader and SiteFooter", async () => {
    renderAppAt("/profile", {
      token: "access-token",
      isAuthenticated: true,
      user: USER,
    })

    expect(await screen.findByRole("heading", { name: "Hồ sơ" })).toBeInTheDocument()
    expect(screen.getByRole("searchbox", { name: "Tìm kiếm sản phẩm" })).toBeInTheDocument()
    expect(screen.getByText("Hỗ trợ khách hàng")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Trung tâm trợ giúp" })).toHaveAttribute(
      "href",
      "/tim-kiem"
    )
  })

  it("wraps /login inside StoreLayout with SiteHeader and SiteFooter", () => {
    renderAppAt("/login")

    expect(screen.getByRole("searchbox", { name: "Tìm kiếm sản phẩm" })).toBeInTheDocument()
    expect(screen.getByText("Đăng nhập vào tài khoản của bạn")).toBeInTheDocument()
    expect(screen.getByText("Hỗ trợ khách hàng")).toBeInTheDocument()
  })

  it("keeps SiteHeader and SiteFooter visible while RequireAuth restores session on /profile", () => {
    renderAppAt("/profile", { ready: false, isAuthenticated: false })

    expect(screen.getByRole("searchbox", { name: "Tìm kiếm sản phẩm" })).toBeInTheDocument()
    expect(screen.getByText("Đang khôi phục phiên...")).toBeInTheDocument()
    expect(screen.getByText("Hỗ trợ khách hàng")).toBeInTheDocument()
  })
})
