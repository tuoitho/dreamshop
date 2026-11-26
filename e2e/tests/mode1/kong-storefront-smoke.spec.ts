import { expect, test } from "@playwright/test"
import {
  assertNoTokenInWebStorage,
  assertOrSkipKongStackReachable,
  loginViaKongCookieJar,
} from "../../fixtures/kong-auth"
import { HomePage } from "../../pages/HomePage"

test.describe("Mode 1 (Live via Kong): Storefront Smoke", () => {
  test("restores session via HttpOnly refresh cookie through Kong and loads live BFF products", async ({
    page,
  }) => {
    await assertOrSkipKongStackReachable(page)

    // Reuses default seeded customer account from AGENTS.md without creating throwaway users
    await loginViaKongCookieJar(page, { role: "customer" })

    const refreshResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/auth/refresh") && resp.status() === 200
    )
    const productsResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/storefront/products") && resp.status() === 200
    )

    const homePage = new HomePage(page)
    await homePage.goto()

    const [refresh, products] = await Promise.all([refreshResp, productsResp])
    expect(refresh.status()).toBe(200)

    const productsBody = (await products.json()) as {
      data?: { items?: unknown[]; page?: number }
      message?: string
    }
    expect(Array.isArray(productsBody.data?.items)).toBe(true)
    expect(typeof productsBody.message).toBe("string")
    await homePage.expectLoaded()

    await assertNoTokenInWebStorage(page)
  })
})
