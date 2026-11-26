import { expect, type Locator, type Page } from "@playwright/test"

export class AdminOrdersPage {
  readonly page: Page
  readonly heading: Locator
  readonly manualRefundButton: Locator
  readonly refundReasonInput: Locator
  readonly confirmRefundButton: Locator

  constructor(page: Page) {
    this.page = page
    this.heading = page.getByRole("heading", { name: "Quản lý đơn hàng" })
    this.manualRefundButton = page.getByRole("button", { name: "Hoàn tiền thủ công" })
    this.refundReasonInput = page.getByLabel(/Lý do hoàn tiền/i)
    this.confirmRefundButton = page.getByRole("button", { name: "Xác nhận hoàn tiền" })
  }

  async goto() {
    await this.page.goto("/orders")
    await expect(this.heading).toBeVisible()
  }

  async openOrderDetail(orderCode: string) {
    await this.page.getByRole("button", { name: `Xem đơn ${orderCode}` }).click()
    await expect(this.page.getByRole("heading", { name: new RegExp(orderCode) })).toBeVisible()
  }

  async executeManualRefund(reason: string) {
    await expect(this.manualRefundButton).toBeVisible()
    await this.manualRefundButton.click()
    await expect(this.refundReasonInput).toBeVisible()
    await this.refundReasonInput.fill(reason)
    await this.confirmRefundButton.click()
  }
}
