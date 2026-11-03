import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import * as api from "@/lib/api"
import { OverviewPage } from "./overview-page"

function makeAuth(permissions: string[]): AuthContextValue {
  const permSet = new Set(permissions)
  return {
    status: "authenticated",
    user: {
      userId: "u-1",
      email: "ops@sshop.vn",
      fullName: "Điều hành viên",
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

describe("OverviewPage", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("renders permitted KPI cards, deduplicates low-stock watchlist by skuId across variants of the same product, and opens StockDialog for that SKU", async () => {
    vi.spyOn(api, "listAdminProducts").mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 1,
      totalItems: 48,
      totalPages: 48,
      activeCount: 42,
      deletedCount: 6,
      categories: [
        { categoryId: "dien-thoai", categoryName: "Điện thoại", activeProductCount: 18 },
        { categoryId: "tai-nghe", categoryName: "Tai nghe", activeProductCount: 12 },
      ],
      stockDegraded: false,
    })

    const invSpy = vi.spyOn(api, "listAdminInventory").mockImplementation(async (params) => ({
      items:
        params?.stockStatus === "out_of_stock"
          ? [
              {
                skuId: "p-sony-v1",
                productId: "p-sony",
                name: "Tai nghe Sony WH-1000XM5",
                variantLabel: "Đen Khói",
                variantColorHex: "#1c1c1e",
                brand: "Sony",
                categoryId: "tai-nghe",
                categoryName: "Tai nghe",
                price: 6990000,
                originalPrice: 7990000,
                imageUrl: "",
                emoji: "🎧",
                imageHue: [160, 40],
                deleted: false,
                quantity: 2,
                reserved: 2,
                availableQuantity: 0,
                stockStatus: "OUT_OF_STOCK",
              },
            ]
          : [
              {
                skuId: "p-sony-v2",
                productId: "p-sony",
                name: "Tai nghe Sony WH-1000XM5",
                variantLabel: "Bạc Platinum",
                variantColorHex: "#cbd5e1",
                brand: "Sony",
                categoryId: "tai-nghe",
                categoryName: "Tai nghe",
                price: 7190000,
                originalPrice: 7990000,
                imageUrl: "",
                emoji: "🎧",
                imageHue: [160, 40],
                deleted: false,
                quantity: 6,
                reserved: 3,
                availableQuantity: 3,
                stockStatus: "LOW_STOCK",
              },
            ],
      page: 1,
      pageSize: 8,
      totalItems: 1,
      totalPages: 1,
      summary: {
        totalTrackedProducts: 42,
        totalTrackedSkus: 64,
        inStockCount: 57,
        lowStockCount: 5,
        outOfStockCount: 2,
        totalQuantity: 820,
        totalReserved: 40,
        totalAvailable: 780,
      },
    }))

    const setStockSpy = vi.spyOn(api, "setSkuStock").mockResolvedValue({
      skuId: "p-sony-v2",
      productId: "p-sony",
      quantity: 12,
      reserved: 3,
      availableQuantity: 9,
    })

    vi.spyOn(api, "listAdminUsers").mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 1,
      totalItems: 125,
      totalPages: 125,
      activeCount: 120,
      disabledCount: 5,
    })

    vi.spyOn(api, "listRbacItems").mockResolvedValue([
      {
        name: "SUPER_ADMIN",
        type: "ROLE",
        description: "Toàn quyền",
        systemItem: true,
        directChildren: ["ADMIN"],
        createdAt: "2026-10-01T00:00:00Z",
        updatedAt: "2026-10-01T00:00:00Z",
      },
      {
        name: "product.view",
        type: "PERMISSION",
        description: "Xem sản phẩm",
        systemItem: true,
        directChildren: [],
        createdAt: "2026-10-01T00:00:00Z",
        updatedAt: "2026-10-01T00:00:00Z",
      },
    ])

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth([
            "product.view",
            "inventory.view",
            "inventory.update",
            "user.view",
            "rbac.view",
          ])}
        >
          <OverviewPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    // Both SKUs of the same productId ("p-sony") appear in watchlist
    expect(await screen.findByText("Đen Khói")).toBeInTheDocument()
    expect(screen.getByText("Bạc Platinum")).toBeInTheDocument()
    expect(screen.getByText(/p-sony-v1/)).toBeInTheDocument()
    expect(screen.getByText(/p-sony-v2/)).toBeInTheDocument()
    expect(invSpy).toHaveBeenCalledWith(
      expect.objectContaining({ stockStatus: "out_of_stock" }),
      expect.anything()
    )
    expect(invSpy).toHaveBeenCalledWith(
      expect.objectContaining({ stockStatus: "low_stock" }),
      expect.anything()
    )
    expect(screen.getByText("780")).toBeInTheDocument()
    expect(screen.getByText("120")).toBeInTheDocument()

    // Open StockDialog specifically for the second SKU (p-sony-v2)
    const adjustButtons = screen.getAllByRole("button", { name: /điều chỉnh tồn/i })
    await userEvent.click(adjustButtons[1]!)
    const qtyInput = await screen.findByLabelText(/tổng tồn kho mới/i)
    await userEvent.clear(qtyInput)
    await userEvent.type(qtyInput, "12")
    await userEvent.click(screen.getByRole("button", { name: /xác nhận cập nhật/i }))
    await waitFor(() => expect(setStockSpy).toHaveBeenCalledWith("p-sony-v2", "p-sony", 12))
  })

  it("skips unauthorized endpoints when operator only holds inventory.view", async () => {
    const productsSpy = vi.spyOn(api, "listAdminProducts")
    const usersSpy = vi.spyOn(api, "listAdminUsers")
    const rbacSpy = vi.spyOn(api, "listRbacItems")
    const inventorySpy = vi.spyOn(api, "listAdminInventory").mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 8,
      totalItems: 0,
      totalPages: 0,
      summary: {
        totalTrackedProducts: 10,
        totalTrackedSkus: 15,
        inStockCount: 14,
        lowStockCount: 1,
        outOfStockCount: 0,
        totalQuantity: 100,
        totalReserved: 5,
        totalAvailable: 95,
      },
    })

    render(
      <MemoryRouter>
        <AuthContext.Provider value={makeAuth(["inventory.view"])}>
          <OverviewPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    await waitFor(() => expect(inventorySpy).toHaveBeenCalled())
    expect(await screen.findByText("95")).toBeInTheDocument()
    expect(productsSpy).not.toHaveBeenCalled()
    expect(usersSpy).not.toHaveBeenCalled()
    expect(rbacSpy).not.toHaveBeenCalled()
  })

  it("preserves fulfilled KPI cards when one upstream endpoint returns 503", async () => {
    vi.spyOn(api, "listAdminProducts").mockRejectedValue(
      new api.ApiClientError(
        503,
        "Authorization service is unavailable.",
        "AUTHORIZATION_SERVICE_UNAVAILABLE"
      )
    )
    vi.spyOn(api, "listAdminInventory").mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 8,
      totalItems: 0,
      totalPages: 0,
      summary: {
        totalTrackedProducts: 16,
        totalTrackedSkus: 24,
        inStockCount: 18,
        lowStockCount: 5,
        outOfStockCount: 1,
        totalQuantity: 120,
        totalReserved: 0,
        totalAvailable: 120,
      },
    })
    vi.spyOn(api, "listAdminUsers").mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 1,
      totalItems: 42,
      totalPages: 42,
      activeCount: 40,
      disabledCount: 2,
    })
    vi.spyOn(api, "listRbacItems").mockResolvedValue([
      {
        name: "SUPER_ADMIN",
        type: "ROLE",
        description: "Super administrator role",
        systemItem: true,
        directChildren: [],
        createdAt: "2026-10-01T00:00:00Z",
        updatedAt: "2026-10-01T00:00:00Z",
      },
    ])

    render(
      <MemoryRouter>
        <AuthContext.Provider
          value={makeAuth(["product.view", "inventory.view", "user.view", "rbac.view"])}
        >
          <OverviewPage />
        </AuthContext.Provider>
      </MemoryRouter>
    )

    expect(await screen.findByText("Authorization service is unavailable.")).toBeInTheDocument()
    expect(screen.getByText("40")).toBeInTheDocument()
    expect(screen.getByText("120")).toBeInTheDocument()
  })
})
