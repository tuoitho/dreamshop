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
    updateQuantity,
    toggleSelected,
    removeSelected,
    clearCart,
    loadError,
    retryLoad,
  } = useCart()

  return (
    <div>
      <output data-testid="items">{items.map((entry) => `${entry.itemId}:${entry.quantity}`).join(",")}</output>
      <output data-testid="selected-total">{selectedTotal}</output>
      <output data-testid="selected-count">{selectedCount}</output>
      <button onClick={() => void addItem("product-1", "Red", 2)}>Add</button>
      <button onClick={() => toggleSelected("item-1")}>Toggle first</button>
      <button onClick={() => void removeSelected()}>Remove selected</button>
      <button onClick={() => void clearCart()}>Clear</button>
      <button onClick={() => void updateQuantity("item-1", 3)}>Update to 3</button>
      <button onClick={() => void updateQuantity("item-1", 4)}>Update to 4</button>
      {loadError && <output role="alert">{loadError.message}</output>}
      <button onClick={() => void retryLoad()}>Retry load</button>
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

  it.each([
    { label: "not ready", ready: false, isAuthenticated: false, user: null },
    { label: "not authenticated", ready: true, isAuthenticated: false, user: null },
    { label: "missing user", ready: true, isAuthenticated: true, user: null },
  ])("does not invoke mutation APIs when auth state is $label", async (auth) => {
    renderCart(auth)

    await userEvent.click(screen.getByRole("button", { name: "Add" }))
    await userEvent.click(screen.getByRole("button", { name: "Update to 3" }))
    await userEvent.click(screen.getByRole("button", { name: "Remove selected" }))
    await userEvent.click(screen.getByRole("button", { name: "Clear" }))

    expect(addCartItem).not.toHaveBeenCalled()
    expect(updateCartItemQuantity).not.toHaveBeenCalled()
    expect(removeCartItem).not.toHaveBeenCalled()
    expect(clearCart).not.toHaveBeenCalled()
    expect(screen.getByTestId("items")).toHaveTextContent("")
  })

  it("serializes rapid mutations so the next request waits for the previous response", async () => {
    vi.mocked(getCart).mockResolvedValue(cart([item({ quantity: 2 })]))
    let resolveFirst!: (value: CartData) => void
    vi.mocked(updateCartItemQuantity)
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
      .mockResolvedValueOnce(cart([item({ quantity: 4 })]))
    renderCart()

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("item-1:2"))
    await userEvent.click(screen.getByRole("button", { name: "Update to 3" }))
    await userEvent.click(screen.getByRole("button", { name: "Update to 4" }))

    expect(updateCartItemQuantity).toHaveBeenCalledTimes(1)
    resolveFirst(cart([item({ quantity: 3 })]))
    await waitFor(() => expect(updateCartItemQuantity).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("item-1:4"))
  })

  it("ignores an in-flight mutation response after logout", async () => {
    vi.mocked(getCart).mockResolvedValue(cart([item()]))
    let resolveMutation!: (value: CartData) => void
    vi.mocked(addCartItem).mockReturnValueOnce(new Promise((resolve) => { resolveMutation = resolve }))
    const view = renderCart()

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("item-1:2"))
    await userEvent.click(screen.getByRole("button", { name: "Add" }))
    view.rerender(
      <AuthContext value={authValue({ user: null, isAuthenticated: false })}>
        <CartProvider>
          <Consumer />
        </CartProvider>
      </AuthContext>
    )
    resolveMutation(cart([item({ itemId: "old-account-item" })]))

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent(""))
    expect(screen.getByTestId("items")).not.toHaveTextContent("old-account-item")
  })

  it("ignores an in-flight load response after switching accounts", async () => {
    let resolveFirstLoad!: (value: CartData) => void
    vi.mocked(getCart)
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirstLoad = resolve }))
      .mockResolvedValueOnce(cart([item({ itemId: "new-account-item" })]))
    const view = renderCart()

    view.rerender(
      <AuthContext value={authValue({ user: { ...USER, userId: "user-2" } })}>
        <CartProvider>
          <Consumer />
        </CartProvider>
      </AuthContext>
    )
    resolveFirstLoad(cart([item({ itemId: "old-account-item" })]))

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("new-account-item:2"))
    expect(screen.getByTestId("items")).not.toHaveTextContent("old-account-item")
  })

  it("retains the current cart when a mutation fails", async () => {
    vi.mocked(getCart).mockResolvedValue(cart([item()]))
    vi.mocked(updateCartItemQuantity).mockRejectedValueOnce(new Error("Update failed"))
    renderCart()

    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("item-1:2"))
    await userEvent.click(screen.getByRole("button", { name: "Update to 3" }))

    await waitFor(() => expect(updateCartItemQuantity).toHaveBeenCalledWith("item-1", 3))
    expect(screen.getByTestId("items")).toHaveTextContent("item-1:2")
    expect(screen.queryByRole("alert")).not.toBeInTheDocument()
  })

  it("exposes the initial load error and retries without treating it as an empty cart", async () => {
    vi.mocked(getCart)
      .mockRejectedValueOnce(new Error("Cart unavailable"))
      .mockResolvedValueOnce(cart([item()]))
    renderCart()

    expect(await screen.findByRole("alert")).toHaveTextContent("Cart unavailable")
    await userEvent.click(screen.getByRole("button", { name: "Retry load" }))
    await waitFor(() => expect(screen.getByTestId("items")).toHaveTextContent("item-1:2"))
    expect(getCart).toHaveBeenCalledTimes(2)
  })
})
