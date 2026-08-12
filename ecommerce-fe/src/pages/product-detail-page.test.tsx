import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { ApiClientError, getProduct, searchProducts } from "@/lib/api"
import type { ProductDetail, ProductSummary } from "@/lib/types"
import { CartProvider } from "@/context/cart-provider"
import ProductDetailPage from "./product-detail-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    getProduct: vi.fn(),
    searchProducts: vi.fn(),
  }
})

beforeAll(() => {
  class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  window.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver
})

const DETAIL: ProductDetail = {
  id: "dt-1",
  name: "Điện thoại thông minh Nova X15 Pro 5G",
  brand: "Nova",
  categoryId: "dien-thoai",
  categoryName: "Điện thoại",
  price: 18990000,
  originalPrice: 21990000,
  rating: 4.9,
  reviewCount: 1284,
  soldCount: 2310,
  images: [
    "https://picsum.photos/seed/dt-1-1/800/800",
    "https://picsum.photos/seed/dt-1-2/800/800",
  ],
  emoji: "📱",
  imageHue: [220, 260],
  badge: "hot",
  shortDescription: "Chip xử lý mạnh mẽ, camera 200MP.",
  description: "Mô tả chi tiết sản phẩm Nova X15 Pro.",
  details: ['Màn hình: 6.8" AMOLED 144Hz', "Pin: 5500mAh, sạc nhanh 120W"],
  variants: [
    { label: "Đen", colorHex: "#1f2937" },
    { label: "Xanh", colorHex: "#1e40af" },
  ],
}

const RELATED: ProductSummary[] = [
  {
    id: "dt-2",
    name: "Điện thoại thông minh Lotus 12A",
    brand: "Lotus",
    categoryId: "dien-thoai",
    categoryName: "Điện thoại",
    price: 5490000,
    originalPrice: 6990000,
    rating: 4.7,
    reviewCount: 3521,
    soldCount: 8920,
    emoji: "📲",
  },
]

function renderDetail(id = "dt-1") {
  return render(
    <MemoryRouter initialEntries={[`/san-pham/${id}`]}>
      <CartProvider>
        <Routes>
          <Route path="/san-pham/:id" element={<ProductDetailPage />} />
        </Routes>
      </CartProvider>
    </MemoryRouter>
  )
}

describe("ProductDetailPage", () => {
  beforeEach(() => {
    vi.mocked(getProduct).mockReset()
    vi.mocked(searchProducts).mockReset()
    vi.mocked(searchProducts).mockResolvedValue({
      items: [],
      page: 1,
      pageSize: 20,
      totalItems: 0,
      totalPages: 0,
    })
  })

  it("shows a loading skeleton then the product content", async () => {
    let resolve!: (value: ProductDetail) => void
    vi.mocked(getProduct).mockReturnValueOnce(
      new Promise((res) => {
        resolve = res
      })
    )
    renderDetail()

    expect(screen.getByLabelText("Đang tải sản phẩm")).toBeInTheDocument()

    resolve(DETAIL)

    expect(await screen.findByRole("heading", { level: 1, name: DETAIL.name })).toBeInTheDocument()
    expect(screen.getByText(DETAIL.shortDescription!)).toBeInTheDocument()
    expect(screen.getByText(DETAIL.details[0])).toBeInTheDocument()
  })

  it("switches the main image when a thumbnail is selected", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail()

    const mainImage = await screen.findByAltText(DETAIL.name)
    expect(mainImage).toHaveAttribute("src", DETAIL.images![0])

    await userEvent.click(screen.getByRole("button", { name: "Xem ảnh 2" }))

    await waitFor(() => expect(mainImage).toHaveAttribute("src", DETAIL.images![1]))
  })

  it("falls back to the emoji placeholder when the image fails to load", async () => {
    vi.mocked(getProduct).mockResolvedValue({ ...DETAIL, images: undefined })
    renderDetail()

    expect(await screen.findByRole("heading", { level: 1, name: DETAIL.name })).toBeInTheDocument()
    expect(screen.getByRole("img", { name: DETAIL.name })).toBeInTheDocument()
  })

  it("renders related products from the same category and excludes the current product", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    vi.mocked(searchProducts).mockResolvedValue({
      items: [{ ...DETAIL }, ...RELATED],
      page: 1,
      pageSize: 4,
      totalItems: 2,
      totalPages: 1,
    })
    renderDetail()

    expect(await screen.findByText("Sản phẩm tương tự")).toBeInTheDocument()

    await waitFor(() => {
      expect(searchProducts).toHaveBeenCalledWith({
        categoryId: "dien-thoai",
        pageSize: 4,
        sort: "pho-bien",
      })
    })

    const section = screen.getByRole("heading", { name: "Sản phẩm tương tự" }).closest("section")!
    expect(within(section).getByText(RELATED[0].name)).toBeInTheDocument()
    expect(within(section).queryByText(DETAIL.name)).not.toBeInTheDocument()
  })

  it("renders the not-found state for PRODUCT_NOT_FOUND", async () => {
    vi.mocked(getProduct).mockRejectedValue(
      new ApiClientError(404, "Product not found", "PRODUCT_NOT_FOUND", "t-1")
    )
    renderDetail("missing")

    expect(await screen.findByText("Không tìm thấy sản phẩm")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Về trang chủ" })).toBeInTheDocument()
  })

  it("shows an error message and retries the request", async () => {
    vi.mocked(getProduct).mockRejectedValueOnce(new ApiClientError(500, "Máy chủ đang bận"))
    vi.mocked(getProduct).mockResolvedValueOnce(DETAIL)
    renderDetail()

    expect(await screen.findByRole("alert")).toHaveTextContent("Máy chủ đang bận")

    await userEvent.click(screen.getByRole("button", { name: "Thử lại" }))

    expect(await screen.findByRole("heading", { level: 1, name: DETAIL.name })).toBeInTheDocument()
    expect(getProduct).toHaveBeenCalledTimes(2)
  })

  it("caps the quantity stepper at the purchase limit", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail()

    const increase = await screen.findByRole("button", { name: "Tăng số lượng" })
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)
    await userEvent.click(increase)

    expect(screen.getByText("10")).toBeInTheDocument()
    expect(screen.getByText("Mua tối đa 10 sản phẩm mỗi lần")).toBeInTheDocument()
  })
})
