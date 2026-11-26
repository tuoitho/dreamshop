import * as path from "node:path"
import { fileURLToPath } from "node:url"
import { expect, test } from "@playwright/test"
import {
  loadContractSnapshots,
  mountContractRoutes,
  mountPresignedUploadTrap,
} from "../../fixtures/contract-routes"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const SNAPSHOTS_DIR = path.resolve(__dirname, "../../../contracts/http-snapshots")
const snapshots = loadContractSnapshots([
  path.join(SNAPSHOTS_DIR, "identity-auth.json"),
  path.join(SNAPSHOTS_DIR, "bff-storefront.json"),
])

test.describe("Mode 2 Contract-Lock Guard Verification", () => {
  test("traps unmocked /api/v1 requests instead of falling through to live network", async ({ page }) => {
    const violations: string[] = []
    await mountContractRoutes(page, snapshots, [], (v) => violations.push(v))

    await page.goto("about:blank")
    await page.evaluate(async () => {
      await fetch("http://localhost:8000/api/v1/orders").catch(() => undefined)
    })

    expect(violations).toContain("Unmocked API request: GET /api/v1/orders")
  })

  test("catches request key drift against recorded backend snapshot", async ({ page }) => {
    const violations: string[] = []
    await mountContractRoutes(
      page,
      snapshots,
      [
        {
          method: "POST",
          path: "/api/v1/auth/login",
          requestSnapshot: "auth_login_200",
          handle: () => ({ snapshot: "auth_login_200" }),
        },
      ],
      (v) => violations.push(v)
    )

    await page.goto("about:blank")
    await page.evaluate(async () => {
      await fetch("http://localhost:8000/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: "customer1@gmail.com", password: "123" }),
      }).catch(() => undefined)
    })

    expect(violations.some((v) => v.includes('Request key "username"'))).toBe(true)
  })

  test("catches empty/non-JSON request body and unexpected null request fields", async ({ page }) => {
    const violations: string[] = []
    await mountContractRoutes(
      page,
      snapshots,
      [
        {
          method: "POST",
          path: "/api/v1/auth/login",
          requestSnapshot: "auth_login_200",
          handle: () => ({ snapshot: "auth_login_200" }),
        },
      ],
      (v) => violations.push(v)
    )

    await page.goto("about:blank")
    await page.evaluate(async () => {
      await fetch("http://localhost:8000/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      }).catch(() => undefined)
      await fetch("http://localhost:8000/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: null, password: "123" }),
      }).catch(() => undefined)
    })

    expect(violations.filter((v) => v.includes("must not be null or empty")).length).toBe(2)
  })

  test("catches response shape drift when mock override deviates from golden snapshot", async ({ page }) => {
    const violations: string[] = []
    await mountContractRoutes(
      page,
      snapshots,
      [
        {
          method: "GET",
          path: "/api/v1/storefront/products",
          handle: () => ({
            snapshot: "storefront_products_200",
            body: { data: { wrongKey: [] }, message: "Products retrieved." },
          }),
        },
      ],
      (v) => violations.push(v)
    )

    await page.goto("about:blank")
    await page.evaluate(async () => {
      await fetch("http://localhost:8000/api/v1/storefront/products").catch(() => undefined)
    })

    expect(violations.some((v) => v.includes("Contract drift"))).toBe(true)
  })

  test("enforces ADR-012 presigned PUT rules and serves uploaded media in memory on GET", async ({ page }) => {
    const violations: string[] = []
    const uploads = await mountPresignedUploadTrap(
      page,
      { allowedContentTypes: ["image/png"] },
      (v) => violations.push(v)
    )

    await page.goto("about:blank")
    await page.evaluate(async () => {
      await fetch("http://localhost:9000/product-media/test.png?X-Amz-Signature=abc", {
        method: "PUT",
        headers: { "Content-Type": "image/png", Authorization: "Bearer forbidden-on-s3" },
        body: "fake-png",
      }).catch(() => undefined)
    })
    expect(violations.some((v) => v.includes("ADR-012 violation"))).toBe(true)
    expect(uploads).toHaveLength(0)

    const result = await page.evaluate(async () => {
      const putResp = await fetch("http://localhost:9000/product-media/ok.png?X-Amz-Signature=abc", {
        method: "PUT",
        headers: { "Content-Type": "image/png" },
        body: "fake-png",
      })
      const getResp = await fetch("http://localhost:9000/product-media/ok.png")
      return { putStatus: putResp.status, getStatus: getResp.status, getBody: await getResp.text() }
    })
    expect(result.putStatus).toBe(200)
    expect(result.getStatus).toBe(200)
    expect(result.getBody).toBe("fake-png")
    expect(uploads).toHaveLength(1)
  })
})
