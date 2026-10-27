import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import * as api from "@/lib/api"
import { clearAccessToken, getAccessToken } from "@/lib/auth-session"
import { AuthProvider } from "./auth-provider"
import { useAuth } from "@/hooks/use-auth"

function AuthHarness() {
  const { status, user, hasAnyAdminAccess, can, login, logout } = useAuth()
  return (
    <div>
      <span data-testid="status">{status}</span>
      <span data-testid="email">{user?.email ?? "none"}</span>
      <span data-testid="admin-access">{String(hasAnyAdminAccess)}</span>
      <span data-testid="can-product-view">{String(can("product.view"))}</span>
      <span data-testid="can-rbac-manage">{String(can("rbac.manage"))}</span>
      <button
        type="button"
        onClick={() => {
          void login({ email: "ops@sshop.vn", password: "Password123!" }).catch(() => {})
        }}
      >
        Login
      </button>
      <button type="button" onClick={() => void logout()}>
        Logout
      </button>
    </div>
  )
}

describe("AuthProvider", () => {
  afterEach(() => {
    clearAccessToken()
    vi.restoreAllMocks()
  })

  it("restores operator session and computes effective permission checks", async () => {
    vi.spyOn(api, "refreshAccessToken").mockResolvedValue("token-1")
    vi.spyOn(api, "fetchCurrentUser").mockResolvedValue({
      userId: "u-admin",
      email: "ops@sshop.vn",
      fullName: "Vận hành kho",
      roles: ["INVENTORY_MANAGER"],
    })
    vi.spyOn(api, "fetchMyPermissions").mockResolvedValue(["product.view", "inventory.view"])

    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>
    )

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated"))
    expect(screen.getByTestId("email")).toHaveTextContent("ops@sshop.vn")
    expect(screen.getByTestId("admin-access")).toHaveTextContent("true")
    expect(screen.getByTestId("can-product-view")).toHaveTextContent("true")
    expect(screen.getByTestId("can-rbac-manage")).toHaveTextContent("false")
  })

  it("marks customer-only accounts as authenticated without admin console access", async () => {
    vi.spyOn(api, "refreshAccessToken").mockResolvedValue("token-cust")
    vi.spyOn(api, "fetchCurrentUser").mockResolvedValue({
      userId: "u-cust",
      email: "customer@sshop.vn",
      fullName: "Khách hàng",
      roles: ["CUSTOMER"],
    })
    vi.spyOn(api, "fetchMyPermissions").mockResolvedValue(["profile.view", "cart.write"])

    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>
    )

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("authenticated"))
    expect(screen.getByTestId("admin-access")).toHaveTextContent("false")
  })

  it("discards late bootstrap resolution if logout advances session epoch mid-flight", async () => {
    let resolvePermissions: ((perms: string[]) => void) | undefined
    vi.spyOn(api, "refreshAccessToken").mockResolvedValue("token-race")
    vi.spyOn(api, "fetchCurrentUser").mockResolvedValue({
      userId: "u-admin",
      email: "race@sshop.vn",
      fullName: "Admin",
      roles: ["ADMIN"],
    })
    vi.spyOn(api, "fetchMyPermissions").mockImplementation(
      () =>
        new Promise<string[]>((resolve) => {
          resolvePermissions = resolve
        })
    )
    vi.spyOn(api, "logoutRequest").mockResolvedValue(undefined)

    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>
    )

    await waitFor(() => expect(resolvePermissions).toBeDefined())
    await userEvent.click(screen.getByRole("button", { name: "Logout" }))
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated"))

    resolvePermissions?.(["product.view", "user.view"])
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated")
    expect(screen.getByTestId("email")).toHaveTextContent("none")
  })

  it("clears in-memory access token if session hydration fails during login", async () => {
    vi.spyOn(api, "refreshAccessToken").mockRejectedValue(new Error("no cookie"))
    vi.spyOn(api, "login").mockResolvedValue({
      accessToken: "ephemeral-login-token",
      tokenType: "Bearer",
      expiresIn: 900,
    })
    vi.spyOn(api, "fetchCurrentUser").mockRejectedValue(new Error("profile 503"))
    vi.spyOn(api, "fetchMyPermissions").mockResolvedValue(["product.view"])

    render(
      <AuthProvider>
        <AuthHarness />
      </AuthProvider>
    )

    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated"))
    await userEvent.click(screen.getByRole("button", { name: "Login" }))

    await waitFor(() => expect(api.fetchCurrentUser).toHaveBeenCalled())
    expect(getAccessToken()).toBeNull()
    expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated")
  })
})
