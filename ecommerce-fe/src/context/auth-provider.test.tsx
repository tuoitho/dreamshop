import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { act, render, screen, waitFor } from "@testing-library/react"
import { AxiosError, AxiosHeaders, type InternalAxiosRequestConfig } from "axios"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { AuthProvider } from "./auth-provider"
import { useAuth } from "@/hooks/use-auth"
import { getCart, http, refreshHttp } from "@/lib/api"
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
  })
})
