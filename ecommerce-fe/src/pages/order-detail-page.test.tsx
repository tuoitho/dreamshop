import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import * as api from "@/lib/api"
import type { OrderDetail } from "@/lib/types"
import OrderDetailPage from "./order-detail-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    getOrder: vi.fn(),
    cancelOrder: vi.fn(),
  }
})

const BASE_ORDER: OrderDetail = {
  orderId: "ord-cod-1",
  orderCode: "ORD-20261112-COD001",
  userId: "u-1",
  status: "PENDING_CONFIRMATION",
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
      quantity: 1,
      unitPrice: 200000,
      lineTotal: 200000,
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
  subtotal: 200000,
  shippingFee: 25000,
  totalAmount: 225000,
  currency: "VND",
  autoConfirmAt: "2026-11-12T08:01:00Z",
  createdAt: "2026-11-12T08:00:00Z",
  updatedAt: "2026-11-12T08:00:00Z",
  statusHistory: [],
}

function renderDetail(order: OrderDetail, path = "/don-hang/ord-cod-1") {
  vi.mocked(api.getOrder).mockResolvedValue(order)
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/don-hang/:orderId" element={<OrderDetailPage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe("OrderDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("tells a customer who just placed a COD order that it is confirmed automatically", async () => {
    const user = userEvent.setup()
    renderDetail(BASE_ORDER, "/don-hang/ord-cod-1?placed=1")

    expect(await screen.findByText(/hệ thống sẽ tự xác nhận trong khoảng 1 phút/i)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /hủy đơn hàng/i })).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: /làm mới/i }))
    await waitFor(() => expect(api.getOrder).toHaveBeenCalledTimes(2))
  })

  it("lets the customer cancel a confirmed order", async () => {
    const user = userEvent.setup()
    vi.mocked(api.cancelOrder).mockResolvedValue({ ...BASE_ORDER, status: "CANCELLED" })
    renderDetail({ ...BASE_ORDER, status: "CONFIRMED" })

    await user.click(await screen.findByRole("button", { name: /hủy đơn hàng/i }))

    await waitFor(() => expect(api.cancelOrder).toHaveBeenCalledWith("ord-cod-1"))
  })

  it("shows shipping details and hides cancellation once the order ships", async () => {
    renderDetail({ ...BASE_ORDER, status: "SHIPPING", shippingCarrier: "GHN", trackingNumber: "GHN123" })

    expect(await screen.findByText("GHN")).toBeInTheDocument()
    expect(screen.getByText("GHN123")).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: /hủy đơn hàng/i })).not.toBeInTheDocument()
  })

  it("renders the status timeline oldest first", async () => {
    renderDetail({
      ...BASE_ORDER,
      status: "CONFIRMED",
      statusHistory: [
        { toStatus: "PENDING_STOCK", actorType: "CUSTOMER", createdAt: "2026-11-12T08:00:00Z" },
        { fromStatus: "PENDING_STOCK", toStatus: "PENDING_CONFIRMATION", actorType: "SYSTEM", createdAt: "2026-11-12T08:00:01Z" },
        { fromStatus: "PENDING_CONFIRMATION", toStatus: "CONFIRMED", actorType: "SYSTEM", createdAt: "2026-11-12T08:01:00Z" },
      ],
    })

    const timeline = await screen.findByRole("list", { name: /lịch sử đơn hàng/i })
    const entries = within(timeline).getAllByRole("listitem")
    expect(entries).toHaveLength(3)
    expect(within(entries[0]).getByText("Đang giữ hàng")).toBeInTheDocument()
    expect(within(entries[1]).getByText("Chờ xác nhận")).toBeInTheDocument()
    expect(within(entries[2]).getByText("Đã xác nhận")).toBeInTheDocument()
  })

  it("displays green refunded banner and refundTransId when paymentStatus is REFUNDED", async () => {
    renderDetail({
      ...BASE_ORDER,
      status: "CANCELLED",
      paymentMethod: "MOMO",
      paymentStatus: "REFUNDED",
      refundTransId: "momo-ref-888",
    })

    expect(await screen.findByText(/Đơn hàng đã được hoàn tiền/i)).toBeInTheDocument()
    expect(screen.getAllByText(/Đã hoàn tiền/i).length).toBeGreaterThan(0)
    expect(screen.getByText(/momo-ref-888/i)).toBeInTheDocument()
  })

  it("displays friendly pending-refund status when paymentStatus is REFUND_FAILED without raw error", async () => {
    renderDetail({
      ...BASE_ORDER,
      status: "CANCELLED",
      paymentMethod: "MOMO",
      paymentStatus: "REFUND_FAILED",
    })

    expect(await screen.findByText(/Đơn hàng đang xử lý hoàn tiền/i)).toBeInTheDocument()
    expect(screen.getAllByText(/Đang xử lý hoàn tiền/i).length).toBeGreaterThan(0)
    expect(screen.queryByText(/thất bại/i)).not.toBeInTheDocument()
  })
})
