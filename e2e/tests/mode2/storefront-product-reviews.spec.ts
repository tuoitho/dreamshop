import * as fs from "node:fs"
import * as path from "node:path"
import { fileURLToPath } from "node:url"
import { expect, test } from "@playwright/test"
import {
  loadContractSnapshots,
  mountContractRoutes,
  mountPresignedUploadTrap,
  type ContractRoute,
} from "../../fixtures/contract-routes"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const SNAPSHOTS_DIR = path.resolve(__dirname, "../../../contracts/http-snapshots")
const IMGS_DIR = path.resolve(__dirname, "../../reports/product-reviews/imgs")

const snapshots = loadContractSnapshots(
  [
    path.join(SNAPSHOTS_DIR, "identity-auth.json"),
    path.join(SNAPSHOTS_DIR, "bff-storefront.json"),
  ],
  {
    nullable: ["phone", "dateOfBirth", "gender", "provinceCode", "wardCode", "addressLine"],
  }
)

test.describe("Mode 2 (Shape-Locked Mock): Storefront Product Reviews & Photo Upload", () => {
  test.beforeAll(() => {
    fs.mkdirSync(IMGS_DIR, { recursive: true })
  })

  test("hiển thị đánh giá sản phẩm, tải ảnh review theo ADR-012, gửi đánh giá mới và giữ state sau reload", async ({
    page,
  }) => {
    const baseList = structuredClone(
      snapshots.snapshots.storefront_product_reviews_200.responseBody as {
        data: {
          items: Array<Record<string, unknown>>
          page: number
          pageSize: number
          totalItems: number
          totalPages: number
          summary: {
            averageRating: number
            totalReviews: number
            breakdown: Array<{ rating: number; count: number }>
          }
        }
        message: string
      }
    )
    const baseElig = structuredClone(
      snapshots.snapshots.reviews_eligibility_200.responseBody as {
        data: {
          canReview: boolean
          eligibleOrders: Array<Record<string, unknown>>
          myReviews: Array<Record<string, unknown>>
        }
        message: string
      }
    )
    const baseCreate = structuredClone(
      snapshots.snapshots.reviews_create_201.responseBody as {
        data: {
          review: Record<string, unknown>
          summary: {
            averageRating: number
            totalReviews: number
            breakdown: Array<{ rating: number; count: number }>
          }
        }
        message: string
      }
    )

    const state = {
      submitted: false,
      newComment: "",
      newImages: [] as string[],
    }

    const routes: ContractRoute[] = [
      {
        method: "POST",
        path: "/api/v1/auth/refresh",
        auth: "refreshCookie",
        handle: () => ({ snapshot: "auth_refresh_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/users/me",
        auth: "bearer",
        handle: () => ({ snapshot: "users_me_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/cart",
        auth: "bearer",
        unauthenticatedSnapshot: "bff_unauthenticated_401",
        handle: () => ({ snapshot: "cart_get_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/favorites",
        auth: "bearer",
        unauthenticatedSnapshot: "bff_unauthenticated_401",
        handle: () => ({ snapshot: "favorites_get_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/storefront/products",
        handle: () => ({ snapshot: "storefront_products_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/storefront/products/dt-1",
        handle: () => ({ snapshot: "storefront_product_detail_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/storefront/products/dt-1/reviews",
        handle: () => {
          const body = structuredClone(baseList)
          if (state.submitted) {
            const created = structuredClone(baseCreate.data.review)
            created.id = "rev-202"
            created.comment = state.newComment
            created.images = state.newImages
            body.data.items = [created, ...body.data.items]
            body.data.totalItems = body.data.items.length
            body.data.summary.totalReviews = body.data.items.length
            body.data.summary.breakdown[0].count = body.data.items.length
          }
          return { snapshot: "storefront_product_reviews_200", body }
        },
      },
      {
        method: "GET",
        path: "/api/v1/reviews/eligibility",
        auth: "bearer",
        unauthenticatedSnapshot: "bff_unauthenticated_401",
        handle: () => {
          const body = structuredClone(baseElig)
          if (state.submitted) {
            body.data.canReview = false
            body.data.eligibleOrders[0].reviewed = true
            body.data.eligibleOrders[0].reviewId = "rev-202"
          }
          return { snapshot: "reviews_eligibility_200", body }
        },
      },
      {
        method: "POST",
        path: "/api/v1/reviews/media/presign",
        auth: "bearer",
        unauthenticatedSnapshot: "bff_unauthenticated_401",
        requestSnapshot: "reviews_media_presign_200",
        handle: () => ({ snapshot: "reviews_media_presign_200" }),
      },
      {
        method: "POST",
        path: "/api/v1/reviews",
        auth: "bearer",
        unauthenticatedSnapshot: "bff_unauthenticated_401",
        requestSnapshot: "reviews_create_201",
        handle: ({ body }) => {
          const payload = (body ?? {}) as { comment?: string; images?: string[] }
          state.submitted = true
          state.newComment = payload.comment ?? ""
          state.newImages = payload.images ?? []
          const reply = structuredClone(baseCreate)
          reply.data.review.id = "rev-202"
          reply.data.review.comment = state.newComment
          reply.data.review.images = state.newImages
          reply.data.summary.totalReviews = 2
          reply.data.summary.breakdown[0].count = 2
          return { snapshot: "reviews_create_201", body: reply }
        },
      },
    ]

    await mountContractRoutes(page, snapshots, routes)
    const uploads = await mountPresignedUploadTrap(page, {
      allowedContentTypes: ["image/png"],
    })

    await page.context().addCookies([
      {
        name: "refresh_token",
        value: "e2e-customer-refresh-cookie",
        domain: "localhost",
        path: "/api/v1/auth",
        httpOnly: true,
        sameSite: "Lax",
      },
    ])

    await page.goto("/san-pham/dt-1")

    const reviewsSection = page.getByTestId("product-reviews-section")
    await expect(reviewsSection).toBeVisible()
    await expect(
      page.getByTestId("review-card-rev-101").getByText("Máy chạy mượt, đóng gói cẩn thận.")
    ).toBeVisible()
    await expect(
      page.getByTestId("review-card-rev-101").getByText("Cảm ơn bạn đã ủng hộ DreamShop!")
    ).toBeVisible()

    await page.getByTestId("open-review-composer").click()
    await page.getByLabel("Nhận xét của bạn").fill("Pin trâu, màn hình đẹp xuất sắc!")

    const tinyPng = Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
      "base64"
    )
    const presignResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/reviews/media/presign") && resp.status() === 200
    )
    await page.getByTestId("review-image-input").setInputFiles({
      name: "review-1.png",
      mimeType: "image/png",
      buffer: tinyPng,
    })
    await presignResp
    await expect(page.getByAltText("Ảnh đánh giá 1")).toBeVisible()
    await expect.poll(() => uploads.length).toBe(1)
    expect(uploads[0].url).toContain("/product-media/reviews/2026/11/review-1.png")

    const createResp = page.waitForResponse(
      (resp) => resp.url().endsWith("/api/v1/reviews") && resp.status() === 201
    )
    await page.getByRole("button", { name: "Gửi đánh giá" }).click()
    await createResp

    await expect(
      page.getByTestId("review-card-rev-202").getByText("Pin trâu, màn hình đẹp xuất sắc!")
    ).toBeVisible()
    await expect(page.getByTestId("reviews-total-count")).toContainText("2 đánh giá")

    await page.reload()
    await expect(
      page.getByTestId("review-card-rev-202").getByText("Pin trâu, màn hình đẹp xuất sắc!")
    ).toBeVisible()
  })
})
