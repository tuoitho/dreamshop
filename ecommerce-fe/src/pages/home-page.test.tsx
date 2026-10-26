import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { ApiClientError, searchProducts } from "@/lib/api"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import type { ProductSummary, SearchProductsResult } from "@/lib/types"
import HomePage from "./home-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    searchProducts: vi.fn(),
  }
})

const LIVE_PRODUCTS: ProductSummary[] = [
  {
    id: "dt-live-1",
    name: "Điện thoại Live Flagship 5G",
    brand: "Nova",
    categoryId: "dien-thoai",
    categoryName: "Điện thoại",
    price: 15990000,
    originalPrice: 18990000,
    rating: 4.9,
    reviewCount: 420,
    soldCount: 1500,
    featured: true,
    status: "AVAILABLE",
    availableQuantity: 20,
  },
  {
    id: "lt-live-1",
    name: "Laptop Live Studio 14",
    brand: "MaxBook",
    categoryId: "laptop",
    categoryName: "Laptop",
    price: 22990000,
    originalPrice: 22990000,
    rating: 4.8,
    reviewCount: 180,
    soldCount: 600,
    status: "LIMITED",
    availableQuantity: 3,
  },
]

function makeAuth(isAuthenticated: boolean): AuthContextValue {
  return {
    token: isAuthenticated ? "token" : null,
    user: isAuthenticated
      ? {
          userId: "u-1",
          email: "member@example.com",
          fullName: "Thành Viên",
          roles: ["USER"],
          phone: null,
          dateOfBirth: null,
          gender: null,
          provinceCode: null,
          wardCode: null,
          addressLine: null,
        }
      : null,
    ready: true,
    isAuthenticated,
    login: vi.fn(),
    register: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn(),
  }
}

function renderHome(isAuthenticated = false) {
  return render(
    <MemoryRouter>
      <AuthContext value={makeAuth(isAuthenticated)}>
        <HomePage />
      </AuthContext>
    </MemoryRouter>
  )
}

describe("HomePage", () => {
  beforeEach(() => {
    vi.mocked(searchProducts).mockReset()
  })

  it("loads products from searchProducts API, shows skeleton first, and renders dynamic hero starting price", async () => {
    let resolveSearch!: (value: SearchProductsResult) => void
    vi.mocked(searchProducts).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSearch = resolve
      })
    )

    renderHome(false)

    expect(screen.getByRole("status", { name: "Đang tải sản phẩm trang chủ" })).toBeInTheDocument()
    expect(searchProducts).toHaveBeenCalledWith(
      { sort: "pho-bien", pageSize: 20 },
      expect.anything()
    )

    resolveSearch({
      items: LIVE_PRODUCTS,
      page: 1,
      pageSize: 20,
      totalItems: 2,
      totalPages: 1,
    })

    expect((await screen.findAllByText("Điện thoại Live Flagship 5G")).length).toBeGreaterThan(0)
    expect(screen.queryByText("3.450.000đ")).not.toBeInTheDocument()
    expect(screen.getAllByText(/15\.990\.000/).length).toBeGreaterThan(0)
    expect(screen.getByRole("link", { name: "Đăng ký miễn phí" })).toHaveAttribute(
      "href",
      "/register"
    )
  })

  it("filters featured section by category button with aria-pressed", async () => {
    vi.mocked(searchProducts).mockResolvedValue({
      items: LIVE_PRODUCTS,
      page: 1,
      pageSize: 20,
      totalItems: 2,
      totalPages: 1,
    })

    renderHome(false)
    await screen.findAllByText("Laptop Live Studio 14")

    const featuredSection = screen
      .getByRole("heading", { name: "Sản phẩm nổi bật" })
      .closest("section")!
    const laptopTab = within(featuredSection).getByRole("button", { name: "Laptop" })
    expect(laptopTab).toHaveAttribute("aria-pressed", "false")

    await userEvent.click(laptopTab)

    expect(laptopTab).toHaveAttribute("aria-pressed", "true")
    expect(within(featuredSection).getByText("Laptop Live Studio 14")).toBeInTheDocument()
    expect(within(featuredSection).queryByText("Điện thoại Live Flagship 5G")).not.toBeInTheDocument()
  })

  it("displays an inline error alert and retries searchProducts when API fails", async () => {
    vi.mocked(searchProducts)
      .mockRejectedValueOnce(new ApiClientError(502, "Catalog tạm thời gián đoạn"))
      .mockResolvedValueOnce({
        items: LIVE_PRODUCTS,
        page: 1,
        pageSize: 20,
        totalItems: 2,
        totalPages: 1,
      })

    renderHome(false)

    expect(await screen.findByRole("alert")).toHaveTextContent("Catalog tạm thời gián đoạn")
    await userEvent.click(screen.getByRole("button", { name: "Thử lại" }))

    expect((await screen.findAllByText("Điện thoại Live Flagship 5G")).length).toBeGreaterThan(0)
    expect(searchProducts).toHaveBeenCalledTimes(2)
  })

  it("adapts bottom CTA banner for authenticated members", async () => {
    vi.mocked(searchProducts).mockResolvedValue({
      items: LIVE_PRODUCTS,
      page: 1,
      pageSize: 20,
      totalItems: 2,
      totalPages: 1,
    })

    renderHome(true)
    await screen.findAllByText("Điện thoại Live Flagship 5G")

    expect(screen.queryByRole("link", { name: "Đăng ký miễn phí" })).not.toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Khám phá ưu đãi hôm nay" })).toHaveAttribute(
      "href",
      "/tim-kiem?giam-gia=1"
    )
  })
})
