import { describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { CartContext, type CartContextValue } from "@/context/cart-context"
import {
  DEFAULT_FAVORITES_CONTEXT,
  FavoritesContext,
  type FavoritesContextValue,
} from "@/context/favorites-context"
import type { FavoriteItem } from "@/lib/types"
import FavoritesPage from "./favorites-page"

function makeCart(overrides: Partial<CartContextValue> = {}): CartContextValue {
  return {
    items: [],
    addItem: vi.fn().mockResolvedValue(undefined),
    updateQuantity: vi.fn().mockResolvedValue(undefined),
    removeItem: vi.fn().mockResolvedValue(undefined),
    removeSelected: vi.fn().mockResolvedValue(undefined),
    clearCart: vi.fn().mockResolvedValue(undefined),
    toggleSelected: vi.fn(),
    selectOnly: vi.fn(),
    toggleAllSelected: vi.fn(),
    selectedItems: [],
    selectedCount: 0,
    totalQuantity: 0,
    selectedTotal: 0,
    allSelected: false,
    loading: false,
    degraded: false,
    loadError: null,
    retryLoad: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function makeItem(overrides: Partial<FavoriteItem> = {}): FavoriteItem {
  return {
    itemId: "fav-1",
    productId: "dt-1",
    name: "iPhone 16 Pro",
    brand: "Apple",
    categoryId: "dien-thoai",
    categoryName: "Điện thoại",
    price: 26990000,
    originalPrice: 29990000,
    rating: 4.9,
    reviewCount: 120,
    soldCount: 540,
    imageUrl: "",
    imageEmoji: "📱",
    imageHue: [210, 250],
    badge: "hot",
    variants: [
      {
        skuId: "dt-1-v1",
        label: "Đen Titan 256GB",
        colorHex: "#1c1c1e",
        price: 26990000,
        originalPrice: 29990000,
      },
    ],
    status: "AVAILABLE",
    availableQuantity: 12,
    ...overrides,
  }
}

function renderFavoritesPage(
  favoritesOverrides: Partial<FavoritesContextValue> = {},
  cartOverrides: Partial<CartContextValue> = {},
) {
  const favoritesValue: FavoritesContextValue = {
    ...DEFAULT_FAVORITES_CONTEXT,
    ...favoritesOverrides,
  }
  const cartValue = makeCart(cartOverrides)
  return {
    favoritesValue,
    cartValue,
    ...render(
      <CartContext value={cartValue}>
        <FavoritesContext value={favoritesValue}>
          <MemoryRouter initialEntries={["/yeu-thich"]}>
            <FavoritesPage />
          </MemoryRouter>
        </FavoritesContext>
      </CartContext>
    ),
  }
}

describe("FavoritesPage", () => {
  it("renders loading skeleton while initial favorites load", () => {
    renderFavoritesPage({ loading: true, items: [] })
    expect(screen.getByRole("status", { name: "Đang tải danh sách yêu thích" })).toBeInTheDocument()
  })

  it("renders customer-only notice when forbidden=true", () => {
    renderFavoritesPage({ forbidden: true })
    expect(
      screen.getByText("Tính năng sản phẩm yêu thích chỉ dành cho tài khoản khách hàng.")
    ).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Về trang chủ" })).toHaveAttribute("href", "/")
  })

  it("renders load error alert and triggers retryLoad", async () => {
    const retryLoad = vi.fn().mockResolvedValue(undefined)
    renderFavoritesPage({
      loadError: { message: "Favorites service is unavailable." },
      retryLoad,
    })

    expect(screen.getByRole("alert")).toHaveTextContent("Favorites service is unavailable.")
    await userEvent.click(screen.getByRole("button", { name: "Thử lại" }))
    expect(retryLoad).toHaveBeenCalledTimes(1)
  })

  it("renders empty state with link to /tim-kiem when items is empty", () => {
    renderFavoritesPage({ items: [], totalCount: 0 })
    expect(screen.getByRole("heading", { name: "Chưa có sản phẩm yêu thích" })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "Khám phá sản phẩm" })).toHaveAttribute("href", "/tim-kiem")
  })

  it("renders degraded banner, filters by availability, removes/clears items, handles 1- vs multi-variant CTAs, and renders orphan fallback safely", async () => {
    const addItem = vi.fn().mockResolvedValue(undefined)
    const removeFavorite = vi.fn().mockResolvedValue(undefined)
    const clearFavorites = vi.fn().mockResolvedValue(undefined)

    const items: FavoriteItem[] = [
      makeItem({
        itemId: "fav-single",
        productId: "dt-single",
        name: "Tai nghe AirPods 4",
        variants: [{ skuId: "dt-single-v1", label: "Trắng tiêu chuẩn", price: 3990000 }],
        status: "LIMITED",
        availableQuantity: 3,
      }),
      makeItem({
        itemId: "fav-multi",
        productId: "dt-multi",
        name: "MacBook Pro M4",
        price: 39990000,
        maxPrice: 49990000,
        variants: [
          { skuId: "dt-multi-v1", label: "Đen 16GB/512GB", price: 39990000 },
          { skuId: "dt-multi-v2", label: "Bạc 24GB/1TB", price: 49990000 },
        ],
        status: "AVAILABLE",
        availableQuantity: 10,
      }),
      makeItem({
        itemId: "fav-oos",
        productId: "dt-oos",
        name: "Apple Watch Ultra",
        status: "OUT_OF_STOCK",
        availableQuantity: 0,
      }),
      makeItem({
        itemId: "fav-orphan",
        productId: "dt-orphan",
        name: "",
        price: 0,
        originalPrice: 0,
        variants: [],
        status: "UNAVAILABLE",
        availableQuantity: 0,
      }),
    ]

    renderFavoritesPage(
      {
        items,
        totalCount: items.length,
        degraded: true,
        removeFavorite,
        clearFavorites,
      },
      { addItem },
    )

    expect(screen.getByText(/chưa thể kiểm tra đầy đủ tồn kho/i)).toBeInTheDocument()
    expect(screen.getByText("Chỉ còn 3 sản phẩm")).toBeInTheDocument()

    // Orphan hard-deleted item fallback: no broken link, no "0 ₫", removable
    expect(screen.getByText("Sản phẩm không còn tồn tại")).toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "Sản phẩm không còn tồn tại" })).not.toBeInTheDocument()
    expect(screen.queryByText(/^0\s*₫$/)).not.toBeInTheDocument()

    // 1-variant item adds directly to cart with skuId, multi-variant item shows 'Từ' prefix
    expect(screen.getByText("Từ")).toBeInTheDocument()
    await userEvent.click(screen.getByRole("button", { name: "Thêm vào giỏ" }))
    expect(addItem).toHaveBeenCalledWith("dt-single", "dt-single-v1", 1)

    // Multi-variant item links to /san-pham/:id
    expect(screen.getByRole("link", { name: "Chọn phân loại" })).toHaveAttribute(
      "href",
      "/san-pham/dt-multi"
    )

    // Single remove on orphan item
    await userEvent.click(
      screen.getByRole("button", { name: "Bỏ yêu thích Sản phẩm không còn tồn tại" })
    )
    expect(removeFavorite).toHaveBeenCalledWith("dt-orphan")

    // Filter tabs
    await userEvent.click(screen.getByRole("button", { name: /Còn hàng/ }))
    expect(screen.getByText("Tai nghe AirPods 4")).toBeInTheDocument()
    expect(screen.getByText("MacBook Pro M4")).toBeInTheDocument()
    expect(screen.queryByText("Apple Watch Ultra")).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /Hết hàng \/ Ngừng bán/ }))
    expect(screen.getByText("Apple Watch Ultra")).toBeInTheDocument()
    expect(screen.getByText("Sản phẩm không còn tồn tại")).toBeInTheDocument()
    expect(screen.queryByText("Tai nghe AirPods 4")).not.toBeInTheDocument()

    // Clear all
    await userEvent.click(screen.getByRole("button", { name: "Xóa tất cả" }))
    expect(clearFavorites).toHaveBeenCalledTimes(1)
  })

  it("renders stock status badge even when product brand is empty", () => {
    renderFavoritesPage({
      items: [
        makeItem({
          itemId: "fav-nobrand",
          productId: "dt-nobrand",
          name: "Ốp lưng trong suốt",
          brand: "",
          status: "LIMITED",
          availableQuantity: 2,
        }),
      ],
      totalCount: 1,
    })

    expect(screen.getByText("Ốp lưng trong suốt")).toBeInTheDocument()
    expect(screen.getByText("Chỉ còn 2 sản phẩm")).toBeInTheDocument()
  })
})
