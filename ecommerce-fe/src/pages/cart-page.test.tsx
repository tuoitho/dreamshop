import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "@/context/cart-context"
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

const LIMITED_ITEM: CartItem = {
  ...ITEM,
  itemId: "item-limited",
  productId: "product-limited",
  name: "Limited headphones",
  variantLabel: "Blue",
  quantity: 2,
  unitPrice: 50000,
  status: "LIMITED",
  availableQuantity: 1,
}

const OUT_OF_STOCK_ITEM: CartItem = {
  ...ITEM,
  itemId: "item-out-of-stock",
  productId: "product-out-of-stock",
  name: "Sold out keyboard",
  status: "OUT_OF_STOCK",
  availableQuantity: 0,
}

const UNAVAILABLE_ITEM: CartItem = {
  ...ITEM,
  itemId: "item-unavailable",
  productId: "product-unavailable",
  name: "Unavailable mouse",
  status: "UNAVAILABLE",
  availableQuantity: 0,
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

function cartContextValue(items: CartItem[], selectedItems: CartItem[] = items.filter((item) =>
  item.status === "AVAILABLE" || item.status === "LIMITED"
)): CartContextValue {
  return {
    items,
    addItem: vi.fn(),
    updateQuantity: vi.fn(),
    removeItem: vi.fn(),
    removeSelected: vi.fn(),
    clearCart: vi.fn(),
    toggleSelected: vi.fn(),
    toggleAllSelected: vi.fn(),
    selectedItems,
    selectedCount: selectedItems.reduce((total, item) => total + item.quantity, 0),
    totalQuantity: items.reduce((total, item) => total + item.quantity, 0),
    selectedTotal: selectedItems.reduce((total, item) => total + item.unitPrice * item.quantity, 0),
    allSelected: selectedItems.length > 0,
    loading: false,
    degraded: false,
    loadError: null,
    retryLoad: vi.fn(),
  }
}

function renderWithCart(items: CartItem[], selectedItems?: CartItem[]) {
  const value = cartContextValue(items, selectedItems)
  return {
    value,
    ...render(
      <MemoryRouter>
        <CartContext value={value}>
          <CartPage />
        </CartContext>
      </MemoryRouter>
    ),
  }
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

describe("CartPage availability and selection", () => {
  it("keeps available and limited products in the primary list", () => {
    renderWithCart([ITEM, LIMITED_ITEM, OUT_OF_STOCK_ITEM, UNAVAILABLE_ITEM])

    expect(screen.getByRole("heading", { name: "Sản phẩm có thể đặt" })).toBeInTheDocument()
    expect(screen.getByText(ITEM.name)).toBeInTheDocument()
    expect(screen.getByText(LIMITED_ITEM.name)).toBeInTheDocument()
    expect(screen.getByText("Sản phẩm không khả dụng")).toBeInTheDocument()
  })

  it("offers to reduce a limited item to its available quantity", async () => {
    const user = userEvent.setup()
    const { value } = renderWithCart([LIMITED_ITEM])

    expect(screen.getByText("Chỉ còn 1 sản phẩm")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Đặt về 1" }))

    expect(value.updateQuantity).toHaveBeenCalledWith(LIMITED_ITEM.itemId, 1)
  })

  it("disables out-of-stock selection and groups unavailable items with a remove action", () => {
    const { value } = renderWithCart([OUT_OF_STOCK_ITEM, UNAVAILABLE_ITEM], [])

    expect(screen.getByText("Hết hàng")).toBeInTheDocument()
    expect(screen.getByRole("checkbox", { name: `Chọn ${OUT_OF_STOCK_ITEM.name}` })).toBeDisabled()
    expect(screen.getByRole("heading", { name: "Sản phẩm không khả dụng" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: `Xóa ${UNAVAILABLE_ITEM.name}` })).toBeInTheDocument()
    expect(value.selectedCount).toBe(0)
  })

  it("disables checkout when no valid item is selected", () => {
    renderWithCart([ITEM], [])

    expect(screen.getByRole("button", { name: /Tiếp tục đặt hàng/ })).toBeDisabled()
  })

  it("links valid selected items to the order preview", () => {
    renderWithCart([ITEM])

    expect(screen.getByRole("link", { name: /Tiếp tục đặt hàng/ })).toHaveAttribute(
      "href",
      "/gio-hang/dat-hang"
    )
  })

  it("shows a degraded cart warning without replacing the loaded cart", () => {
    const value = cartContextValue([ITEM])
    value.degraded = true
    render(
      <MemoryRouter>
        <CartContext value={value}>
          <CartPage />
        </CartContext>
      </MemoryRouter>
    )

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Một số thông tin giỏ hàng chưa được cập nhật. Vui lòng thử lại trước khi đặt hàng."
    )
    expect(screen.getByRole("heading", { name: "Giỏ hàng" })).toBeInTheDocument()
  })
})
