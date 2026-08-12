import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartProvider } from "@/context/cart-provider"
import { getCart } from "@/lib/api"
import type { CartData, CartItem, CurrentUser } from "@/lib/types"
import CartPage from "./cart-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return { ...actual, getCart: vi.fn() }
})

const USER: CurrentUser = {
  userId: "user-1",
  email: "user@example.com",
  fullName: "Test User",
  roles: ["USER"],
  phone: null,
  dateOfBirth: null,
  gender: null,
  provinceCode: null,
  wardCode: null,
  addressLine: null,
}

const ITEM: CartItem = {
  itemId: "item-1",
  productId: "product-1",
  variantLabel: "Red",
  quantity: 1,
  name: "Nova X15 Pro",
  brand: "Nova",
  categoryName: "Phones",
  unitPrice: 100000,
  imageEmoji: "📱",
  imageHue: [220, 260],
  imageUrl: "",
  status: "AVAILABLE",
  availableQuantity: 4,
}

function authValue(): AuthContextValue {
  return {
    token: "token",
    user: USER,
    ready: true,
    isAuthenticated: true,
    login: vi.fn(),
    register: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn(),
  }
}

function cart(items: CartItem[]): CartData {
  return { items, degraded: false }
}

function renderPage() {
  return render(
    <MemoryRouter>
      <AuthContext value={authValue()}>
        <CartProvider>
          <CartPage />
        </CartProvider>
      </AuthContext>
    </MemoryRouter>
  )
}

describe("CartPage load failure", () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  it("renders an actionable accessible error instead of the empty-cart success state", async () => {
    vi.mocked(getCart)
      .mockRejectedValueOnce(new Error("Cart unavailable"))
      .mockResolvedValueOnce(cart([ITEM]))
    renderPage()

    expect(await screen.findByRole("alert")).toHaveTextContent("Cart unavailable")
    expect(screen.queryByText("Giỏ hàng đang trống")).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: "Thử lại" }))
    expect(await screen.findByRole("heading", { name: "Giỏ hàng" })).toBeInTheDocument()
  })
})
