import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { act, render, screen, waitFor } from "@testing-library/react"
import { AxiosError, AxiosHeaders, type InternalAxiosRequestConfig } from "axios"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { toast } from "sonner"
import { AuthProvider } from "./auth-provider"
import { useAuth } from "@/hooks/use-auth"
import { ApiClientError, getCart, http, refreshHttp } from "@/lib/api"
import { clearAccessToken, getAccessToken } from "@/lib/auth-session"
import type { CurrentUser } from "@/lib/types"
import { RequireAuth } from "@/routes/require-auth"

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

interface Deferred<T> {
  promise: Promise<T>
  resolve: (value: T) => void
  reject: (reason?: unknown) => void
}

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function okResponse<T>(config: InternalAxiosRequestConfig, data: T, status = 200) {
  return {
    data,
    status,
    statusText: "OK",
    headers: {},
    config,
  }
}

function httpError(config: InternalAxiosRequestConfig, status: number, code: string, message: string): AxiosError {
  return new AxiosError(
    message,
    "ERR_BAD_REQUEST",
    config,
    undefined,
    {
      status,
      data: { code, message, traceId: "trace-1" },
      statusText: "",
      headers: new AxiosHeaders(),
      config,
    }
  )
}

const BOOTSTRAP_USER: CurrentUser = {
  userId: "user-boot",
  email: "boot@example.com",
  fullName: "Bootstrap User",
  roles: ["CUSTOMER"],
  phone: null,
  dateOfBirth: null,
  gender: null,
  provinceCode: null,
  wardCode: null,
  addressLine: null,
}

const LOGIN_USER: CurrentUser = {
  userId: "user-login",
  email: "login@example.com",
  fullName: "Login User",
  roles: ["CUSTOMER"],
  phone: null,
  dateOfBirth: null,
  gender: null,
  provinceCode: null,
  wardCode: null,
  addressLine: null,
}

function AuthStatusProbe() {
  const { ready, isAuthenticated, token, user } = useAuth()
  return (
    <div>
      <span data-testid="ready">{String(ready)}</span>
      <span data-testid="authenticated">{String(isAuthenticated)}</span>
      <span data-testid="token">{token ?? "none"}</span>
      <span data-testid="user-email">{user?.email ?? "none"}</span>
    </div>
  )
}

let latestAuth: ReturnType<typeof useAuth> | null = null

function AuthControllerProbe() {
  latestAuth = useAuth()
  return <AuthStatusProbe />
}

describe("AuthProvider session synchronization and concurrency guards", () => {
  beforeEach(() => {
    latestAuth = null
    clearAccessToken()
    vi.clearAllMocks()
    delete http.defaults.adapter
    delete refreshHttp.defaults.adapter
  })

  afterEach(() => {
    clearAccessToken()
    delete http.defaults.adapter
    delete refreshHttp.defaults.adapter
  })

  it("resets AuthProvider state and redirects RequireAuth to /login when a 401 refresh fails", async () => {
    let refreshCalls = 0
    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      refreshCalls += 1
      if (refreshCalls === 1) {
        return okResponse(config, {
          data: { accessToken: "boot-jwt", tokenType: "Bearer", expiresIn: 900 },
          message: "Refreshed",
        })
      }
      throw httpError(config, 401, "INVALID_REFRESH_TOKEN", "Refresh token is invalid or expired")
    }) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/users/me") {
        return okResponse(config, BOOTSTRAP_USER)
      }
      if (config.url === "/cart") {
        throw httpError(config, 401, "UNAUTHORIZED", "Missing or invalid access token")
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <MemoryRouter initialEntries={["/protected"]}>
          <Routes>
            <Route
              path="/protected"
              element={
                <RequireAuth>
                  <div>
                    <p>Protected screen</p>
                    <AuthStatusProbe />
                  </div>
                </RequireAuth>
              }
            />
            <Route path="/login" element={<p>Login screen</p>} />
          </Routes>
        </MemoryRouter>
      </AuthProvider>
    )

    expect(await screen.findByText("Protected screen")).toBeInTheDocument()
    expect(screen.getByTestId("authenticated")).toHaveTextContent("true")
    expect(screen.getByTestId("user-email")).toHaveTextContent("boot@example.com")

    await act(async () => {
      await getCart().catch(() => undefined)
    })

    expect(await screen.findByText("Login screen")).toBeInTheDocument()
    expect(getAccessToken()).toBeNull()
  })

  it("preserves a newly logged-in session when slow bootstrap refresh fails afterwards", async () => {
    const bootstrapRefresh = deferred<{ accessToken: string }>()

    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      const payload = await bootstrapRefresh.promise
      return okResponse(config, {
        data: { accessToken: payload.accessToken, tokenType: "Bearer", expiresIn: 900 },
        message: "Refreshed",
      })
    }) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/auth/login") {
        return okResponse(config, {
          data: { accessToken: "login-jwt", tokenType: "Bearer", expiresIn: 900 },
          message: "Logged in",
        })
      }
      if (config.url === "/users/me") {
        return okResponse(config, LOGIN_USER)
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await act(async () => {
      await latestAuth!.login({ email: "login@example.com", password: "Secret123!" })
    })

    expect(screen.getByTestId("authenticated")).toHaveTextContent("true")
    expect(screen.getByTestId("token")).toHaveTextContent("login-jwt")
    expect(screen.getByTestId("user-email")).toHaveTextContent("login@example.com")

    await act(async () => {
      bootstrapRefresh.reject(new Error("Bootstrap refresh failed"))
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    expect(screen.getByTestId("authenticated")).toHaveTextContent("true")
    expect(screen.getByTestId("token")).toHaveTextContent("login-jwt")
    expect(screen.getByTestId("user-email")).toHaveTextContent("login@example.com")
    expect(getAccessToken()).toBe("login-jwt")
  })

  it("does not overwrite a newly logged-in session when slow bootstrap refresh resolves afterwards", async () => {
    const bootstrapRefresh = deferred<string>()

    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      const accessToken = await bootstrapRefresh.promise
      return okResponse(config, {
        data: { accessToken, tokenType: "Bearer", expiresIn: 900 },
        message: "Refreshed",
      })
    }) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/auth/login") {
        return okResponse(config, {
          data: { accessToken: "fresh-login-jwt", tokenType: "Bearer", expiresIn: 900 },
          message: "Logged in",
        })
      }
      if (config.url === "/users/me") {
        return okResponse(
          config,
          config.headers?.Authorization === "Bearer fresh-login-jwt" ? LOGIN_USER : BOOTSTRAP_USER
        )
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await act(async () => {
      await latestAuth!.login({ email: "login@example.com", password: "Secret123!" })
    })

    await act(async () => {
      bootstrapRefresh.resolve("stale-bootstrap-jwt")
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    expect(screen.getByTestId("token")).toHaveTextContent("fresh-login-jwt")
    expect(screen.getByTestId("user-email")).toHaveTextContent("login@example.com")
    expect(getAccessToken()).toBe("fresh-login-jwt")
  })

  it("prevents in-flight updateProfile and fetchCurrentUser promises from resurrecting state after logout", async () => {
    const profilePatch = deferred<CurrentUser>()

    refreshHttp.defaults.adapter = vi.fn(async (config) =>
      okResponse(config, {
        data: { accessToken: "boot-jwt", tokenType: "Bearer", expiresIn: 900 },
        message: "Refreshed",
      })
    ) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/users/me" && config.method?.toLowerCase() === "get") {
        return okResponse(config, BOOTSTRAP_USER)
      }
      if (config.url === "/users/me" && config.method?.toLowerCase() === "patch") {
        const updated = await profilePatch.promise
        return okResponse(config, { data: updated, message: "Profile updated." })
      }
      if (config.url === "/auth/logout") {
        return okResponse(config, { data: null, message: "Logged out." })
      }
      throw new Error(`Unexpected request: ${config.method} ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId("user-email")).toHaveTextContent("boot@example.com")
    })

    let pendingUpdate!: Promise<void>
    act(() => {
      pendingUpdate = latestAuth!.updateProfile({ fullName: "Updated After Logout" })
    })

    await act(async () => {
      await latestAuth!.logout()
    })

    expect(screen.getByTestId("authenticated")).toHaveTextContent("false")
    expect(screen.getByTestId("user-email")).toHaveTextContent("none")

    await act(async () => {
      profilePatch.resolve({ ...BOOTSTRAP_USER, fullName: "Updated After Logout" })
      await pendingUpdate
    })

    expect(screen.getByTestId("authenticated")).toHaveTextContent("false")
    expect(screen.getByTestId("token")).toHaveTextContent("none")
    expect(screen.getByTestId("user-email")).toHaveTextContent("none")
    expect(getAccessToken()).toBeNull()
  })

  it("prevents an in-flight login fetchCurrentUser from resurrecting a session after logout", async () => {
    const loginUserFetch = deferred<CurrentUser>()

    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      throw httpError(config, 401, "INVALID_REFRESH_TOKEN", "No refresh cookie")
    }) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/auth/login") {
        return okResponse(config, {
          data: { accessToken: "login-jwt", tokenType: "Bearer", expiresIn: 900 },
          message: "Logged in",
        })
      }
      if (config.url === "/users/me") {
        return okResponse(config, await loginUserFetch.promise)
      }
      if (config.url === "/auth/logout") {
        return okResponse(config, { data: null, message: "Logged out." })
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId("ready")).toHaveTextContent("true")
    })

    let pendingLogin!: Promise<void>
    act(() => {
      pendingLogin = latestAuth!.login({ email: "login@example.com", password: "Secret123!" })
    })

    await waitFor(() => {
      expect(getAccessToken()).toBe("login-jwt")
    })

    await act(async () => {
      await latestAuth!.logout()
    })

    await act(async () => {
      loginUserFetch.resolve(LOGIN_USER)
      await pendingLogin
    })

    expect(screen.getByTestId("authenticated")).toHaveTextContent("false")
    expect(screen.getByTestId("token")).toHaveTextContent("none")
    expect(screen.getByTestId("user-email")).toHaveTextContent("none")
    expect(getAccessToken()).toBeNull()
    expect(toast.success).not.toHaveBeenCalledWith("Logged in")
  })

  it("shows the login mutation success toast only after verifying pure CUSTOMER role", async () => {
    const loginUserFetch = deferred<CurrentUser>()

    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      throw httpError(config, 401, "INVALID_REFRESH_TOKEN", "No refresh cookie")
    }) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/auth/login") {
        return okResponse(config, {
          data: { accessToken: "customer-jwt", tokenType: "Bearer", expiresIn: 900 },
          message: "Login successful",
        })
      }
      if (config.url === "/users/me") {
        return okResponse(config, await loginUserFetch.promise)
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId("ready")).toHaveTextContent("true")
    })

    let pendingLogin!: Promise<void>
    await act(async () => {
      pendingLogin = latestAuth!.login({ email: "login@example.com", password: "Secret123!" })
    })

    await waitFor(() => {
      expect(getAccessToken()).toBe("customer-jwt")
    })
    expect(toast.success).not.toHaveBeenCalled()

    await act(async () => {
      loginUserFetch.resolve(LOGIN_USER)
      await pendingLogin
    })

    expect(screen.getByTestId("authenticated")).toHaveTextContent("true")
    expect(toast.success).toHaveBeenCalledTimes(1)
    expect(toast.success).toHaveBeenCalledWith("Login successful")
  })

  it.each([
    { label: "operator-only", roles: ["SUPER_ADMIN"] },
    { label: "dual customer and operator", roles: ["CUSTOMER", "SUPER_ADMIN"] },
    { label: "empty role set", roles: [] as string[] },
  ])("rejects login and revokes server session silently for $label accounts", async ({ roles }) => {
    let logoutCalls = 0
    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      throw httpError(config, 401, "INVALID_REFRESH_TOKEN", "No refresh cookie")
    }) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/auth/login") {
        return okResponse(config, {
          data: { accessToken: "non-customer-jwt", tokenType: "Bearer", expiresIn: 900 },
          message: "Login successful",
        })
      }
      if (config.url === "/users/me") {
        return okResponse(config, {
          ...LOGIN_USER,
          email: "admin@example.com",
          roles,
        })
      }
      if (config.url === "/auth/logout") {
        logoutCalls += 1
        return okResponse(config, { data: null, message: "Logout successful" })
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId("ready")).toHaveTextContent("true")
    })

    let caughtError: unknown = null
    await act(async () => {
      try {
        await latestAuth!.login({ email: "admin@example.com", password: "AdminPassword123!" })
      } catch (err) {
        caughtError = err
      }
    })

    expect(caughtError).toBeInstanceOf(ApiClientError)
    expect((caughtError as ApiClientError).code).toBe("CUSTOMER_ROLE_REQUIRED")
    expect((caughtError as ApiClientError).message).toBe("Trang mua sắm chỉ dành cho tài khoản khách hàng.")
    expect(logoutCalls).toBe(1)
    expect(screen.getByTestId("authenticated")).toHaveTextContent("false")
    expect(screen.getByTestId("token")).toHaveTextContent("none")
    expect(screen.getByTestId("user-email")).toHaveTextContent("none")
    expect(getAccessToken()).toBeNull()
    expect(toast.success).not.toHaveBeenCalled()
  })

  it("ignores non-customer sessions during bootstrap refresh without calling /auth/logout", async () => {
    let logoutCalls = 0
    refreshHttp.defaults.adapter = vi.fn(async (config) =>
      okResponse(config, {
        data: { accessToken: "admin-boot-jwt", tokenType: "Bearer", expiresIn: 900 },
        message: "Refreshed",
      })
    ) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/users/me") {
        return okResponse(config, {
          ...BOOTSTRAP_USER,
          email: "admin@example.com",
          roles: ["ADMIN", "SUPER_ADMIN"],
        })
      }
      if (config.url === "/auth/logout") {
        logoutCalls += 1
        return okResponse(config, { data: null, message: "Logout successful" })
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId("ready")).toHaveTextContent("true")
    })

    expect(logoutCalls).toBe(0)
    expect(screen.getByTestId("authenticated")).toHaveTextContent("false")
    expect(screen.getByTestId("token")).toHaveTextContent("none")
    expect(screen.getByTestId("user-email")).toHaveTextContent("none")
    expect(getAccessToken()).toBeNull()
  })

  it("preserves a newer customer login when slow rollback logout from a rejected non-customer login finishes later", async () => {
    const rollbackLogout = deferred<void>()

    refreshHttp.defaults.adapter = vi.fn(async (config) => {
      throw httpError(config, 401, "INVALID_REFRESH_TOKEN", "No refresh cookie")
    }) as never

    http.defaults.adapter = vi.fn(async (config) => {
      if (config.url === "/auth/login") {
        const body = typeof config.data === "string" ? JSON.parse(config.data) : config.data
        const accessToken = body.email === "admin@example.com" ? "admin-jwt" : "customer-jwt"
        return okResponse(config, {
          data: { accessToken, tokenType: "Bearer", expiresIn: 900 },
          message: "Login successful",
        })
      }
      if (config.url === "/users/me") {
        const isAdmin = config.headers?.Authorization === "Bearer admin-jwt"
        return okResponse(
          config,
          isAdmin
            ? { ...LOGIN_USER, email: "admin@example.com", roles: ["SUPER_ADMIN"] }
            : LOGIN_USER
        )
      }
      if (config.url === "/auth/logout") {
        await rollbackLogout.promise
        return okResponse(config, { data: null, message: "Logout successful" })
      }
      throw new Error(`Unexpected request: ${config.url}`)
    }) as never

    render(
      <AuthProvider>
        <AuthControllerProbe />
      </AuthProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId("ready")).toHaveTextContent("true")
    })

    let rejectedAdminLogin!: Promise<void>
    await act(async () => {
      rejectedAdminLogin = latestAuth!
        .login({ email: "admin@example.com", password: "AdminPassword123!" })
        .catch(() => undefined)
    })

    await waitFor(() => {
      expect(getAccessToken()).toBeNull()
    })

    await act(async () => {
      await latestAuth!.login({ email: "login@example.com", password: "Secret123!" })
    })

    await act(async () => {
      rollbackLogout.resolve()
      await rejectedAdminLogin
    })

    expect(screen.getByTestId("authenticated")).toHaveTextContent("true")
    expect(screen.getByTestId("token")).toHaveTextContent("customer-jwt")
    expect(screen.getByTestId("user-email")).toHaveTextContent("login@example.com")
    expect(getAccessToken()).toBe("customer-jwt")
  })
})
