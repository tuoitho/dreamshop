import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { ApiClientError, searchProducts } from "@/lib/api"
import type { ProductSummary, SearchProductsResult } from "@/lib/types"
import { CartProvider } from "@/context/cart-provider"
import SearchPage from "./search-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
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

const EMPTY_RESULT: SearchProductsResult = {
  items: [],
  page: 1,
  pageSize: 20,
  totalItems: 0,
  totalPages: 0,
}

const PRODUCT: ProductSummary = {
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
  images: ["https://picsum.photos/seed/dt-1-1/800/800"],
  badge: "hot",
}

function renderSearch(initialEntry = "/tim-kiem") {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <CartProvider>
        <SearchPage />
      </CartProvider>
    </MemoryRouter>
  )
}

describe("SearchPage", () => {
  beforeEach(() => {
    vi.mocked(searchProducts).mockReset()
    vi.mocked(searchProducts).mockResolvedValue(EMPTY_RESULT)
  })

  it("derives storefront params from the URL query string", async () => {
    renderSearch("/tim-kiem?q=nova&danh-muc=dien-thoai&gia=500000-2000000&giam-gia=1&danh-gia=4&sap-xep=gia-thap")

    await screen.findByText("Không tìm thấy sản phẩm")

    expect(searchProducts).toHaveBeenCalledWith(
      {
        q: "nova",
        categoryId: "dien-thoai",
        priceMin: 500000,
        priceMax: 2000000,
        onSale: true,
        ratingMin: 4,
        sort: "gia-thap",
        page: 1,
        pageSize: 20,
      },
      expect.anything()
    )
  })

  it("maps an unbounded price range to priceMin only", async () => {
    renderSearch("/tim-kiem?gia=10000000-0")

    await screen.findByText("Không tìm thấy sản phẩm")

    const params = vi.mocked(searchProducts).mock.calls[0][0]
    expect(params).toMatchObject({ priceMin: 10000000, sort: "pho-bien", page: 1 })
    expect(params).not.toHaveProperty("priceMax")
  })

  it("shows a loading skeleton then the product results", async () => {
    let resolve!: (value: SearchProductsResult) => void
    vi.mocked(searchProducts).mockReturnValueOnce(
      new Promise((res) => {
        resolve = res
      })
    )
    renderSearch()

    expect(screen.getByLabelText("Đang tải sản phẩm")).toBeInTheDocument()

    resolve({ ...EMPTY_RESULT, items: [PRODUCT], totalItems: 1, totalPages: 1 })

    expect(await screen.findByText(PRODUCT.name)).toBeInTheDocument()
    expect(screen.getByText("1 sản phẩm")).toBeInTheDocument()
  })

  it("shows the empty state when the server returns no products", async () => {
    vi.mocked(searchProducts).mockResolvedValue(EMPTY_RESULT)
    renderSearch("/tim-kiem?q=xyz")

    expect(await screen.findByText("Không tìm thấy sản phẩm")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Xóa toàn bộ bộ lọc" })).toBeInTheDocument()
  })

  it("shows an error message and retries the request", async () => {
    vi.mocked(searchProducts).mockRejectedValueOnce(new ApiClientError(500, "Máy chủ đang bận"))
    vi.mocked(searchProducts).mockResolvedValue({ ...EMPTY_RESULT, items: [PRODUCT], totalItems: 1, totalPages: 1 })
    renderSearch()

    expect(await screen.findByRole("alert")).toHaveTextContent("Máy chủ đang bận")

    await userEvent.click(screen.getByRole("button", { name: "Thử lại" }))

    expect(await screen.findByText(PRODUCT.name)).toBeInTheDocument()
    expect(searchProducts).toHaveBeenCalledTimes(2)
  })

  it("shows pagination controls when more than one page exists", async () => {
    vi.mocked(searchProducts).mockResolvedValue({
      items: [PRODUCT],
      page: 1,
      pageSize: 20,
      totalItems: 25,
      totalPages: 2,
    })
    renderSearch()

    expect(await screen.findByText("Trang 1 / 2")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Trang trước" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Trang sau" })).toBeEnabled()
  })

  it("preserves original user casing in heading while sending lowercase q to searchProducts", async () => {
    renderSearch("/tim-kiem?q=iPhone%2015%20Pro")

    await screen.findByText("Không tìm thấy sản phẩm")

    expect(
      screen.getByRole("heading", { level: 1, name: 'Kết quả cho "iPhone 15 Pro"' })
    ).toBeInTheDocument()
    expect(vi.mocked(searchProducts).mock.calls[0][0].q).toBe("iphone 15 pro")
  })

  it("renders 'Sản phẩm khuyến mãi' heading when filtering on-sale items without q or category", async () => {
    renderSearch("/tim-kiem?giam-gia=1")

    await screen.findByText("Không tìm thấy sản phẩm")

    expect(
      screen.getByRole("heading", { level: 1, name: "Sản phẩm khuyến mãi" })
    ).toBeInTheDocument()
  })

  it("synchronizes price slider display with URL gia param and resets when clearing filters", async () => {
    renderSearch("/tim-kiem?gia=2000000-8000000")

    await screen.findByText("Không tìm thấy sản phẩm")
    expect(screen.getByText("2.0tr - 8.0tr")).toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: "Xóa toàn bộ bộ lọc" }))
    expect(await screen.findByText("0.0tr - 10.0tr")).toBeInTheDocument()
  })
})
