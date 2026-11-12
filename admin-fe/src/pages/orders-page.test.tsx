import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import * as api from "@/lib/api"
import type { AdminOrder, AdminOrderPage } from "@/lib/types"
import { OrdersPage } from "./orders-page"

function makeAuth(permissions: string[]): AuthContextValue {
  const permSet = new Set(permissions)
  return {
    status: "authenticated",
    user: { userId: "u-1", email: "ops@sshop.vn", fullName: "Vận hành", roles: ["ADMIN"] },
    permissions: permSet,
    hasAnyAdminAccess: true,
    can: (perm: string) => permSet.has(perm),
    login: vi.fn(),
    logout: vi.fn(),
    refreshPermissions: vi.fn(),
  }
}

const sampleOrder: AdminOrder = {
  orderId: "ord-1",
  orderCode: "ORD-20261112-AAA111",
  userId: "user-1",
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
  items: [
    {
      productId: "dt-1",
      skuId: "dt-1-v1",
      quantity: 2,
      unitPrice: 150000,
      lineTotal: 300000,
      expectedName: "Nova X15",
      brand: "Nova",
      categoryName: "Điện thoại",
      variantLabel: "Đen 256GB",
      imageUrl: "",
      imageEmoji: "📱",
      imageHue: [210, 250],
      expectedProductVersion: 3,
    },
  ],
  subtotal: 300000,
  shippingFee: 25000,
  totalAmount: 325000,
  currency: "VND",
  createdAt: "2026-11-12T08:00:00Z",
  updatedAt: "2026-11-12T08:01:00Z",
  statusHistory: [],
}

const samplePage: AdminOrderPage = {
  items: [sampleOrder],
  page: 1,
  pageSize: 20,
  totalItems: 1,
  totalPages: 1,
  statusCounts: {
    PENDING_STOCK: 0,
    PENDING_PAYMENT: 0,
    PENDING_CONFIRMATION: 4,
    CONFIRMED: 1,
    PREPARING: 0,
    SHIPPING: 7,
    DELIVERED: 0,
    DELIVERY_FAILED: 0,
    CANCELLED: 2,
  },
}

function renderPage(permissions: string[]) {
  return render(
    <MemoryRouter>
      <AuthContext.Provider value={makeAuth(permissions)}>
        <OrdersPage />
      </AuthContext.Provider>
    </MemoryRouter>
  )
}

describe("OrdersPage", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("shows status tabs with counts and filters by the selected status", async () => {
    const listSpy = vi.spyOn(api, "listAdminOrders").mockResolvedValue(samplePage)

    renderPage(["order.admin.view"])

    expect(await screen.findByText("ORD-20261112-AAA111")).toBeInTheDocument()
    expect(screen.getByText("Nguyễn Văn A")).toBeInTheDocument()
    expect(listSpy).toHaveBeenCalledWith({ page: 1, pageSize: 20 }, expect.anything())

    const shippingTab = screen.getByRole("button", { name: /đang giao/i })
    expect(within(shippingTab).getByText("7")).toBeInTheDocument()
    await userEvent.click(shippingTab)

    await waitFor(() =>
      expect(listSpy).toHaveBeenLastCalledWith(
        { status: "SHIPPING", page: 1, pageSize: 20 },
        expect.anything()
      )
    )
  })

  it("searches by order code", async () => {
    const listSpy = vi.spyOn(api, "listAdminOrders").mockResolvedValue(samplePage)

    renderPage(["order.admin.view"])
    await screen.findByText("ORD-20261112-AAA111")

    await userEvent.type(screen.getByPlaceholderText(/mã đơn/i), " ord-2026 ")
    await userEvent.click(screen.getByRole("button", { name: /^lọc$/i }))

    await waitFor(() =>
      expect(listSpy).toHaveBeenLastCalledWith({ q: "ord-2026", page: 1, pageSize: 20 }, expect.anything())
    )
  })

  it("opens the order detail with its status timeline", async () => {
    vi.spyOn(api, "listAdminOrders").mockResolvedValue(samplePage)
    const getSpy = vi.spyOn(api, "getAdminOrder").mockResolvedValue({
      order: {
        ...sampleOrder,
        statusHistory: [
          { toStatus: "PENDING_STOCK", actorType: "CUSTOMER", createdAt: "2026-11-12T08:00:00Z" },
          {
            fromStatus: "PENDING_CONFIRMATION",
            toStatus: "CONFIRMED",
            actorType: "SYSTEM",
            createdAt: "2026-11-12T08:01:00Z",
          },
        ],
      },
      allowedNextStatuses: ["PREPARING", "CANCELLED"],
    })

    renderPage(["order.admin.view", "order.admin.update"])
    await userEvent.click(await screen.findByRole("button", { name: /xem đơn ORD-20261112-AAA111/i }))

    await waitFor(() => expect(getSpy).toHaveBeenCalledWith("ord-1", expect.anything()))
    const timeline = await screen.findByRole("list", { name: /lịch sử trạng thái/i })
    const entries = within(timeline).getAllByRole("listitem")
    expect(entries).toHaveLength(2)
    expect(within(entries[1]).getByText(/hệ thống/i)).toBeInTheDocument()
    expect(within(entries[1]).getByText("Đã xác nhận")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /cập nhật trạng thái/i })).toBeInTheDocument()
  })

  it("hides the status update action without order.admin.update", async () => {
    vi.spyOn(api, "listAdminOrders").mockResolvedValue(samplePage)
    vi.spyOn(api, "getAdminOrder").mockResolvedValue({
      order: sampleOrder,
      allowedNextStatuses: ["PREPARING", "CANCELLED"],
    })

    renderPage(["order.admin.view"])
    await userEvent.click(await screen.findByRole("button", { name: /xem đơn ORD-20261112-AAA111/i }))

    expect(await screen.findByRole("list", { name: /lịch sử trạng thái/i })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /cập nhật trạng thái/i })).not.toBeInTheDocument()
  })
})
