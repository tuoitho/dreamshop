import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { ReviewsPage } from "./reviews-page"
import * as api from "@/lib/api"

vi.mock("@/lib/api", () => ({
  listAdminReviews: vi.fn(),
  setAdminReviewVisibility: vi.fn(),
  replyAdminReview: vi.fn(),
}))

vi.mock("@/hooks/use-auth", () => ({
  useAuth: () => ({
    can: (perm: string) => perm === "review.admin.view" || perm === "review.admin.moderate",
  }),
}))

const SAMPLE_REVIEW = {
  id: "rev-101",
  productId: "dt-1",
  productName: "Điện thoại Nova X",
  userId: "user-1",
  orderId: "ord-123",
  skuId: "dt-1-v1",
  variantLabel: "256GB · Tiêu chuẩn",
  authorName: "Customer One",
  rating: 5,
  comment: "Máy chạy mượt, đóng gói cẩn thận.",
  images: [],
  verifiedPurchase: true,
  hidden: false,
  createdAt: "2026-11-26T05:00:00Z",
  updatedAt: "2026-11-26T05:00:00Z",
}

describe("ReviewsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listAdminReviews).mockResolvedValue({
      items: [SAMPLE_REVIEW],
      page: 1,
      pageSize: 20,
      totalItems: 1,
      totalPages: 1,
      visibleCount: 1,
      hiddenCount: 0,
      unrepliedCount: 1,
      averageRating: 5,
    })
    vi.mocked(api.replyAdminReview).mockResolvedValue({
      ...SAMPLE_REVIEW,
      adminReply: "Cảm ơn bạn!",
      adminRepliedAt: "2026-11-26T06:00:00Z",
    })
    vi.mocked(api.setAdminReviewVisibility).mockResolvedValue({
      ...SAMPLE_REVIEW,
      hidden: true,
      hiddenReason: "Spam",
    })
  })

  it("renders KPIs, allows replying and hiding a review", async () => {
    render(<ReviewsPage />)

    expect(await screen.findByText("Máy chạy mượt, đóng gói cẩn thận.")).toBeInTheDocument()
    expect(screen.getByTestId("kpi-visible-reviews")).toHaveTextContent("1")
    expect(screen.getByTestId("kpi-unreplied-reviews")).toHaveTextContent("1")

    await userEvent.click(screen.getByRole("button", { name: "Phản hồi" }))
    await userEvent.type(screen.getByLabelText(/Nội dung phản hồi/), "Cảm ơn bạn!")
    await userEvent.click(screen.getByRole("button", { name: "Lưu phản hồi" }))
    await waitFor(() => {
      expect(api.replyAdminReview).toHaveBeenCalledWith("rev-101", "Cảm ơn bạn!")
    })

    await userEvent.click(screen.getByRole("button", { name: "Ẩn" }))
    await userEvent.type(screen.getByLabelText(/Lý do ẩn/), "Spam")
    await userEvent.click(screen.getByRole("button", { name: "Xác nhận ẩn" }))
    await waitFor(() => {
      expect(api.setAdminReviewVisibility).toHaveBeenCalledWith("rev-101", {
        hidden: true,
        reason: "Spam",
      })
    })
  })
})
