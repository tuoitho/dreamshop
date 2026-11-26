import { expect, type Locator, type Page } from "@playwright/test"

export class AdminProductsPage {
  readonly page: Page
  readonly addProductButton: Locator
  readonly drawerTitle: Locator
  readonly mediaFileInput: Locator

  constructor(page: Page) {
    this.page = page
    this.addProductButton = page.getByRole("button", { name: "Thêm sản phẩm" })
    this.drawerTitle = page.getByRole("heading", { name: "Thêm sản phẩm mới" })
    this.mediaFileInput = page.locator('input[type="file"]')
  }

  async goto() {
    await this.page.goto("/products")
    await expect(this.addProductButton).toBeVisible()
  }

  async openCreateDrawer() {
    await this.addProductButton.click()
    await expect(this.drawerTitle).toBeVisible()
  }

  async uploadMediaBuffer(file: { name: string; mimeType: string; buffer: Buffer }) {
    await this.mediaFileInput.setInputFiles(file)
  }
}
