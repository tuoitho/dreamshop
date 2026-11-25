import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import * as api from "@/lib/api"
import type { OrderDetail } from "@/lib/types"
import OrderDetailPage from "./order-detail-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    getOrder: vi.fn(),
  }
})

function makeOrder(overrides: Partial<OrderDetail>): OrderDetail {
  return {
    orderId: "ord-det-1",
    orderCode: "ORD-20261125-DET01",
    userId: "u-1",
    status: "CANCELLED",
    paymentMethod: "MOMO",
    paymentStatus: "REFUNDED",
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
    shippingFee: 0,
    totalAmount: 200000,
    currency: "VND",
    createdAt: "2026-11-25T08:00:00Z",
    updatedAt: "2026-11-25T08:05:00Z",
    ...overrides,
  }
}

describe("OrderDetailPage refund states", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("displays green refunded banner and refundTransId when paymentStatus is REFUNDED", async () => {
    vi.mocked(api.getOrder).mockResolvedValueOnce(
      makeOrder({
        paymentStatus: "REFUNDED",
        refundTransId: "momo-ref-888",
      })
    )

    render(
      <MemoryRouter initialEntries={["/don-hang/ord-det-1"]}>
        <Routes>
          <Route path="/don-hang/:orderId" element={<OrderDetailPage />} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByText(/Đơn hàng đã được hoàn tiền/i)).toBeInTheDocument()
    expect(screen.getAllByText(/Đã hoàn tiền/i).length).toBeGreaterThan(0)
    expect(screen.getByText(/momo-ref-888/i)).toBeInTheDocument()
  })

  it("displays friendly pending-refund status when paymentStatus is REFUND_FAILED without raw error", async () => {
    vi.mocked(api.getOrder).mockResolvedValueOnce(
      makeOrder({
        paymentStatus: "REFUND_FAILED",
      })
    )

    render(
      <MemoryRouter initialEntries={["/don-hang/ord-det-1"]}>
        <Routes>
          <Route path="/don-hang/:orderId" element={<OrderDetailPage />} />
        </Routes>
      </MemoryRouter>
    )

    expect(await screen.findByText(/Đơn hàng đang xử lý hoàn tiền/i)).toBeInTheDocument()
    expect(screen.getAllByText(/Đang xử lý hoàn tiền/i).length).toBeGreaterThan(0)
    expect(screen.queryByText(/thất bại/i)).not.toBeInTheDocument()
  })
})
