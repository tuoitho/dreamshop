import { expect, type Locator, type Page } from "@playwright/test"

export class HomePage {
  readonly page: Page
  readonly heroHeading: Locator

  constructor(page: Page) {
    this.page = page
    this.heroHeading = page.getByRole("heading", { level: 1 })
  }

  async goto() {
    await this.page.goto("/")
  }

  async expectLoaded() {
    await expect(this.heroHeading).toBeVisible()
  }

  async expectProductVisible(name: string) {
    await expect(this.page.getByText(name).first()).toBeVisible()
  }
}
