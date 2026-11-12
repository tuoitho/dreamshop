import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import * as api from "@/lib/api"
import type { AdminOrder } from "@/lib/types"
import { OrderStatusDialog } from "./order-status-dialog"

const order = {
  orderId: "ord-1",
  orderCode: "ORD-20261112-AAA111",
  status: "PREPARING",
} as AdminOrder

function renderDialog(allowed: AdminOrder["status"][], callbacks = { onUpdated: vi.fn(), onConflict: vi.fn() }) {
  render(
    <OrderStatusDialog
      open
      onOpenChange={vi.fn()}
      order={order}
      allowedNextStatuses={allowed}
      onUpdated={callbacks.onUpdated}
      onConflict={callbacks.onConflict}
    />
  )
  return callbacks
}

describe("OrderStatusDialog", () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it("offers only the allowed next statuses", () => {
    renderDialog(["SHIPPING", "CANCELLED"])

    expect(screen.getByRole("radio", { name: "Đang giao" })).toBeInTheDocument()
    expect(screen.getByRole("radio", { name: "Đã hủy" })).toBeInTheDocument()
    expect(screen.queryByRole("radio", { name: "Đã giao" })).not.toBeInTheDocument()
  })

  it("collects carrier and tracking number when moving to shipping", async () => {
    const updateSpy = vi.spyOn(api, "updateAdminOrderStatus").mockResolvedValue({
      order: { ...order, status: "SHIPPING" },
      allowedNextStatuses: ["DELIVERED", "DELIVERY_FAILED"],
    })
    const { onUpdated } = renderDialog(["SHIPPING", "CANCELLED"])

    await userEvent.click(screen.getByRole("radio", { name: "Đang giao" }))
    await userEvent.type(screen.getByLabelText(/đơn vị vận chuyển/i), "GHN")
    await userEvent.type(screen.getByLabelText(/mã vận đơn/i), "T1")
    await userEvent.click(screen.getByRole("button", { name: /xác nhận/i }))

    await waitFor(() =>
      expect(updateSpy).toHaveBeenCalledWith("ord-1", {
        expectedStatus: "PREPARING",
        targetStatus: "SHIPPING",
        reason: "",
        shippingCarrier: "GHN",
        trackingNumber: "T1",
      })
    )
    expect(onUpdated).toHaveBeenCalledWith(expect.objectContaining({ allowedNextStatuses: ["DELIVERED", "DELIVERY_FAILED"] }))
  })

  it("requires a reason before cancelling", async () => {
    const updateSpy = vi.spyOn(api, "updateAdminOrderStatus")
    renderDialog(["SHIPPING", "CANCELLED"])

    await userEvent.click(screen.getByRole("radio", { name: "Đã hủy" }))

    expect(screen.getByText(/bắt buộc nhập lý do/i)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /xác nhận/i })).toBeDisabled()
    expect(screen.queryByLabelText(/đơn vị vận chuyển/i)).not.toBeInTheDocument()
    expect(updateSpy).not.toHaveBeenCalled()
  })

  it("reports a concurrent status change and asks the caller to reload", async () => {
    vi.spyOn(api, "updateAdminOrderStatus").mockRejectedValue(
      new api.ApiClientError(409, "Order status changed", "ORDER_STATUS_CONFLICT")
    )
    const { onConflict, onUpdated } = renderDialog(["SHIPPING", "CANCELLED"])

    await userEvent.click(screen.getByRole("radio", { name: "Đã hủy" }))
    await userEvent.type(screen.getByLabelText(/lý do/i), "Hết hàng")
    await userEvent.click(screen.getByRole("button", { name: /xác nhận/i }))

    expect(await screen.findByText(/trạng thái đơn vừa thay đổi, vui lòng kiểm tra lại/i)).toBeInTheDocument()
    expect(onConflict).toHaveBeenCalled()
    expect(onUpdated).not.toHaveBeenCalled()
  })
})
