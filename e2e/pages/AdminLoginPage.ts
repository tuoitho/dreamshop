import { expect, type Locator, type Page } from "@playwright/test"

export class AdminLoginPage {
  readonly page: Page
  readonly emailInput: Locator
  readonly passwordInput: Locator
  readonly submitButton: Locator
  readonly errorAlert: Locator

  constructor(page: Page) {
    this.page = page
    this.emailInput = page.getByLabel("Email vận hành")
    this.passwordInput = page.getByLabel("Mật khẩu")
    this.submitButton = page.getByRole("button", { name: /Đăng nhập vào trung tâm/i })
    this.errorAlert = page.getByRole("alert")
  }

  async goto() {
    await this.page.goto("/login")
    await expect(this.emailInput).toBeVisible()
  }

  async submit(email: string, password: string) {
    await this.emailInput.fill(email)
    await this.passwordInput.fill(password)
    await this.submitButton.click()
  }
}
