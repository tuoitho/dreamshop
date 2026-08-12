import { beforeEach, describe, expect, it, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { CartProvider } from "@/context/cart-provider"
import { useCart } from "@/hooks/use-cart"
import {
  addCartItem,
  clearCart,
  getCart,
  removeCartItem,
  updateCartItemQuantity,
} from "@/lib/api"
import type { CartData, CartItem, CurrentUser } from "@/lib/types"

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    addCartItem: vi.fn(),
    clearCart: vi.fn(),
    getCart: vi.fn(),
    removeCartItem: vi.fn(),
    updateCartItemQuantity: vi.fn(),
  }
})

const USER: CurrentUser = {
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
}

function item(overrides: Partial<CartItem> = {}): CartItem {
  return {
    itemId: "item-1",
    productId: "product-1",
    variantLabel: "16GB/512GB",
    quantity: 2,
    name: "Nova X15 Pro",
    brand: "Nova",
    categoryName: "Điện thoại",
    unitPrice: 100000,
    imageEmoji: "📱",
    imageHue: [220, 260],
    imageUrl: "",
    status: "AVAILABLE",
    availableQuantity: 10,
    ...overrides,
  }
}

function cart(items: CartItem[]): CartData {
  return { items, degraded: false }
}

function authValue(overrides: Partial<AuthContextValue> = {}): AuthContextValue {
  return {
    token: "token",
    user: USER,
    ready: true,
    isAuthenticated: true,
    login: vi.fn(),
    register: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn(),
    ...overrides,
  }
}

function Consumer() {
  const {
    items,
    selectedTotal,
    selectedCount,
    addItem,
    toggleSelected,
    removeSelected,
  } = useCart()

  return (
    <div>
      <output data-testid="items">{items.map((entry) => `${entry.itemId}:${entry.quantity}`).join(",")}</output>
      <output data-testid="selected-total">{selectedTotal}</output>
      <output data-testid="selected-count">{selectedCount}</output>
      <button onClick={() => void addItem("product-1", "Red", 2)}>Add</button>
      <button onClick={() => toggleSelected("item-1")}>Toggle first</button>
      <button onClick={() => void removeSelected()}>Remove selected</button>
    </div>
  )
}

function renderCart(auth: Partial<AuthContextValue> = {}) {
  return render(
    <AuthContext value={authValue(auth)}>
      <CartProvider>
        <Consumer />
      </CartProvider>
    </AuthContext>
  )
}

describe("CartProvider", () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    vi.mocked(getCart).mockResolvedValue(cart([]))
    vi.mocked(addCartItem).mockResolvedValue(cart([]))
    vi.mocked(updateCartItemQuantity).mockResolvedValue(cart([]))
    vi.mocked(removeCartItem).mockResolvedValue(cart([]))
    vi.mocked(clearCart).mockResolvedValue(cart([]))
  })

  it("selects a newly loaded available item and includes it in selectedTotal", async () => {
    vi.mocked(getCart).mockResolvedValue(cart([item()]))

    renderCart()

    await waitFor(() => expect(screen.getByTestId("selected-total")).toHaveTextContent("200000"))
    expect(screen.getByTestId("selected-count")).toHaveTextContent("2")
  })

  it("keeps out-of-stock items unselected and prevents selecting them", async () => {
    vi.mocked(getCart).mockResolvedValue(cart([item({ status: "OUT_OF_STOCK", availableQuantity: 0 })]))

    renderCart()

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("item-1:2"))
    await userEvent.click(screen.getByRole("button", { name: "Toggle first" }))

    expect(screen.getByTestId("selected-total")).toHaveTextContent("0")
    expect(screen.getByTestId("selected-count")).toHaveTextContent("0")
  })

  it("forwards variantLabel to addItem and displays the API response", async () => {
    const response = cart([item({ itemId: "item-2", variantLabel: "Red", quantity: 2, unitPrice: 125000 })])
    vi.mocked(addCartItem).mockResolvedValue(response)
    renderCart()

    await userEvent.click(screen.getByRole("button", { name: "Add" }))

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("item-2:2"))
    expect(addCartItem).toHaveBeenCalledWith({ productId: "product-1", variantLabel: "Red", quantity: 2 })
  })

  it("removes each selected item through the API", async () => {
    const second = item({ itemId: "item-2", productId: "product-2", unitPrice: 50000 })
    vi.mocked(getCart).mockResolvedValue(cart([item(), second]))
    vi.mocked(removeCartItem).mockResolvedValue(cart([]))
    renderCart()

    await waitFor(() => expect(screen.getByTestId("selected-count")).toHaveTextContent("4"))
    await userEvent.click(screen.getByRole("button", { name: "Remove selected" }))

    await waitFor(() => expect(removeCartItem).toHaveBeenCalledTimes(2))
    expect(removeCartItem).toHaveBeenNthCalledWith(1, "item-1")
    expect(removeCartItem).toHaveBeenNthCalledWith(2, "item-2")
  })

  it("does not invoke getCart when unauthenticated", async () => {
    renderCart({ user: null, isAuthenticated: false })

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent(""))
    expect(getCart).not.toHaveBeenCalled()
  })
})
