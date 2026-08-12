import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom"
import { ApiClientError, addCartItem, getCart, getProduct, searchProducts, softDeleteProduct } from "@/lib/api"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import type { CartItem, ProductDetail, ProductSummary } from "@/lib/types"
import { CartProvider } from "@/context/cart-provider"
import { useCart } from "@/hooks/use-cart"
import ProductDetailPage from "./product-detail-page"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    getProduct: vi.fn(),
    searchProducts: vi.fn(),
    addCartItem: vi.fn(),
    getCart: vi.fn(),
    softDeleteProduct: vi.fn(),
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
    { label: "16GB/512GB" },
    { label: "32GB/1TB" },
  ],
}

const CART_ITEM: CartItem = {
  itemId: "cart-item-1",
  productId: DETAIL.id,
  variantLabel: "16GB/512GB",
  quantity: 1,
  name: DETAIL.name,
  brand: DETAIL.brand,
  categoryName: DETAIL.categoryName,
  unitPrice: DETAIL.price,
  imageEmoji: DETAIL.emoji!,
  imageHue: DETAIL.imageHue!,
  imageUrl: DETAIL.images![0],
  status: "AVAILABLE",
  availableQuantity: 10,
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

function authValue(overrides: Partial<AuthContextValue> = {}): AuthContextValue {
  return {
    token: "token",
    user: {
      userId: "user-1",
      email: "user@example.com",
      fullName: "Test User",
      roles: ["USER"],
      phone: null,
      dateOfBirth: null,
      gender: null,
      provinceCode: null,
      wardCode: null,
      addressLine: null,
    },
    ready: true,
    isAuthenticated: true,
    login: vi.fn(),
    register: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn(),
    ...overrides,
  }
}

function CheckoutProbe() {
  const { selectedItems } = useCart()
  return <output data-testid="checkout-items">{selectedItems.map((item) => item.itemId).join(",")}</output>
}

function LoginProbe() {
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from
  return <output data-testid="login-from">{from}</output>
}

function renderDetail(id = "dt-1", auth: Partial<AuthContextValue> = {}, destination = "product") {
  return render(
    <MemoryRouter initialEntries={[`/san-pham/${id}`]}>
      <AuthContext value={authValue(auth)}>
        <CartProvider>
          <Routes>
            <Route path="/san-pham/:id" element={<ProductDetailPage />} />
            {destination === "checkout" && <Route path="/gio-hang/dat-hang" element={<CheckoutProbe />} />}
            {destination === "login" && <Route path="/login" element={<LoginProbe />} />}
          </Routes>
        </CartProvider>
      </AuthContext>
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
    vi.mocked(getCart).mockResolvedValue({ items: [], degraded: false })
    vi.mocked(addCartItem).mockResolvedValue({ items: [CART_ITEM], degraded: false })
    vi.mocked(softDeleteProduct).mockResolvedValue({ id: DETAIL.id, deleted: true })
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

  it("shows a color swatch when a variant provides colorHex", async () => {
    vi.mocked(getProduct).mockResolvedValue({
      ...DETAIL,
      variants: [{ label: "Đen", colorHex: "#1f2937" }],
    })
    renderDetail()

    const variantButton = await screen.findByRole("button", { name: "Đen" })
    expect(variantButton.querySelector("span")).toHaveStyle({ backgroundColor: "#1f2937" })
  })

  it("keeps a capacity variant usable without a color swatch", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail()

    expect(await screen.findByText("Phân loại")).toBeInTheDocument()
    const variantButton = screen.getByRole("button", { name: "16GB/512GB" })
    expect(variantButton).toHaveTextContent("16GB/512GB")
    expect(variantButton.querySelector("span")).not.toBeInTheDocument()
  })

  it("redirects unauthenticated add-to-cart attempts to login with the product origin", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail("dt-1", { token: null, user: null, isAuthenticated: false }, "login")

    await userEvent.click(await screen.findByRole("button", { name: "Thêm vào giỏ" }))

    expect(await screen.findByTestId("login-from")).toHaveTextContent("/san-pham/dt-1")
    expect(addCartItem).not.toHaveBeenCalled()
  })

  it("adds the selected classification to the cart for authenticated users", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail()

    await userEvent.click(await screen.findByRole("button", { name: "Thêm vào giỏ" }))

    await waitFor(() => expect(addCartItem).toHaveBeenCalledWith({
      productId: "dt-1",
      variantLabel: "16GB/512GB",
      quantity: 1,
    }))
    expect(await screen.findByRole("button", { name: "Đã thêm vào giỏ" })).toBeInTheDocument()
  })

  it("buys now by selecting the added cart item and opening checkout", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail("dt-1", {}, "checkout")

    await userEvent.click(await screen.findByRole("button", { name: "Mua ngay" }))

    expect(await screen.findByTestId("checkout-items")).toHaveTextContent("cart-item-1")
    expect(addCartItem).toHaveBeenCalledWith({
      productId: "dt-1",
      variantLabel: "16GB/512GB",
      quantity: 1,
    })
  })

  it("shows the development soft-delete control and reloads after deleting", async () => {
    vi.stubEnv("DEV", true)
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail()

    await userEvent.click(await screen.findByRole("button", { name: "Xóa mềm sản phẩm" }))

    expect(softDeleteProduct).toHaveBeenCalledWith("dt-1", true)
    await waitFor(() => expect(getProduct).toHaveBeenCalledTimes(2))
    vi.unstubAllEnvs()
  })

  it("labels the development control as restore for a deleted product", async () => {
    vi.stubEnv("DEV", true)
    vi.mocked(getProduct).mockResolvedValue({ ...DETAIL, deleted: true })
    renderDetail()

    expect(await screen.findByRole("button", { name: "Khôi phục sản phẩm" })).toBeInTheDocument()
    vi.unstubAllEnvs()
  })
})
