import { afterEach, describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes, useLocation, type InitialEntry } from "react-router-dom"
import { AuthContext } from "@/context/auth-context"
import LoginPage from "./login-page"

afterEach(() => {
  vi.unstubAllEnvs()
})

function LocationDisplay() {
  const location = useLocation()
  return <p data-testid="current-location">{`${location.pathname}${location.search}${location.hash}`}</p>
}

function renderLogin(
  loginMock = vi.fn().mockResolvedValue(undefined),
  initialEntries: InitialEntry[] = ["/login"],
  authOverrides: { ready?: boolean; isAuthenticated?: boolean } = {},
) {
  const value = {
    token: authOverrides.isAuthenticated ? "access-token" : null,
    user: null,
    ready: authOverrides.ready ?? true,
    isAuthenticated: authOverrides.isAuthenticated ?? false,
    login: loginMock,
    register: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
  }
  return {
    loginMock,
    ...render(
      <AuthContext value={value}>
        <MemoryRouter initialEntries={initialEntries}>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="*" element={<LocationDisplay />} />
          </Routes>
        </MemoryRouter>
      </AuthContext>
    ),
  }
}

describe("LoginPage", () => {
  it("submits email and password to the auth login function", async () => {
    const { loginMock } = renderLogin()
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "SecurePassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect(loginMock).toHaveBeenCalledWith({
      email: "customer@example.com",
      password: "SecurePassword123!",
    })
  })

  it("shows the backend error message when login fails", async () => {
    const { loginMock } = renderLogin(vi.fn().mockRejectedValue(new Error("Email or password is invalid.")))
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "SecurePassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect(loginMock).toHaveBeenCalled()
    expect(await screen.findByRole("alert")).toHaveTextContent("Email or password is invalid.")
  })

  it("prefills customer credentials in development mode and omits non-customer presets", async () => {
    vi.stubEnv("MODE", "development")
    const { loginMock } = renderLogin()

    expect(screen.getByLabelText("Email")).toHaveValue("customer1@gmail.com")
    expect(screen.getByLabelText("Mật khẩu")).toHaveValue("123456123456")
    expect(screen.queryByRole("button", { name: "Admin" })).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect(loginMock).toHaveBeenCalledWith({
      email: "customer1@gmail.com",
      password: "123456123456",
    })
  })

  it("displays customer-only rejection alert when non-customer login is denied", async () => {
    const { loginMock } = renderLogin(
      vi.fn().mockRejectedValue(new Error("Trang mua sắm chỉ dành cho tài khoản khách hàng."))
    )

    await userEvent.type(screen.getByLabelText("Email"), "admin@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "AdminPassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect(loginMock).toHaveBeenCalledWith({
      email: "admin@example.com",
      password: "AdminPassword123!",
    })
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Trang mua sắm chỉ dành cho tài khoản khách hàng."
    )
  })

  it("omits development credentials and preset controls in production", () => {
    vi.stubEnv("MODE", "production")
    renderLogin()

    expect(screen.getByLabelText("Email")).toHaveValue("")
    expect(screen.getByLabelText("Mật khẩu")).toHaveValue("")
    expect(screen.queryByRole("button", { name: "Customer" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Admin" })).not.toBeInTheDocument()
  })

  it("navigates to the home page after login when no redirect target is provided", async () => {
    renderLogin()
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "SecurePassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect((await screen.findByTestId("current-location")).textContent).toBe("/")
  })

  it("navigates back to the guarded route location after login", async () => {
    renderLogin(vi.fn().mockResolvedValue(undefined), [
      { pathname: "/login", state: { from: { pathname: "/gio-hang", search: "?chon=1", hash: "#tom-tat" } } },
    ])
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "SecurePassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect((await screen.findByTestId("current-location")).textContent).toBe("/gio-hang?chon=1#tom-tat")
  })

  it("navigates back to a string redirect path after login", async () => {
    renderLogin(vi.fn().mockResolvedValue(undefined), [
      { pathname: "/login", state: { from: "/san-pham/dt-1" } },
    ])
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "SecurePassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect((await screen.findByTestId("current-location")).textContent).toBe("/san-pham/dt-1")
  })

  it("falls back to the home page when redirect target is an external or protocol-relative URL", async () => {
    renderLogin(vi.fn().mockResolvedValue(undefined), [
      { pathname: "/login", state: { from: "//evil.example/phish" } },
    ])
    await userEvent.type(screen.getByLabelText("Email"), "customer@example.com")
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "SecurePassword123!")
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect((await screen.findByTestId("current-location")).textContent).toBe("/")
  })

  it("displays registration confirmation banner when redirected from /register", () => {
    renderLogin(vi.fn(), [{ pathname: "/login", state: { registered: true } }])

    expect(screen.getByRole("status")).toHaveTextContent(
      "Đăng ký thành công! Vui lòng đăng nhập để tiếp tục."
    )
  })

  it("redirects already-authenticated users away from /login", async () => {
    renderLogin(
      vi.fn(),
      [{ pathname: "/login", state: { from: "/profile" } }],
      { ready: true, isAuthenticated: true },
    )

    expect((await screen.findByTestId("current-location")).textContent).toBe("/profile")
  })
})
