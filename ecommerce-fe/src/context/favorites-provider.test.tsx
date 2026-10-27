import { act, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { AuthContext, type AuthContextValue } from "@/context/auth-context"
import { ApiClientError } from "@/lib/api"
import type { CurrentUser, FavoriteData, FavoriteItem } from "@/lib/types"
import { useFavorites } from "@/hooks/use-favorites"
import { FavoritesProvider } from "./favorites-provider"

const mockGetFavorites = vi.fn<() => Promise<FavoriteData>>()
const mockAddFavoriteItem = vi.fn<(productId: string) => Promise<FavoriteData>>()
const mockRemoveFavoriteItem = vi.fn<(productId: string) => Promise<FavoriteData>>()
const mockClearFavorites = vi.fn<() => Promise<FavoriteData>>()

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>()
  return {
    ...actual,
    getFavorites: (...args: []) => mockGetFavorites(...args),
    addFavoriteItem: (productId: string) => mockAddFavoriteItem(productId),
    removeFavoriteItem: (productId: string) => mockRemoveFavoriteItem(productId),
    clearFavorites: (...args: []) => mockClearFavorites(...args),
  }
})

const USER_1: CurrentUser = {
  userId: "user-1",
  email: "customer1@example.com",
  fullName: "Customer One",
  roles: ["CUSTOMER"],
  phone: null,
  dateOfBirth: null,
  gender: null,
  provinceCode: null,
  wardCode: null,
  addressLine: null,
}

const USER_2: CurrentUser = {
  ...USER_1,
  userId: "user-2",
  email: "customer2@example.com",
  fullName: "Customer Two",
}

function makeFavoriteItem(productId: string): FavoriteItem {
  return {
    itemId: `fav-${productId}`,
    productId,
    name: `Product ${productId}`,
    brand: "Apple",
    categoryId: "dien-thoai",
    categoryName: "Điện thoại",
    price: 20000000,
    originalPrice: 22000000,
    rating: 4.9,
    reviewCount: 50,
    soldCount: 200,
    imageUrl: "",
    imageEmoji: "📱",
    imageHue: [210, 250],
    badge: "hot",
    variants: [{ label: "Đen 256GB", colorHex: "#111111" }],
    status: "AVAILABLE",
    availableQuantity: 10,
  }
}

function makeAuth(overrides: Partial<AuthContextValue> = {}): AuthContextValue {
  return {
    token: null,
    user: null,
    ready: true,
    isAuthenticated: false,
    login: vi.fn().mockResolvedValue(undefined),
    register: vi.fn().mockResolvedValue(undefined),
    updateProfile: vi.fn().mockResolvedValue(undefined),
    logout: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  }
}

function Probe() {
  const fav = useFavorites()
  return (
    <div>
      <span data-testid="count">{fav.totalCount}</span>
      <span data-testid="ids">{fav.items.map((i) => i.productId).join(",")}</span>
      <span data-testid="fav-dt1">{String(fav.isFavorite("dt-1"))}</span>
      <span data-testid="pending-dt1">{String(fav.isPending("dt-1"))}</span>
      <span data-testid="forbidden">{String(fav.forbidden)}</span>
      <span data-testid="loading">{String(fav.loading)}</span>
      <span data-testid="degraded">{String(fav.degraded)}</span>
      <span data-testid="error">{fav.loadError?.message ?? ""}</span>
      <button type="button" onClick={() => void fav.toggleFavorite("dt-1").catch(() => undefined)}>
        toggle-dt1
      </button>
      <button type="button" onClick={() => void fav.clearFavorites().catch(() => undefined)}>
        clear
      </button>
      <button type="button" onClick={() => void fav.retryLoad()}>
        retry
      </button>
    </div>
  )
}

describe("FavoritesProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("stays empty for guests without calling getFavorites", () => {
    render(
      <AuthContext value={makeAuth({ isAuthenticated: false, user: null })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    expect(mockGetFavorites).not.toHaveBeenCalled()
    expect(screen.getByTestId("count")).toHaveTextContent("0")
    expect(screen.getByTestId("forbidden")).toHaveTextContent("false")
  })

  it("loads customer favorites, toggles add/remove with pending state, and clears", async () => {
    mockGetFavorites.mockResolvedValueOnce({ items: [], degraded: false })

    let resolveAdd!: (data: FavoriteData) => void
    mockAddFavoriteItem.mockImplementationOnce(
      () => new Promise<FavoriteData>((resolve) => { resolveAdd = resolve })
    )
    mockRemoveFavoriteItem.mockResolvedValueOnce({ items: [], degraded: false })

    render(
      <AuthContext value={makeAuth({ isAuthenticated: true, token: "t1", user: USER_1 })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    await waitFor(() => expect(mockGetFavorites).toHaveBeenCalledTimes(1))

    act(() => {
      screen.getByRole("button", { name: "toggle-dt1" }).click()
    })
    await waitFor(() => expect(screen.getByTestId("pending-dt1")).toHaveTextContent("true"))

    await act(async () => {
      resolveAdd({ items: [makeFavoriteItem("dt-1")], degraded: true })
    })

    await waitFor(() => {
      expect(screen.getByTestId("pending-dt1")).toHaveTextContent("false")
      expect(screen.getByTestId("count")).toHaveTextContent("1")
      expect(screen.getByTestId("fav-dt1")).toHaveTextContent("true")
      expect(screen.getByTestId("degraded")).toHaveTextContent("true")
    })

    await act(async () => {
      screen.getByRole("button", { name: "toggle-dt1" }).click()
    })
    await waitFor(() => {
      expect(mockRemoveFavoriteItem).toHaveBeenCalledWith("dt-1")
      expect(screen.getByTestId("count")).toHaveTextContent("0")
      expect(screen.getByTestId("fav-dt1")).toHaveTextContent("false")
    })
  })

  it("marks forbidden=true without loadError when getFavorites returns 403 FORBIDDEN", async () => {
    mockGetFavorites.mockRejectedValueOnce(
      new ApiClientError(403, "You do not have permission to manage favorites.", "FORBIDDEN")
    )

    render(
      <AuthContext value={makeAuth({ isAuthenticated: true, token: "t-admin", user: { ...USER_1, roles: ["ADMIN"] } })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    await waitFor(() => expect(screen.getByTestId("forbidden")).toHaveTextContent("true"))
    expect(screen.getByTestId("error")).toHaveTextContent("")
    expect(screen.getByTestId("count")).toHaveTextContent("0")
  })

  it("records loadError on transient failures and recovers via retryLoad", async () => {
    mockGetFavorites
      .mockRejectedValueOnce(new ApiClientError(502, "Favorites service is unavailable.", "FAVORITES_UNAVAILABLE"))
      .mockResolvedValueOnce({ items: [makeFavoriteItem("dt-1")], degraded: false })

    render(
      <AuthContext value={makeAuth({ isAuthenticated: true, token: "t1", user: USER_1 })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    await waitFor(() => expect(screen.getByTestId("error")).toHaveTextContent("Favorites service is unavailable."))

    act(() => {
      screen.getByRole("button", { name: "retry" }).click()
    })

    await waitFor(() => {
      expect(screen.getByTestId("error")).toHaveTextContent("")
      expect(screen.getByTestId("count")).toHaveTextContent("1")
    })
  })

  it("ignores stale in-flight getFavorites and mutation responses after logout or user switch", async () => {
    let resolveUser1Load!: (data: FavoriteData) => void
    mockGetFavorites
      .mockImplementationOnce(() => new Promise<FavoriteData>((resolve) => { resolveUser1Load = resolve }))
      .mockResolvedValueOnce({ items: [makeFavoriteItem("dt-user2")], degraded: false })

    const { rerender } = render(
      <AuthContext value={makeAuth({ isAuthenticated: true, token: "t1", user: USER_1 })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    rerender(
      <AuthContext value={makeAuth({ isAuthenticated: true, token: "t2", user: USER_2 })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    await waitFor(() => expect(screen.getByTestId("ids")).toHaveTextContent("dt-user2"))

    await act(async () => {
      resolveUser1Load({ items: [makeFavoriteItem("dt-stale-user1")], degraded: false })
    })

    expect(screen.getByTestId("ids")).toHaveTextContent("dt-user2")

    let resolveStaleMutation!: (data: FavoriteData) => void
    mockAddFavoriteItem.mockImplementationOnce(
      () => new Promise<FavoriteData>((resolve) => { resolveStaleMutation = resolve })
    )

    await act(async () => {
      screen.getByRole("button", { name: "toggle-dt1" }).click()
    })
    await waitFor(() => expect(mockAddFavoriteItem).toHaveBeenCalledTimes(1))

    rerender(
      <AuthContext value={makeAuth({ isAuthenticated: false, token: null, user: null })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    await act(async () => {
      resolveStaleMutation({ items: [makeFavoriteItem("dt-1")], degraded: false })
    })

    expect(screen.getByTestId("count")).toHaveTextContent("0")
    expect(screen.getByTestId("ids")).toHaveTextContent("")
  })

  it("clears loading and preserves newer mutation state when a mutation succeeds before initial getFavorites settles", async () => {
    let resolveInitialLoad!: (data: FavoriteData) => void
    mockGetFavorites.mockImplementationOnce(
      () => new Promise<FavoriteData>((resolve) => { resolveInitialLoad = resolve })
    )
    mockAddFavoriteItem.mockResolvedValueOnce({
      items: [makeFavoriteItem("dt-1")],
      degraded: false,
    })

    render(
      <AuthContext value={makeAuth({ isAuthenticated: true, token: "t1", user: USER_1 })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    expect(screen.getByTestId("loading")).toHaveTextContent("true")

    await act(async () => {
      screen.getByRole("button", { name: "toggle-dt1" }).click()
    })

    await waitFor(() => {
      expect(screen.getByTestId("ids")).toHaveTextContent("dt-1")
      expect(screen.getByTestId("loading")).toHaveTextContent("false")
    })

    await act(async () => {
      resolveInitialLoad({ items: [], degraded: false })
    })

    expect(screen.getByTestId("ids")).toHaveTextContent("dt-1")
    expect(screen.getByTestId("loading")).toHaveTextContent("false")
  })

  it("preserves initial getFavorites snapshot and clears loading when an overlapping mutation rejects", async () => {
    let resolveInitialLoad!: (data: FavoriteData) => void
    mockGetFavorites.mockImplementationOnce(
      () => new Promise<FavoriteData>((resolve) => { resolveInitialLoad = resolve })
    )
    mockAddFavoriteItem.mockRejectedValueOnce(
      new ApiClientError(409, "Product is no longer available.", "PRODUCT_UNAVAILABLE")
    )

    render(
      <AuthContext value={makeAuth({ isAuthenticated: true, token: "t1", user: USER_1 })}>
        <FavoritesProvider>
          <Probe />
        </FavoritesProvider>
      </AuthContext>
    )

    expect(screen.getByTestId("loading")).toHaveTextContent("true")

    await act(async () => {
      screen.getByRole("button", { name: "toggle-dt1" }).click()
    })
    await waitFor(() => expect(mockAddFavoriteItem).toHaveBeenCalledTimes(1))

    await act(async () => {
      resolveInitialLoad({ items: [makeFavoriteItem("dt-existing")], degraded: false })
    })

    await waitFor(() => {
      expect(screen.getByTestId("ids")).toHaveTextContent("dt-existing")
      expect(screen.getByTestId("loading")).toHaveTextContent("false")
    })
  })
})
