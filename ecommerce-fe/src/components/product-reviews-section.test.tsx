import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { ProductReviewsSection } from "./product-reviews-section"
import * as api from "@/lib/api"

vi.mock("@/lib/api", () => ({
  listProductReviews: vi.fn(),
  getReviewEligibility: vi.fn(),
  createProductReview: vi.fn(),
  updateProductReview: vi.fn(),
  deleteProductReview: vi.fn(),
  uploadReviewMediaFile: vi.fn(),
}))

vi.mock("@/hooks/use-auth", () => ({
  useAuth: () => ({
    user: { userId: "user-1", fullName: "Customer One", email: "customer1@gmail.com", roles: ["CUSTOMER"] },
    isAuthenticated: true,
    ready: true,
    token: "mock-token",
  }),
}))

const SAMPLE_REVIEW = {
  id: "rev-1",
  productId: "dt-1",
  productName: "Nova X",
  userId: "user-2",
  orderId: "ord-2",
  skuId: "dt-1-v1",
  variantLabel: "256GB · Tiêu chuẩn",
  authorName: "Nguyễn Văn B",
  rating: 5,
  comment: "Sản phẩm rất tốt, giao hàng nhanh.",
  images: [],
  verifiedPurchase: true,
  hidden: false,
  adminReply: "Cảm ơn quý khách đã ủng hộ DreamShop!",
  adminRepliedAt: "2026-11-26T05:00:00Z",
  createdAt: "2026-11-26T04:00:00Z",
  updatedAt: "2026-11-26T04:00:00Z",
}

describe("ProductReviewsSection", () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listProductReviews).mockResolvedValue({
      items: [SAMPLE_REVIEW],
      page: 1,
      pageSize: 10,
      totalItems: 1,
      totalPages: 1,
      summary: {
        averageRating: 5,
        totalReviews: 1,
        breakdown: [
          { rating: 5, count: 1 },
          { rating: 4, count: 0 },
          { rating: 3, count: 0 },
          { rating: 2, count: 0 },
          { rating: 1, count: 0 },
        ],
      },
    })
    vi.mocked(api.getReviewEligibility).mockResolvedValue({
      canReview: true,
      eligibleOrders: [
        {
          orderId: "ord-1",
          orderCode: "ORD-20261126-001",
          skuId: "dt-1-v1",
          variantLabel: "256GB · Tiêu chuẩn",
          deliveredAt: "2026-11-26T03:00:00Z",
          reviewed: false,
        },
      ],
      myReviews: [],
    })
    vi.mocked(api.createProductReview).mockResolvedValue({
      review: { ...SAMPLE_REVIEW, id: "rev-new", userId: "user-1", orderId: "ord-1", authorName: "Customer One", comment: "Rất hài lòng" },
      summary: {
        averageRating: 5,
        totalReviews: 2,
        breakdown: [{ rating: 5, count: 2 }, { rating: 4, count: 0 }, { rating: 3, count: 0 }, { rating: 2, count: 0 }, { rating: 1, count: 0 }],
      },
    })
  })

  it("renders review summary, verified review, admin reply, and allows eligible user to submit a review", async () => {
    const onSummaryChange = vi.fn()
    render(
      <MemoryRouter initialEntries={["/san-pham/dt-1"]}>
        <ProductReviewsSection
          productId="dt-1"
          initialRating={5}
          initialReviewCount={1}
          onSummaryChange={onSummaryChange}
        />
      </MemoryRouter>
    )

    expect(await screen.findByText("Sản phẩm rất tốt, giao hàng nhanh.")).toBeInTheDocument()
    expect(screen.getByText("Đã mua hàng")).toBeInTheDocument()
    expect(screen.getByText("Phản hồi từ DreamShop")).toBeInTheDocument()
    expect(screen.getByText("Cảm ơn quý khách đã ủng hộ DreamShop!")).toBeInTheDocument()

    await userEvent.click(screen.getByTestId("open-review-composer"))
    await userEvent.type(screen.getByLabelText("Nhận xét của bạn"), "Rất hài lòng")
    await userEvent.click(screen.getByRole("button", { name: "Gửi đánh giá" }))

    await waitFor(() => {
      expect(api.createProductReview).toHaveBeenCalledWith({
        productId: "dt-1",
        orderId: "ord-1",
        authorName: "Customer One",
        rating: 5,
        comment: "Rất hài lòng",
        images: [],
      })
    })
    expect(onSummaryChange).toHaveBeenCalledWith(expect.objectContaining({ totalReviews: 2 }))
  })
})
