import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import { fetchProvinces, fetchWards } from "@/lib/locations"
import type { CartItem, CurrentUser } from "@/lib/types"
import { toast } from "sonner"
import { formatVND } from "@/lib/utils"
import OrderPreviewPage from "./order-preview-page"

vi.mock("sonner", () => ({ toast: { info: vi.fn() } }))
vi.mock("@/lib/locations", () => ({
  fetchProvinces: vi.fn().mockResolvedValue([{ code: "79", name: "TP. Hồ Chí Minh" }]),
  fetchWards: vi.fn().mockResolvedValue([{ code: "26740", name: "Phường Bến Nghé" }]),
}))

function expectCurrency(value: number) {
  const expected = formatVND(value).replace(/\u00a0/g, " ")
  expect(screen.getAllByText(/₫/).some((element) =>
    element.textContent?.replace(/\u00a0/g, " ").trim() === expected
  )).toBe(true)
}

const AVAILABLE_ITEM: CartItem = {
  itemId: "item-available",
  productId: "product-available",
  variantLabel: "Black",
  quantity: 2,
  name: "Available camera",
  brand: "Nova",
  categoryName: "Cameras",
  unitPrice: 100000,
  imageEmoji: "📷",
  imageHue: [220, 260],
  imageUrl: "",
  status: "AVAILABLE",
  availableQuantity: 4,
}

const LIMITED_ITEM: CartItem = {
  ...AVAILABLE_ITEM,
  itemId: "item-limited",
  productId: "product-limited",
  name: "Limited lens",
  quantity: 1,
  unitPrice: 50000,
  status: "LIMITED",
  availableQuantity: 1,
}

const OUT_OF_STOCK_ITEM: CartItem = {
  ...AVAILABLE_ITEM,
  itemId: "item-out-of-stock",
  productId: "product-out-of-stock",
  name: "Sold out tripod",
  status: "OUT_OF_STOCK",
  availableQuantity: 0,
}

function cartContextValue(items: CartItem[], selectedItems: CartItem[]): CartContextValue {
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
    totalQuantity: selectedItems.reduce((total, item) => total + item.quantity, 0),
    selectedTotal: selectedItems.reduce((total, item) => total + item.unitPrice * item.quantity, 0),
    allSelected: true,
    loading: false,
    degraded: false,
    loadError: null,
    retryLoad: vi.fn(),
  }
}

function renderPage(items: CartItem[], selectedItems: CartItem[]) {
  const value = cartContextValue(items, selectedItems)
  return {
    value,
    ...render(
      <MemoryRouter>
        <CartContext value={value}>
          <OrderPreviewPage />
        </CartContext>
      </MemoryRouter>
    ),
  }
}

describe("OrderPreviewPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("previews only selected purchasable items and updates totals when selection changes", () => {
    const allItems = [AVAILABLE_ITEM, LIMITED_ITEM, OUT_OF_STOCK_ITEM]
    const view = renderPage(allItems, [AVAILABLE_ITEM, OUT_OF_STOCK_ITEM])

    expect(screen.getByRole("heading", { name: "Xác nhận đơn hàng" })).toBeInTheDocument()
    expect(screen.getByText(AVAILABLE_ITEM.name)).toBeInTheDocument()
    expect(screen.queryByText(LIMITED_ITEM.name)).not.toBeInTheDocument()
    expect(screen.queryByText(OUT_OF_STOCK_ITEM.name)).not.toBeInTheDocument()
    expect(screen.getAllByText((content) => content.includes("200.000"))).toHaveLength(2)
    expectCurrency(25000)
    expectCurrency(225000)

    view.rerender(
      <MemoryRouter>
        <CartContext value={cartContextValue(allItems, [AVAILABLE_ITEM, LIMITED_ITEM])}>
          <OrderPreviewPage />
        </CartContext>
      </MemoryRouter>
    )

    expect(screen.getByText(LIMITED_ITEM.name)).toBeInTheDocument()
    expect(screen.queryByText(OUT_OF_STOCK_ITEM.name)).not.toBeInTheDocument()
    expect(screen.getByText((content) => content.includes("250.000"))).toBeInTheDocument()
    expect(screen.getByText((content) => content.includes("275.000"))).toBeInTheDocument()
  })

  it("shows an informational toast without clearing or mutating the cart", async () => {
    const user = userEvent.setup()
    const view = renderPage([AVAILABLE_ITEM], [AVAILABLE_ITEM])

    await user.click(screen.getByRole("button", { name: "Đặt hàng" }))

    expect(toast.info).toHaveBeenCalledWith("Chức năng đặt hàng chưa khả dụng")
    expect(view.baseElement).toHaveTextContent(AVAILABLE_ITEM.name)
    expect(view.baseElement).toHaveTextContent("225.000")
    expect(view.value.clearCart).not.toHaveBeenCalled()
    expect(view.value.removeItem).not.toHaveBeenCalled()
    expect(view.value.updateQuantity).not.toHaveBeenCalled()
    expect(view.value.addItem).not.toHaveBeenCalled()
  })

  it("provides a return link when no selected purchasable items remain", () => {
    renderPage([OUT_OF_STOCK_ITEM], [OUT_OF_STOCK_ITEM])

    expect(screen.getByText("Chưa có sản phẩm hợp lệ để đặt hàng")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Quay lại giỏ hàng" })).toHaveAttribute("href", "/gio-hang")
    expect(screen.queryByRole("button", { name: "Đặt hàng" })).not.toBeInTheDocument()
  })

  it("displays unit price × quantity, low-stock excess warning, and shipping recipient summary", async () => {
    const overStockLimited: CartItem = {
      ...LIMITED_ITEM,
      quantity: 3,
      availableQuantity: 1,
      imageUrl: "https://cdn.example.com/lens.jpg",
    }
    const incompleteUser: CurrentUser = {
      userId: "u-1",
      email: "buyer@example.com",
      fullName: "Trần Thị B",
      roles: ["USER"],
      phone: null,
      dateOfBirth: null,
      gender: null,
      provinceCode: "79",
      wardCode: "26740",
      addressLine: "45 Nguyễn Huệ",
    }
    const authVal: AuthContextValue = {
      token: "jwt",
      user: incompleteUser,
      ready: true,
      isAuthenticated: true,
      login: vi.fn(),
      register: vi.fn(),
      updateProfile: vi.fn(),
      logout: vi.fn(),
    }

    render(
      <MemoryRouter>
        <AuthContext value={authVal}>
          <CartContext value={cartContextValue([overStockLimited], [overStockLimited])}>
            <OrderPreviewPage />
          </CartContext>
        </AuthContext>
      </MemoryRouter>
    )

    expect(screen.getByText(/50\.000.*×\s*3/)).toBeInTheDocument()
    expect(screen.getByRole("img", { name: overStockLimited.name })).toHaveAttribute(
      "src",
      "https://cdn.example.com/lens.jpg"
    )
    expect(screen.getByText(/chỉ còn 1 sản phẩm/i)).toBeInTheDocument()
    expect(screen.getByText("Trần Thị B")).toBeInTheDocument()
    expect(await screen.findByText(/TP\. Hồ Chí Minh/)).toBeInTheDocument()
    expect(fetchProvinces).toHaveBeenCalled()
    expect(fetchWards).toHaveBeenCalledWith("79")
    expect(screen.getByRole("link", { name: /Cập nhật địa chỉ giao hàng/i })).toHaveAttribute(
      "href",
      "/profile"
    )
  })
})
