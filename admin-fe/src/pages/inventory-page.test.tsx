import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import * as api from "@/lib/api"
import type { AdminInventoryPage } from "@/lib/types"
import { InventoryPage } from "./inventory-page"

function makeAuth(permissions: string[]): AuthContextValue {
  const permSet = new Set(permissions)
  return {
    status: "authenticated",
    user: {
      userId: "u-1",
      email: "warehouse@sshop.vn",
      fullName: "Quản kho",
      roles: ["ADMIN"],
    },
    permissions: permSet,
    hasAnyAdminAccess: true,
    can: (perm: string) => permSet.has(perm),
    login: vi.fn(),
    logout: vi.fn(),
    refreshPermissions: vi.fn(),
  }
}

const sampleInventoryPage: AdminInventoryPage = {
  items: [
    {
      skuId: "p-iphone-v1",
      productId: "p-iphone",
      name: "iPhone 16 Pro",
      variantLabel: "Đen Titan · 256GB",
      variantColorHex: "#1c1c1e",
      brand: "Apple",
      categoryId: "dien-thoai",
      categoryName: "Điện thoại",
      price: 28990000,
      originalPrice: 30990000,
      imageUrl: "",
      emoji: "📱",
      imageHue: [215, 25],
      deleted: false,
      quantity: 9,
      reserved: 5,
      availableQuantity: 4,
      stockStatus: "LOW_STOCK",
      updatedAt: "2026-10-26T08:00:00Z",
    },
    {
      skuId: "p-archived-v1",
      productId: "p-archived",
      name: "Áo khoác Gió Cũ",
      variantLabel: "Xanh rêu · L",
      brand: "S-Wear",
      categoryId: "thoi-trang",
      categoryName: "Thời trang",
      price: 590000,
      originalPrice: 590000,
      imageUrl: "",
      emoji: "👕",
      imageHue: [140, 20],
      deleted: true,
      quantity: 0,
      reserved: 0,
      availableQuantity: 0,
      stockStatus: "OUT_OF_STOCK",
    },
  ],
  page: 1,
  pageSize: 15,
  totalItems: 2,
  totalPages: 1,
  summary: {
    totalTrackedProducts: 2,
    totalTrackedSkus: 2,
    inStockCount: 0,
    lowStockCount: 1,
    outOfStockCount: 1,
    totalQuantity: 9,
    totalReserved: 5,
    totalAvailable: 4,
  },
}

describe("InventoryPage & StockDialog", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("renders SKU stock rows with skuId, variantLabel, price, soft-deleted badge, and filters by Sắp hết", async () => {
    const listSpy = vi.spyOn(api, "listAdminInventory").mockResolvedValue(sampleInventoryPage)

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["inventory.view", "inventory.update"])}>
          <InventoryPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    expect(await screen.findByText("iPhone 16 Pro")).toBeInTheDocument()
    expect(screen.getByText("Đen Titan · 256GB")).toBeInTheDocument()
    expect(screen.getByText(/p-iphone-v1/)).toBeInTheDocument()
    expect(screen.getByText(/28\.990\.000/)).toBeInTheDocument()
    expect(screen.getByText("Áo khoác Gió Cũ")).toBeInTheDocument()
    expect(screen.getByText("Đã ẩn")).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /sắp hết/i }))
    await waitFor(() =>
      expect(listSpy).toHaveBeenCalledWith(
        expect.objectContaining({ stockStatus: "low_stock", page: 1 }),
        expect.anything()
      )
    )
  })

  it("surfaces inline 409 STOCK_BELOW_RESERVED conflict on setSkuStock and submits adjustSkuStock (+/-)", async () => {
    vi.spyOn(api, "listAdminInventory").mockResolvedValue(sampleInventoryPage)
    const setStockSpy = vi
      .spyOn(api, "setSkuStock")
      .mockRejectedValueOnce(
        new api.ApiClientError(
          409,
          "Mức tồn kho mới (3) không thể nhỏ hơn số lượng đang giữ chỗ (5).",
          "STOCK_BELOW_RESERVED"
        )
      )
    const adjustStockSpy = vi.spyOn(api, "adjustSkuStock").mockResolvedValue({
      skuId: "p-iphone-v1",
      productId: "p-iphone",
      quantity: 19,
      reserved: 5,
      availableQuantity: 14,
    })

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["inventory.view", "inventory.update"])}>
          <InventoryPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await screen.findByText("iPhone 16 Pro")
    await userEvent.click(
      screen.getByRole("button", { name: /điều chỉnh tồn iphone 16 pro/i })
    )

    const quantityInput = await screen.findByLabelText(/tổng tồn kho mới/i)
    await userEvent.clear(quantityInput)
    await userEvent.type(quantityInput, "3")
    await userEvent.click(screen.getByRole("button", { name: /xác nhận cập nhật/i }))

    await waitFor(() => expect(setStockSpy).toHaveBeenCalledWith("p-iphone-v1", "p-iphone", 3))
    expect(
      await screen.findByText(/không thể nhỏ hơn số lượng đang giữ chỗ/i)
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole("tab", { name: /điều chỉnh \+\/-/i }))
    const deltaInput = await screen.findByLabelText(/biến động số lượng/i)
    await userEvent.clear(deltaInput)
    await userEvent.type(deltaInput, "10")
    await userEvent.click(screen.getByRole("button", { name: /xác nhận cập nhật/i }))

    await waitFor(() =>
      expect(adjustStockSpy).toHaveBeenCalledWith("p-iphone-v1", "p-iphone", 10)
    )
  })
})
