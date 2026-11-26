import * as fs from "node:fs"
import * as path from "node:path"
import { fileURLToPath } from "node:url"
import { expect, test } from "@playwright/test"
import {
  loadContractSnapshots,
  mountContractRoutes,
  type ContractRoute,
} from "../../../fixtures/contract-routes"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const SNAPSHOTS_DIR = path.resolve(__dirname, "../../../../contracts/http-snapshots")
const IMGS_DIR = path.resolve(__dirname, "../../../reports/admin-reviews/imgs")

const snapshots = loadContractSnapshots(
  [
    path.join(SNAPSHOTS_DIR, "identity-auth.json"),
    path.join(SNAPSHOTS_DIR, "bff-admin.json"),
  ],
  {
    nullable: ["phone", "dateOfBirth", "gender", "provinceCode", "wardCode", "addressLine"],
  }
)

test.describe("Mode 2 (Shape-Locked Mock): Admin Review Moderation", () => {
  test.beforeAll(() => {
    fs.mkdirSync(IMGS_DIR, { recursive: true })
  })

  test("kiểm duyệt đánh giá: phản hồi chính thức, ẩn đánh giá kèm lý do và giữ state sau reload", async ({
    page,
  }) => {
    const adminProfile = structuredClone(
      snapshots.snapshots.users_me_200.responseBody as Record<string, unknown>
    )
    adminProfile.email = "admin@example.com"
    adminProfile.fullName = "Quản trị viên Hệ thống"
    adminProfile.roles = ["ADMIN"]

    const permsBody = structuredClone(
      snapshots.snapshots.users_me_permissions_200.responseBody as Record<string, unknown>
    )
    permsBody.data = ["review.admin.view", "review.admin.moderate"]

    const baseList = structuredClone(
      snapshots.snapshots.admin_reviews_list_200.responseBody as {
        data: {
          items: Array<Record<string, unknown>>
          page: number
          pageSize: number
          totalItems: number
          totalPages: number
          visibleCount: number
          hiddenCount: number
          unrepliedCount: number
          averageRating: number
        }
        message: string
      }
    )

    const state = {
      reply: String(baseList.data.items[0].adminReply ?? ""),
      hidden: false,
      hiddenReason: "",
    }

    const routes: ContractRoute[] = [
      {
        method: "POST",
        path: "/api/v1/auth/admin/refresh",
        auth: "refreshCookie",
        handle: () => ({ snapshot: "auth_refresh_200" }),
      },
      {
        method: "GET",
        path: "/api/v1/users/me",
        auth: "bearer",
        handle: () => ({ snapshot: "users_me_200", body: adminProfile }),
      },
      {
        method: "GET",
        path: "/api/v1/users/me/permissions",
        auth: "bearer",
        handle: () => ({ snapshot: "users_me_permissions_200", body: permsBody }),
      },
      {
        method: "GET",
        path: "/api/v1/admin/reviews",
        auth: "bearer",
        handle: () => {
          const body = structuredClone(baseList)
          body.data.items[0].adminReply = state.reply
          body.data.items[0].hidden = state.hidden
          if (state.hidden) {
            body.data.items[0].hiddenReason = state.hiddenReason
            body.data.visibleCount = 0
            body.data.hiddenCount = 2
          }
          return { snapshot: "admin_reviews_list_200", body }
        },
      },
      {
        method: "PUT",
        path: "/api/v1/admin/reviews/rev-101/reply",
        auth: "bearer",
        requestSnapshot: "admin_review_reply_200",
        handle: ({ body }) => {
          const payload = (body ?? {}) as { reply?: string }
          state.reply = payload.reply ?? ""
          const resp = structuredClone(
            snapshots.snapshots.admin_review_reply_200.responseBody as {
              data: Record<string, unknown>
              message: string
            }
          )
          resp.data.adminReply = state.reply
          return { snapshot: "admin_review_reply_200", body: resp }
        },
      },
      {
        method: "PATCH",
        path: "/api/v1/admin/reviews/rev-101/visibility",
        auth: "bearer",
        requestSnapshot: "admin_review_visibility_200",
        handle: ({ body }) => {
          const payload = (body ?? {}) as { hidden?: boolean; reason?: string }
          state.hidden = Boolean(payload.hidden)
          state.hiddenReason = payload.reason ?? ""
          const resp = structuredClone(
            snapshots.snapshots.admin_review_visibility_200.responseBody as {
              data: Record<string, unknown>
              message: string
            }
          )
          resp.data.hidden = state.hidden
          resp.data.hiddenReason = state.hiddenReason
          return { snapshot: "admin_review_visibility_200", body: resp }
        },
      },
    ]

    await mountContractRoutes(page, snapshots, routes)
    await page.context().addCookies([
      {
        name: "admin_refresh_token",
        value: "e2e-admin-refresh-cookie",
        domain: "localhost",
        path: "/api/v1/auth/admin",
        httpOnly: true,
        sameSite: "Lax",
      },
    ])

    await page.goto("/reviews")
    await expect(page.getByTestId("admin-review-row-rev-101")).toBeVisible()

    // Reply on rev-101
    const row101 = page.getByTestId("admin-review-row-rev-101")
    await row101.getByRole("button", { name: "Sửa phản hồi" }).click()
    const replyBox = page.getByLabel(/Nội dung phản hồi/)
    await replyBox.fill("DreamShop cảm ơn đánh giá chi tiết của bạn!")
    const replyResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/admin/reviews/rev-101/reply") && resp.status() === 200
    )
    await page.getByRole("button", { name: "Lưu phản hồi" }).click()
    await replyResp
    await expect(row101.getByText("DreamShop cảm ơn đánh giá chi tiết của bạn!")).toBeVisible()

    // Hide rev-101 with reason
    await row101.getByRole("button", { name: "Ẩn", exact: true }).click()
    await page.getByLabel(/Lý do ẩn/).fill("Nội dung quảng cáo không liên quan")
    const hideResp = page.waitForResponse(
      (resp) => resp.url().includes("/api/v1/admin/reviews/rev-101/visibility") && resp.status() === 200
    )
    await page.getByRole("button", { name: "Xác nhận ẩn" }).click()
    await hideResp
    await expect(page.getByTestId("kpi-hidden-reviews")).toHaveText("2")

    await page.reload()
    await expect(page.getByTestId("kpi-hidden-reviews")).toHaveText("2")
    await expect(
      page.getByTestId("admin-review-row-rev-101").getByText("DreamShop cảm ơn đánh giá chi tiết của bạn!")
    ).toBeVisible()

    await page.screenshot({
      path: path.join(IMGS_DIR, "01-admin-reviews-moderation-desktop.png"),
      fullPage: false,
    })
  })
})
