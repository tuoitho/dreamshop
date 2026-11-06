import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import * as api from "@/lib/api"
import type { OrderDetail } from "@/lib/types"
import OrdersPage from "./orders-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    listOrders: vi.fn(),
    cancelOrder: vi.fn(),
  }
})

const PENDING_ORDER: OrderDetail = {
  orderId: "ord-pending-1",
  orderCode: "ORD-20261105-PND001",
  userId: "u-1",
  status: "PENDING_PAYMENT",
  paymentMethod: "MOMO",
  paymentStatus: "PENDING",
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
      quantity: 1,
      unitPrice: 18990000,
      lineTotal: 18990000,
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
  subtotal: 18990000,
  shippingFee: 0,
  totalAmount: 18990000,
  currency: "VND",
  payUrl: "https://test-payment.momo.vn/pay/ord-pending-1",
  createdAt: "2026-11-05T10:00:00Z",
  updatedAt: "2026-11-05T10:00:00Z",
}

describe("OrdersPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("renders paginated order history and cancels a PENDING_PAYMENT order", async () => {
    const user = userEvent.setup()
    vi.mocked(api.listOrders).mockResolvedValue({
      items: [PENDING_ORDER],
      page: 1,
      pageSize: 10,
      totalItems: 1,
      totalPages: 1,
    })
    vi.mocked(api.cancelOrder).mockResolvedValue({
      ...PENDING_ORDER,
      status: "CANCELLED",
      cancelReason: "USER_CANCELLED",
    })

    render(
      <MemoryRouter initialEntries={["/don-hang"]}>
        <Routes>
          <Route path="/don-hang" element={<OrdersPage />} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByText("ORD-20261105-PND001")).toBeInTheDocument()
    expect(screen.getByText("Nova X15")).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: /Hủy đơn/i }))
    await waitFor(() => {
      expect(api.cancelOrder).toHaveBeenCalledWith("ord-pending-1")
    })
  })
})
