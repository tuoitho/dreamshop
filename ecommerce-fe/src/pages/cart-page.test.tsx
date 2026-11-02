import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import { CartProvider } from "@/context/cart-provider"
import { ApiClientError, getCart } from "@/lib/api"
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
  skuId: "product-1-v1",
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
    selectOnly: vi.fn(),
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

  it("renders customer-only notice instead of load error when getCart returns 403 FORBIDDEN", async () => {
    vi.mocked(getCart).mockRejectedValueOnce(
      new ApiClientError(403, "You do not have permission to access or modify the cart.", "FORBIDDEN")
    )
    renderPage()

    expect(
      await screen.findByText("Tính năng giỏ hàng chỉ dành cho tài khoản khách hàng.")
    ).toBeInTheDocument()
    expect(screen.queryByRole("heading", { name: "Không tải được giỏ hàng" })).not.toBeInTheDocument()
    expect(screen.queryByText("Giỏ hàng đang trống")).not.toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Về trang chủ" })).toHaveAttribute("href", "/")
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

  it("centers item checkboxes vertically and aligns purchasable and unavailable sections on the same left axis", () => {
    renderWithCart([ITEM, UNAVAILABLE_ITEM])

    const purchasableCheckbox = screen.getByRole("checkbox", { name: `Chọn ${ITEM.name}` })
    const unavailableCheckbox = screen.getByRole("checkbox", { name: `Chọn ${UNAVAILABLE_ITEM.name}` })
    const purchasableLabel = purchasableCheckbox.closest("label")
    const unavailableLabel = unavailableCheckbox.closest("label")

    expect(purchasableLabel).toHaveClass("items-center")
    expect(purchasableLabel).not.toHaveClass("items-start", "pt-1")
    expect(purchasableLabel?.nextElementSibling).toHaveClass("self-center")
    expect(unavailableLabel).toHaveClass("items-center")
    expect(unavailableLabel).not.toHaveClass("items-start", "pt-1")
    expect(unavailableLabel?.nextElementSibling).toHaveClass("self-center")

    const purchasableSection = screen.getByRole("region", { name: "Sản phẩm có thể đặt" })
    const unavailableSection = screen.getByRole("region", { name: "Sản phẩm không khả dụng" })
    expect(purchasableLabel?.parentElement?.parentElement?.parentElement).toBe(purchasableSection)
    expect(unavailableLabel?.parentElement?.parentElement?.parentElement).toBe(unavailableSection)
  })

  it("offers to reduce a limited item to its available quantity", async () => {
    const user = userEvent.setup()
    const { value } = renderWithCart([LIMITED_ITEM])

    expect(screen.getByText("Chỉ còn 1 sản phẩm")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: "Đặt về 1" }))

    expect(value.updateQuantity).toHaveBeenCalledWith(LIMITED_ITEM.itemId, 1)
  })

  it("disables out-of-stock selection and removes unavailable items from the grouped section", async () => {
    const user = userEvent.setup()
    const { value } = renderWithCart([OUT_OF_STOCK_ITEM, UNAVAILABLE_ITEM], [])

    expect(screen.getByText("Hết hàng")).toBeInTheDocument()
    expect(screen.getByRole("checkbox", { name: `Chọn ${OUT_OF_STOCK_ITEM.name}` })).toBeDisabled()
    expect(screen.getByRole("heading", { name: "Sản phẩm không khả dụng" })).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: `Xóa ${UNAVAILABLE_ITEM.name}` }))

    expect(value.removeItem).toHaveBeenCalledWith(UNAVAILABLE_ITEM.itemId)
    expect(value.selectedCount).toBe(0)
  })

  it("gives quantity controls names that identify their product", () => {
    renderWithCart([ITEM])

    expect(screen.getByRole("button", { name: `Giảm số lượng ${ITEM.name}` })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: `Tăng số lượng ${ITEM.name}` })).toBeInTheDocument()
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

  it("shows the degraded warning above the empty-cart direction", () => {
    const value = cartContextValue([])
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
    expect(screen.getByRole("heading", { name: "Giỏ hàng đang trống" })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Tiếp tục mua sắm" })).toHaveAttribute("href", "/")
  })

  it("renders item.imageUrl when provided and disables 'Xóa đã chọn' when nothing is selected", () => {
    renderWithCart(
      [{ ...ITEM, imageUrl: "https://cdn.example.com/nova.jpg" }],
      []
    )

    const img = screen.getByRole("img", { name: ITEM.name })
    expect(img.tagName).toBe("IMG")
    expect(img).toHaveAttribute("src", "https://cdn.example.com/nova.jpg")
    expect(screen.getByRole("button", { name: "Xóa đã chọn" })).toBeDisabled()
  })

  it("renders orphan hard-deleted cart items with graceful fallback copy instead of empty name or 0 ₫", () => {
    const orphanItem: CartItem = {
      itemId: "item-orphan",
      productId: "deleted-forever",
      skuId: "deleted-forever-v1",
      variantLabel: "",
      quantity: 1,
      name: "",
      brand: "",
      categoryName: "",
      unitPrice: 0,
      imageEmoji: "",
      imageHue: [],
      imageUrl: "",
      status: "UNAVAILABLE",
      availableQuantity: 0,
    }
    renderWithCart([orphanItem], [])

    expect(screen.getByText("Sản phẩm không còn tồn tại")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Xóa Sản phẩm không còn tồn tại" })).toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Sản phẩm không còn tồn tại" })).not.toBeInTheDocument()
  })

  it("removes all unavailable items via 'Xóa tất cả' button in the unavailable section", async () => {
    const user = userEvent.setup()
    const { value } = renderWithCart([ITEM, OUT_OF_STOCK_ITEM, UNAVAILABLE_ITEM])

    await user.click(screen.getByRole("button", { name: /Xóa tất cả \(2\)/ }))

    expect(value.removeItem).toHaveBeenCalledTimes(2)
    expect(value.removeItem).toHaveBeenCalledWith(OUT_OF_STOCK_ITEM.itemId)
    expect(value.removeItem).toHaveBeenCalledWith(UNAVAILABLE_ITEM.itemId)
  })

  it("stops cleanly without unhandled rejection when 'Xóa tất cả' removeItem rejects", async () => {
    const user = userEvent.setup()
    const { value } = renderWithCart([ITEM, OUT_OF_STOCK_ITEM, UNAVAILABLE_ITEM])
    vi.mocked(value.removeItem).mockRejectedValueOnce(new Error("Gateway error"))

    await user.click(screen.getByRole("button", { name: /Xóa tất cả \(2\)/ }))

    expect(value.removeItem).toHaveBeenCalledTimes(1)
    expect(value.removeItem).toHaveBeenCalledWith(OUT_OF_STOCK_ITEM.itemId)
  })

  it("renders strikethrough originalPrice when discounted and caps quantity increase at SKU availableQuantity", () => {
    renderWithCart([
      {
        ...ITEM,
        quantity: 4,
        availableQuantity: 4,
        unitPrice: 100000,
        originalPrice: 135000,
      },
    ])

    const strikeEl = screen.getByText(/135\.000/)
    expect(strikeEl).toHaveClass("line-through")
    expect(screen.getByRole("button", { name: `Tăng số lượng ${ITEM.name}` })).toBeDisabled()
  })
})
