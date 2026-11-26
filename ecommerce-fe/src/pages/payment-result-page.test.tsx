import { beforeEach, describe, expect, it, vi } from "vitest"
import { act, render, screen, waitFor } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import * as api from "@/lib/api"
import type { OrderDetail } from "@/lib/types"
import PaymentResultPage from "./payment-result-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    getOrder: vi.fn(),
  }
})

function makeCartValue(retryLoad = vi.fn().mockResolvedValue(undefined)): CartContextValue {
  return {
    items: [],
    addItem: vi.fn(),
    updateQuantity: vi.fn(),
    removeItem: vi.fn(),
    removeSelected: vi.fn(),
    clearCart: vi.fn(),
    toggleSelected: vi.fn(),
    selectOnly: vi.fn(),
    toggleAllSelected: vi.fn(),
    selectedItems: [],
    selectedCount: 0,
    totalQuantity: 0,
    selectedTotal: 0,
    allSelected: false,
    loading: false,
    degraded: false,
    loadError: null,
    retryLoad,
  }
}

function makeOrder(overrides: Partial<OrderDetail>): OrderDetail {
  return {
    orderId: "ord-momo-1",
    orderCode: "ORD-20261105-MOMO01",
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
    items: [],
    subtotal: 300000,
    shippingFee: 0,
    totalAmount: 300000,
    currency: "VND",
    createdAt: "2026-11-05T10:00:00Z",
    updatedAt: "2026-11-05T10:00:00Z",
    ...overrides,
  }
}

describe("PaymentResultPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  it("pollsPendingPaymentOrderUntilConfirmedAndRefreshesCart", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const retryLoad = vi.fn().mockResolvedValue(undefined)

    vi.mocked(api.getOrder)
      .mockResolvedValueOnce(makeOrder({ status: "PENDING_PAYMENT", paymentStatus: "PENDING" }))
      .mockResolvedValueOnce(makeOrder({ status: "CONFIRMED", paymentStatus: "PAID" }))

    render(
      <CartContext value={makeCartValue(retryLoad)}>
        <MemoryRouter initialEntries={["/thanh-toan/ket-qua?orderId=ord-momo-1&result=success"]}>
          <Routes>
            <Route path="/thanh-toan/ket-qua" element={<PaymentResultPage />} />
          </Routes>
        </MemoryRouter>
      </CartContext>
    )

    expect(await screen.findByText(/Đang đồng bộ kết quả thanh toán/i)).toBeInTheDocument()
    expect(retryLoad).not.toHaveBeenCalled()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(550)
    })

    await waitFor(() => {
      expect(screen.getByText(/ORD-20261105-MOMO01/)).toBeInTheDocument()
      expect(screen.getByText(/Thanh toán thành công/i)).toBeInTheDocument()
    })
    expect(retryLoad).toHaveBeenCalledTimes(1)
  })

  it("renders clear refund reconciliation notice when status is CANCELLED and paymentStatus is REFUND_REQUIRED", async () => {
    vi.mocked(api.getOrder).mockResolvedValueOnce(
      makeOrder({
        status: "CANCELLED",
        paymentStatus: "REFUND_REQUIRED",
        cancelReason: "PAYMENT_EXPIRED",
      })
    )

    render(
      <CartContext value={makeCartValue()}>
        <MemoryRouter initialEntries={["/thanh-toan/ket-qua?orderId=ord-momo-1&result=success"]}>
          <Routes>
            <Route path="/thanh-toan/ket-qua" element={<PaymentResultPage />} />
          </Routes>
        </MemoryRouter>
      </CartContext>
    )

    expect(await screen.findByText(/hoàn tiền/i)).toBeInTheDocument()
    expect(screen.getByText(/ORD-20261105-MOMO01/)).toBeInTheDocument()
  })

  it("renders green refunded banner when status is CANCELLED and paymentStatus is REFUNDED", async () => {
    vi.mocked(api.getOrder).mockResolvedValueOnce(
      makeOrder({
        status: "CANCELLED",
        paymentStatus: "REFUNDED",
        refundTransId: "momo-ref-555",
      })
    )

    render(
      <CartContext value={makeCartValue()}>
        <MemoryRouter initialEntries={["/thanh-toan/ket-qua?orderId=ord-momo-1&result=success"]}>
          <Routes>
            <Route path="/thanh-toan/ket-qua" element={<PaymentResultPage />} />
          </Routes>
        </MemoryRouter>
      </CartContext>
    )

    expect(await screen.findByText(/Đã hoàn tiền/i)).toBeInTheDocument()
    expect(screen.getByText(/ORD-20261105-MOMO01/)).toBeInTheDocument()
  })

  it("renders customer-friendly pending refund notice when status is CANCELLED and paymentStatus is REFUND_FAILED", async () => {
    vi.mocked(api.getOrder).mockResolvedValueOnce(
      makeOrder({
        status: "CANCELLED",
        paymentStatus: "REFUND_FAILED",
      })
    )

    render(
      <CartContext value={makeCartValue()}>
        <MemoryRouter initialEntries={["/thanh-toan/ket-qua?orderId=ord-momo-1&result=success"]}>
          <Routes>
            <Route path="/thanh-toan/ket-qua" element={<PaymentResultPage />} />
          </Routes>
        </MemoryRouter>
      </CartContext>
    )

    expect(await screen.findByText(/đang xử lý hoàn tiền/i)).toBeInTheDocument()
    expect(screen.queryByText(/thất bại/i)).not.toBeInTheDocument()
  })
})
