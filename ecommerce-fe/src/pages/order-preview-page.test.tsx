import { beforeEach, describe, expect, it, vi } from "vitest"
import { act, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import { ApiClientError } from "@/lib/api"
import * as api from "@/lib/api"
import { fetchProvinces, fetchWards } from "@/lib/locations"
import type { CartItem, CurrentUser, OrderDetail, ShippingAddress } from "@/lib/types"
import { formatVND } from "@/lib/utils"
import OrderPreviewPage from "./order-preview-page"

vi.mock("sonner", () => ({ toast: { info: vi.fn(), error: vi.fn(), success: vi.fn() } }))
vi.mock("@/lib/locations", () => ({
  fetchProvinces: vi.fn().mockResolvedValue([{ code: "79", name: "TP. Hồ Chí Minh" }]),
  fetchWards: vi.fn().mockResolvedValue([{ code: "26740", name: "Phường Bến Nghé" }]),
}))
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    listShippingAddresses: vi.fn().mockResolvedValue([]),
    createShippingAddress: vi.fn(),
    createOrder: vi.fn(),
  }
})

function expectCurrency(value: number) {
  const expected = formatVND(value).replace(/\u00a0/g, " ")
  expect(
    screen
      .getAllByText(/₫/)
      .some((element) => element.textContent?.replace(/\u00a0/g, " ").trim() === expected)
  ).toBe(true)
}

const COMPLETE_USER: CurrentUser = {
  userId: "u-1",
  email: "buyer@example.com",
  fullName: "Nguyễn Văn A",
  roles: ["CUSTOMER"],
  phone: "0901234567",
  dateOfBirth: null,
  gender: null,
  provinceCode: "79",
  wardCode: "26740",
  addressLine: "1 Lê Duẩn",
}

const SAVED_ADDRESS: ShippingAddress = {
  id: "addr-1",
  recipientName: "Nguyễn Văn A",
  phone: "0901234567",
  provinceCode: "79",
  wardCode: "26740",
  addressLine: "1 Lê Duẩn",
  label: "Nhà riêng",
  isDefault: true,
  createdAt: "2026-11-05T10:00:00Z",
  updatedAt: "2026-11-05T10:00:00Z",
}

const AVAILABLE_ITEM: CartItem = {
  itemId: "item-available",
  productId: "product-available",
  skuId: "product-available-v1",
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
  productVersion: 5,
}

const LIMITED_ITEM: CartItem = {
  ...AVAILABLE_ITEM,
  itemId: "item-limited",
  productId: "product-limited",
  skuId: "product-limited-v1",
  name: "Limited lens",
  quantity: 1,
  unitPrice: 50000,
  status: "LIMITED",
  availableQuantity: 1,
  productVersion: 2,
}

const OUT_OF_STOCK_ITEM: CartItem = {
  ...AVAILABLE_ITEM,
  itemId: "item-out-of-stock",
  productId: "product-out-of-stock",
  skuId: "product-oos-v1",
  name: "Sold out tripod",
  status: "OUT_OF_STOCK",
  availableQuantity: 0,
}

function makeOrderDetail(overrides: Partial<OrderDetail> = {}): OrderDetail {
  return {
    orderId: "ord-999",
    orderCode: "ORD-20261105-XYZ999",
    userId: "u-1",
    status: "CONFIRMED",
    paymentMethod: "COD",
    paymentStatus: "UNPAID",
    shippingAddress: {
      recipientName: "Nguyễn Văn A",
      phone: "0901234567",
      provinceCode: "79",
      provinceName: "TP. Hồ Chí Minh",
      wardCode: "26740",
      wardName: "Phường Bến Nghé",
      addressLine: "1 Lê Duẩn",
    },
    items: [],
    subtotal: 200000,
    shippingFee: 25000,
    totalAmount: 225000,
    currency: "VND",
    createdAt: "2026-11-05T10:00:00Z",
    updatedAt: "2026-11-05T10:00:00Z",
    ...overrides,
  }
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
    retryLoad: vi.fn().mockResolvedValue(undefined),
  }
}

function LocationDisplay() {
  const loc = useLocation()
  return <div data-testid="location-probe">{loc.pathname + loc.search}</div>
}

function renderPage(
  items: CartItem[],
  selectedItems: CartItem[],
  user: CurrentUser | null = COMPLETE_USER
) {
  const value = cartContextValue(items, selectedItems)
  const authVal: AuthContextValue = {
    token: user ? "jwt" : null,
    user,
    ready: true,
    isAuthenticated: Boolean(user),
    login: vi.fn(),
    register: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn(),
  }
  return {
    value,
    authVal,
    ...render(
      <MemoryRouter initialEntries={["/gio-hang/dat-hang"]}>
        <AuthContext value={authVal}>
          <CartContext value={value}>
            <Routes>
              <Route path="/gio-hang/dat-hang" element={<OrderPreviewPage />} />
              <Route path="*" element={<LocationDisplay />} />
            </Routes>
          </CartContext>
        </AuthContext>
      </MemoryRouter>
    ),
  }
}

describe("OrderPreviewPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
    vi.mocked(api.listShippingAddresses).mockResolvedValue([SAVED_ADDRESS])
  })

  it("previews only selected purchasable items and updates totals when selection changes", async () => {
    const allItems = [AVAILABLE_ITEM, LIMITED_ITEM, OUT_OF_STOCK_ITEM]
    const view = renderPage(allItems, [AVAILABLE_ITEM, OUT_OF_STOCK_ITEM])

    expect(screen.getByRole("heading", { name: "Xác nhận đơn hàng" })).toBeInTheDocument()
    expect(screen.getByText(AVAILABLE_ITEM.name)).toBeInTheDocument()
    expect(screen.queryByText(LIMITED_ITEM.name)).not.toBeInTheDocument()
    expect(screen.queryByText(OUT_OF_STOCK_ITEM.name)).not.toBeInTheDocument()
    expectCurrency(25000)
    expectCurrency(225000)

    view.rerender(
      <MemoryRouter initialEntries={["/gio-hang/dat-hang"]}>
        <AuthContext value={view.authVal}>
          <CartContext value={cartContextValue(allItems, [AVAILABLE_ITEM, LIMITED_ITEM])}>
            <OrderPreviewPage />
          </CartContext>
        </AuthContext>
      </MemoryRouter>
    )

    expect(screen.getByText(LIMITED_ITEM.name)).toBeInTheDocument()
    expectCurrency(275000)
  })

  it("submits COD order with SKU snapshot and navigates to /don-hang/:orderId?placed=1 while refreshing cart", async () => {
    const user = userEvent.setup()
    vi.mocked(api.createOrder).mockResolvedValueOnce({
      order: makeOrderDetail({ orderId: "ord-cod-1", status: "CONFIRMED" }),
    })

    const view = renderPage([AVAILABLE_ITEM], [AVAILABLE_ITEM])
    await screen.findByText(/1 Lê Duẩn/)

    await user.click(screen.getByRole("button", { name: "Đặt hàng" }))

    await waitFor(() => {
      expect(api.createOrder).toHaveBeenCalledTimes(1)
    })
    const [sentPayload, sentKey] = vi.mocked(api.createOrder).mock.calls[0]
    expect(sentPayload.paymentMethod).toBe("COD")
    expect(sentPayload.items[0]).toMatchObject({
      productId: "product-available",
      skuId: "product-available-v1",
      quantity: 2,
      unitPrice: 100000,
      expectedName: "Available camera",
      variantLabel: "Black",
      expectedProductVersion: 5,
    })
    expect(typeof sentKey).toBe("string")
    expect(sentKey.length).toBeGreaterThan(10)
    expect(view.value.retryLoad).toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByTestId("location-probe")).toHaveTextContent("/don-hang/ord-cod-1?placed=1")
    })
  })

  it("retains Idempotency-Key on 409 REQUEST_IN_PROGRESS and auto-retries after 1.5s, but rotates key after terminal error", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

    vi.mocked(api.createOrder)
      .mockRejectedValueOnce(
        new ApiClientError(409, "Đơn hàng đang được xử lý", "REQUEST_IN_PROGRESS", "t-1")
      )
      .mockRejectedValueOnce(
        new ApiClientError(409, "Sản phẩm vừa hết hàng", "INVENTORY_INSUFFICIENT", "t-2")
      )
      .mockResolvedValueOnce({
        order: makeOrderDetail({ orderId: "ord-after-retry" }),
      })

    renderPage([AVAILABLE_ITEM], [AVAILABLE_ITEM])
    await screen.findByText(/1 Lê Duẩn/)

    await user.click(screen.getByRole("button", { name: "Đặt hàng" }))

    expect(await screen.findByText(/Đơn hàng đang được xử lý/i)).toBeInTheDocument()
    expect(api.createOrder).toHaveBeenCalledTimes(1)
    const firstKey = vi.mocked(api.createOrder).mock.calls[0][1]

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1600)
    })

    await waitFor(() => {
      expect(api.createOrder).toHaveBeenCalledTimes(2)
    })
    const secondKey = vi.mocked(api.createOrder).mock.calls[1][1]
    expect(secondKey).toBe(firstKey)

    // After terminal INVENTORY_INSUFFICIENT, clicking Đặt hàng again uses a rotated key
    await user.click(screen.getByRole("button", { name: "Đặt hàng" }))
    await waitFor(() => {
      expect(api.createOrder).toHaveBeenCalledTimes(3)
    })
    const thirdKey = vi.mocked(api.createOrder).mock.calls[2][1]
    expect(thirdKey).not.toBe(firstKey)
  })

  it("locks submit button and displays changed SKUs banner on 409 PRODUCT_SNAPSHOT_MISMATCH", async () => {
    const user = userEvent.setup()
    vi.mocked(api.createOrder).mockRejectedValueOnce(
      new ApiClientError(
        409,
        "Giá hoặc thông tin sản phẩm vừa thay đổi",
        "PRODUCT_SNAPSHOT_MISMATCH",
        "t-mismatch",
        {
          "product-available-v1": "[PRICE_CHANGED] Giá phân loại vừa thay đổi từ 100.000₫ sang 120.000₫",
        }
      )
    )

    renderPage([AVAILABLE_ITEM], [AVAILABLE_ITEM])
    await screen.findByText(/1 Lê Duẩn/)

    await user.click(screen.getByRole("button", { name: "Đặt hàng" }))

    expect(await screen.findByRole("alert")).toHaveTextContent(/Giá phân loại vừa thay đổi/)
    expect(screen.getByRole("button", { name: "Đặt hàng" })).toBeDisabled()
    expect(
      screen.getByRole("link", { name: /Quay lại giỏ hàng để cập nhật/i })
    ).toHaveAttribute("href", "/gio-hang")
  })

  it("provides a return link when no selected purchasable items remain", () => {
    renderPage([OUT_OF_STOCK_ITEM], [OUT_OF_STOCK_ITEM])

    expect(screen.getByText("Chưa có sản phẩm hợp lệ để đặt hàng")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Quay lại giỏ hàng" })).toHaveAttribute("href", "/gio-hang")
    expect(screen.queryByRole("button", { name: "Đặt hàng" })).not.toBeInTheDocument()
  })

  it("falls back to user profile when address book is empty and warns if phone is missing", async () => {
    vi.mocked(api.listShippingAddresses).mockResolvedValueOnce([])
    const incompleteUser: CurrentUser = {
      ...COMPLETE_USER,
      fullName: "Trần Thị B",
      phone: null,
      addressLine: "45 Nguyễn Huệ",
    }

    renderPage([LIMITED_ITEM], [LIMITED_ITEM], incompleteUser)

    expect(screen.getByText("Trần Thị B")).toBeInTheDocument()
    expect(await screen.findByText(/TP\. Hồ Chí Minh/)).toBeInTheDocument()
    expect(fetchProvinces).toHaveBeenCalled()
    expect(fetchWards).toHaveBeenCalledWith("79")
  })
})
