import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import type { CartItem } from "@/lib/types"
import { toast } from "sonner"
import OrderPreviewPage from "./order-preview-page"

vi.mock("sonner", () => ({ toast: { info: vi.fn() } }))

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

function cartContextValue(selectedItems: CartItem[]): CartContextValue {
  return {
    items: selectedItems,
    addItem: vi.fn(),
    updateQuantity: vi.fn(),
    removeItem: vi.fn(),
    removeSelected: vi.fn(),
    clearCart: vi.fn(),
    toggleSelected: vi.fn(),
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

function renderPage(selectedItems: CartItem[]) {
  return render(
    <MemoryRouter>
      <CartContext value={cartContextValue(selectedItems)}>
        <OrderPreviewPage />
      </CartContext>
    </MemoryRouter>
  )
}

describe("OrderPreviewPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("previews only selected purchasable items and computes totals", () => {
    renderPage([AVAILABLE_ITEM, LIMITED_ITEM, OUT_OF_STOCK_ITEM])

    expect(screen.getByRole("heading", { name: "Xác nhận đơn hàng" })).toBeInTheDocument()
    expect(screen.getByText(AVAILABLE_ITEM.name)).toBeInTheDocument()
    expect(screen.getByText(LIMITED_ITEM.name)).toBeInTheDocument()
    expect(screen.queryByText(OUT_OF_STOCK_ITEM.name)).not.toBeInTheDocument()
    expect(screen.getByText((content) => content.includes("250.000"))).toBeInTheDocument()
    expect(screen.getByText((content) => content.includes("25.000"))).toBeInTheDocument()
    expect(screen.getByText((content) => content.includes("275.000"))).toBeInTheDocument()
  })

  it("shows an informational toast instead of creating an order", async () => {
    const user = userEvent.setup()
    renderPage([AVAILABLE_ITEM])

    await user.click(screen.getByRole("button", { name: "Đặt hàng" }))

    expect(toast.info).toHaveBeenCalledWith("Chức năng đặt hàng chưa khả dụng")
  })

  it("provides a return link when no selected purchasable items remain", () => {
    renderPage([OUT_OF_STOCK_ITEM])

    expect(screen.getByText("Chưa có sản phẩm hợp lệ để đặt hàng")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Quay lại giỏ hàng" })).toHaveAttribute("href", "/gio-hang")
    expect(screen.queryByRole("button", { name: "Đặt hàng" })).not.toBeInTheDocument()
  })
})
