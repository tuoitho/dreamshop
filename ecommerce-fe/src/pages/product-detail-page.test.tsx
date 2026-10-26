import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { act, render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom"
import { ApiClientError, addCartItem, getCart, getProduct, searchProducts, softDeleteProduct } from "@/lib/api"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import type { CartItem, ProductDetail, ProductSummary } from "@/lib/types"
import { CartProvider } from "@/context/cart-provider"
import { useCart } from "@/hooks/use-cart"
import { toast } from "sonner"
import ProductDetailPage from "./product-detail-page"

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

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

const OTHER_CART_ITEM: CartItem = {
  ...CART_ITEM,
  itemId: "cart-item-other",
  productId: "other-product",
  variantLabel: "Other",
  name: "Other product",
}

function cartItem(overrides: Partial<CartItem> = {}): CartItem {
  return { ...CART_ITEM, ...overrides }
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

function RouteSwitchProbe({ target }: { target: string }) {
  const navigate = useNavigate()
  return <button onClick={() => navigate(target)}>Chuyển trang</button>
}

function cartValue(items: CartItem[], addItem: CartContextValue["addItem"]): CartContextValue {
  const selectedItems = items.filter((item) => item.status === "AVAILABLE" || item.status === "LIMITED")
  return {
    items,
    addItem,
    updateQuantity: vi.fn(),
    removeItem: vi.fn(),
    removeSelected: vi.fn(),
    clearCart: vi.fn(),
    toggleSelected: vi.fn(),
    selectOnly: vi.fn(),
    toggleAllSelected: vi.fn(),
    selectedItems,
    selectedCount: selectedItems.reduce((total, item) => total + item.quantity, 0),
    totalQuantity: items.reduce((total, item) => total + item.quantity, 0),
    selectedTotal: selectedItems.reduce((total, item) => total + item.unitPrice * item.quantity, 0),
    allSelected: selectedItems.length > 0,
    loading: false,
    degraded: false,
    loadError: null,
    retryLoad: vi.fn(),
  }
}

function renderDetail(
  id = "dt-1",
  auth: Partial<AuthContextValue> = {},
  destination = "product",
  switchTarget = "/san-pham/dt-2",
  controlledCart?: CartContextValue
) {
  const routes = (
    <Routes>
      <Route
        path="/san-pham/:id"
        element={
          <>
            <RouteSwitchProbe target={switchTarget} />
            <ProductDetailPage />
          </>
        }
      />
      {destination === "checkout" && <Route path="/gio-hang/dat-hang" element={<CheckoutProbe />} />}
      {destination === "login" && <Route path="/login" element={<LoginProbe />} />}
    </Routes>
  )
  return render(
    <MemoryRouter initialEntries={[`/san-pham/${id}`]}>
      <AuthContext value={authValue(auth)}>
        {controlledCart ? <CartContext value={controlledCart}>{routes}</CartContext> : <CartProvider>{routes}</CartProvider>}
      </AuthContext>
    </MemoryRouter>
  )
}

describe("ProductDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks()
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

  afterEach(() => {
    vi.unstubAllEnvs()
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

  it("buys now by selecting only the requested purchasable cart item", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    vi.mocked(getCart).mockResolvedValue({ items: [OTHER_CART_ITEM], degraded: false })
    const limitedRequested = cartItem({ itemId: "cart-item-limited", status: "LIMITED", availableQuantity: 2 })
    const sameProductDistractor = cartItem({ itemId: "cart-item-distractor", variantLabel: "32GB/1TB" })
    vi.mocked(addCartItem).mockResolvedValue({
      items: [OTHER_CART_ITEM, sameProductDistractor, limitedRequested],
      degraded: false,
    })
    renderDetail("dt-1", {}, "checkout")

    await userEvent.click(await screen.findByRole("button", { name: "Mua ngay" }))

    expect(await screen.findByTestId("checkout-items")).toHaveTextContent("cart-item-limited")
    expect(screen.getByTestId("checkout-items")).not.toHaveTextContent("cart-item-other")
    expect(screen.getByTestId("checkout-items")).not.toHaveTextContent("cart-item-distractor")
    expect(addCartItem).toHaveBeenCalledWith({
      productId: "dt-1",
      variantLabel: "16GB/512GB",
      quantity: 1,
    })
  })

  it("uses the captured variant after the user changes selection while Buy Now waits", async () => {
    let resolveAdd!: (value: { items: CartItem[]; degraded: boolean }) => void
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    vi.mocked(addCartItem).mockReturnValueOnce(new Promise((resolve) => { resolveAdd = resolve }))
    renderDetail("dt-1", {}, "checkout")

    const buyNow = await screen.findByRole("button", { name: "Mua ngay" })
    await userEvent.click(buyNow)
    await userEvent.click(screen.getByRole("button", { name: "32GB/1TB" }))
    resolveAdd({ items: [CART_ITEM], degraded: false })

    expect(await screen.findByTestId("checkout-items")).toHaveTextContent("cart-item-1")
    expect(addCartItem).toHaveBeenCalledWith({
      productId: "dt-1",
      variantLabel: "16GB/512GB",
      quantity: 1,
    })
  })

  it.each([
    {
      label: "an out-of-stock matching row",
      cartResponse: { items: [cartItem({ status: "OUT_OF_STOCK", availableQuantity: 0 })], degraded: false },
      expectedError: "Sản phẩm vừa tạm hết hàng",
      expectReload: true,
    },
    {
      label: "an unavailable matching row",
      cartResponse: { items: [cartItem({ status: "UNAVAILABLE", availableQuantity: 0 })], degraded: false },
      expectedError: "Sản phẩm đã ngừng kinh doanh",
      expectReload: true,
    },
    {
      label: "degraded inventory during Buy Now",
      cartResponse: { items: [cartItem({ status: "UNAVAILABLE", availableQuantity: 0 })], degraded: true },
      expectedError: "Tạm thời chưa kiểm tra được tồn kho, vui lòng kiểm tra giỏ hàng",
      expectReload: false,
    },
    {
      label: "no matching row",
      cartResponse: { items: [OTHER_CART_ITEM], degraded: false },
      expectedError: "Không tìm thấy phân loại có thể mua trong giỏ hàng",
      expectReload: false,
    },
  ])("does not navigate for Buy Now when there is $label", async ({ cartResponse, expectedError, expectReload }) => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    vi.mocked(addCartItem).mockResolvedValue(cartResponse)
    renderDetail("dt-1", {}, "checkout")

    await userEvent.click(await screen.findByRole("button", { name: "Mua ngay" }))

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expectedError))
    expect(screen.queryByTestId("checkout-items")).not.toBeInTheDocument()
    await waitFor(() => expect(getProduct).toHaveBeenCalledTimes(expectReload ? 2 : 1))
  })

  it("keeps Add to Cart pending, prevents duplicates, and reports rejection locally", async () => {
    let rejectAdd!: (error: Error) => void
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    vi.mocked(addCartItem).mockReturnValueOnce(new Promise((_, reject) => { rejectAdd = reject }))
    renderDetail()

    const addButton = await screen.findByRole("button", { name: "Thêm vào giỏ" })
    await userEvent.click(addButton)
    await userEvent.click(addButton)
    expect(addCartItem).toHaveBeenCalledTimes(1)
    expect(addButton).toBeDisabled()

    rejectAdd(new Error("Cart unavailable"))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Cart unavailable"))
    expect(screen.getByRole("button", { name: "Thêm vào giỏ" })).not.toBeDisabled()
  })

  it("reports Buy Now rejection locally and leaves checkout unopened", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    vi.mocked(addCartItem).mockRejectedValueOnce(new Error("Buy unavailable"))
    renderDetail("dt-1", {}, "checkout")

    const buyButton = await screen.findByRole("button", { name: "Mua ngay" })
    await userEvent.click(buyButton)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Buy unavailable"))
    expect(screen.queryByTestId("checkout-items")).not.toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Mua ngay" })).not.toBeDisabled()
  })

  it("ignores a stale Add to Cart completion after switching products", async () => {
    let resolveAdd!: (value: { items: CartItem[]; degraded: boolean }) => void
    const detail2 = { ...DETAIL, id: "dt-2", name: "Lotus 12A" }
    vi.mocked(getProduct).mockResolvedValueOnce(DETAIL).mockResolvedValueOnce(detail2)
    vi.mocked(addCartItem).mockReturnValueOnce(new Promise((resolve) => { resolveAdd = resolve }))
    renderDetail()

    await userEvent.click(await screen.findByRole("button", { name: "Thêm vào giỏ" }))
    await userEvent.click(screen.getByRole("button", { name: "Chuyển trang" }))
    expect(await screen.findByRole("heading", { level: 1, name: detail2.name })).toBeInTheDocument()

    resolveAdd({ items: [CART_ITEM], degraded: false })
    await waitFor(() => expect(screen.getByRole("heading", { level: 1, name: detail2.name })).toBeInTheDocument())
    expect(screen.queryByRole("button", { name: "Đã thêm vào giỏ" })).not.toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Thêm vào giỏ" })).not.toBeDisabled()
  })

  it("ignores a stale Buy Now completion after switching products", async () => {
    let resolveAdd!: (value: { items: CartItem[]; degraded: boolean }) => void
    const detail2 = { ...DETAIL, id: "dt-2", name: "Lotus 12A" }
    vi.mocked(getProduct).mockResolvedValueOnce(DETAIL).mockResolvedValueOnce(detail2)
    vi.mocked(addCartItem).mockReturnValueOnce(new Promise((resolve) => { resolveAdd = resolve }))
    renderDetail("dt-1", {}, "checkout")

    await userEvent.click(await screen.findByRole("button", { name: "Mua ngay" }))
    await userEvent.click(screen.getByRole("button", { name: "Chuyển trang" }))
    expect(await screen.findByRole("heading", { level: 1, name: detail2.name })).toBeInTheDocument()

    resolveAdd({ items: [CART_ITEM], degraded: false })
    await waitFor(() => expect(screen.getByRole("heading", { level: 1, name: detail2.name })).toBeInTheDocument())
    expect(screen.queryByTestId("checkout-items")).not.toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Mua ngay" })).not.toBeDisabled()
  })

  it("does not let stale Buy Now response state interrupt a new product request", async () => {
    let resolveA!: () => void
    let resolveB!: () => void
    const detail2 = { ...DETAIL, id: "dt-2", name: "Lotus 12A" }
    vi.mocked(getProduct).mockResolvedValueOnce(DETAIL).mockResolvedValueOnce(detail2)
    const addItem = vi.fn()
      .mockReturnValueOnce(new Promise<void>((resolve) => { resolveA = resolve }))
      .mockReturnValueOnce(new Promise<void>((resolve) => { resolveB = resolve }))
    const detail2Item = cartItem({ productId: detail2.id, name: detail2.name })
    renderDetail("dt-1", {}, "checkout", "/san-pham/dt-2", cartValue([detail2Item], addItem))

    await userEvent.click(await screen.findByRole("button", { name: "Mua ngay" }))
    await userEvent.click(screen.getByRole("button", { name: "Chuyển trang" }))
    expect(await screen.findByRole("heading", { level: 1, name: detail2.name })).toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: "Mua ngay" }))
    expect(addItem).toHaveBeenCalledTimes(2)

    await act(async () => {
      resolveA()
      await Promise.resolve()
    })

    expect(screen.getByRole("button", { name: "Đang xử lý..." })).toBeInTheDocument()
    expect(toast.error).not.toHaveBeenCalled()
    expect(screen.queryByTestId("checkout-items")).not.toBeInTheDocument()
    await act(async () => {
      resolveB()
      await Promise.resolve()
    })
  })

  it("never renders soft-delete or restore controls on storefront detail even in development", async () => {
    vi.stubEnv("DEV", true)
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    renderDetail()

    expect(await screen.findByRole("heading", { level: 1, name: DETAIL.name })).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Xóa mềm sản phẩm" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Khôi phục sản phẩm" })).not.toBeInTheDocument()
    expect(softDeleteProduct).not.toHaveBeenCalled()
  })

  it("displays discontinued alert and disables purchase controls when product is soft-deleted or UNAVAILABLE", async () => {
    vi.mocked(getProduct).mockResolvedValue({
      ...DETAIL,
      deleted: true,
      status: "UNAVAILABLE",
      availableQuantity: 0,
    })
    renderDetail()

    expect(await screen.findByRole("heading", { level: 1, name: DETAIL.name })).toBeInTheDocument()
    expect(screen.getByText(/Sản phẩm ngừng kinh doanh/i)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Ngừng kinh doanh" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Mua ngay" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Tăng số lượng" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Giảm số lượng" })).toBeDisabled()
  })

  it("displays out-of-stock state and disables purchase controls when status is OUT_OF_STOCK", async () => {
    vi.mocked(getProduct).mockResolvedValue({
      ...DETAIL,
      status: "OUT_OF_STOCK",
      availableQuantity: 0,
    })
    renderDetail()

    expect(await screen.findByRole("heading", { level: 1, name: DETAIL.name })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Tạm hết hàng" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Mua ngay" })).toBeDisabled()
    expect(screen.getByRole("button", { name: "Tăng số lượng" })).toBeDisabled()
  })

  it("caps quantity stepper at availableQuantity and shows low-stock badge when status is LIMITED", async () => {
    vi.mocked(getProduct).mockResolvedValue({
      ...DETAIL,
      status: "LIMITED",
      availableQuantity: 2,
    })
    renderDetail()

    expect(await screen.findByText("Sắp hết — Chỉ còn 2 sản phẩm")).toBeInTheDocument()
    const decrease = screen.getByRole("button", { name: "Giảm số lượng" })
    const increase = screen.getByRole("button", { name: "Tăng số lượng" })
    expect(decrease).toBeDisabled()
    expect(increase).not.toBeDisabled()

    await userEvent.click(increase)

    expect(screen.getByText("2")).toBeInTheDocument()
    expect(increase).toBeDisabled()
    expect(decrease).not.toBeDisabled()
    expect(screen.getByText("Mua tối đa 2 sản phẩm mỗi lần")).toBeInTheDocument()
  })

  it("handles mid-action Add to Cart race for OUT_OF_STOCK and UNAVAILABLE vs degraded inventory", async () => {
    vi.mocked(getProduct)
      .mockResolvedValueOnce({ ...DETAIL, status: "AVAILABLE", availableQuantity: 8 })
      .mockResolvedValueOnce({ ...DETAIL, status: "OUT_OF_STOCK", availableQuantity: 0 })
    vi.mocked(addCartItem).mockResolvedValueOnce({
      items: [cartItem({ status: "OUT_OF_STOCK", availableQuantity: 0 })],
      degraded: false,
    })
    const { unmount } = renderDetail()

    await userEvent.click(await screen.findByRole("button", { name: "Thêm vào giỏ" }))

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Sản phẩm vừa tạm hết hàng"))
    expect(screen.queryByRole("button", { name: "Đã thêm vào giỏ" })).not.toBeInTheDocument()
    await waitFor(() => expect(getProduct).toHaveBeenCalledTimes(2))
    unmount()

    vi.clearAllMocks()
    vi.mocked(getCart).mockResolvedValue({ items: [], degraded: false })
    vi.mocked(searchProducts).mockResolvedValue({ items: [], page: 1, pageSize: 20, totalItems: 0, totalPages: 0 })
    vi.mocked(getProduct).mockResolvedValue({ ...DETAIL, status: "UNKNOWN", inventoryDegraded: true })
    vi.mocked(addCartItem).mockResolvedValueOnce({
      items: [cartItem({ status: "UNAVAILABLE", availableQuantity: 0 })],
      degraded: true,
    })

    renderDetail()
    await userEvent.click(await screen.findByRole("button", { name: "Thêm vào giỏ" }))

    expect(await screen.findByRole("button", { name: "Đã thêm vào giỏ" })).toBeInTheDocument()
    expect(toast.error).not.toHaveBeenCalled()
    expect(getProduct).toHaveBeenCalledTimes(1)
  })

  it("formats float32 rating, renders full description alongside shortDescription, and toggles wishlist", async () => {
    vi.mocked(getProduct).mockResolvedValue({
      ...DETAIL,
      rating: 4.900000095367432,
    })
    renderDetail()

    expect(await screen.findByRole("heading", { level: 1, name: DETAIL.name })).toBeInTheDocument()
    expect(screen.getByText("4.9")).toBeInTheDocument()
    expect(screen.queryByText("4.900000095367432")).not.toBeInTheDocument()
    expect(screen.getByText(DETAIL.shortDescription!)).toBeInTheDocument()
    expect(screen.getByText(DETAIL.description)).toBeInTheDocument()

    const wishlistBtn = screen.getByRole("button", { name: "Thêm vào yêu thích" })
    expect(wishlistBtn).toHaveAttribute("aria-pressed", "false")
    await userEvent.click(wishlistBtn)
    expect(wishlistBtn).toHaveAttribute("aria-pressed", "true")
    expect(toast.success).toHaveBeenCalled()
  })

  it("reports an error toast instead of false success when clipboard API is unavailable on share", async () => {
    vi.mocked(getProduct).mockResolvedValue(DETAIL)
    const originalClipboard = navigator.clipboard
    Object.defineProperty(navigator, "clipboard", {
      value: undefined,
      configurable: true,
    })

    try {
      renderDetail()
      const shareBtn = await screen.findByRole("button", { name: "Chia sẻ sản phẩm" })
      await userEvent.click(shareBtn)

      expect(toast.error).toHaveBeenCalledWith("Không thể sao chép liên kết sản phẩm")
      expect(toast.success).not.toHaveBeenCalledWith("Đã sao chép liên kết sản phẩm")
    } finally {
      Object.defineProperty(navigator, "clipboard", {
        value: originalClipboard,
        configurable: true,
      })
    }
  })
})
