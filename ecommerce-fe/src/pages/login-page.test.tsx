import { afterEach, describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext } from "@/context/auth-context"
import LoginPage from "./login-page"

afterEach(() => {
  vi.unstubAllEnvs()
})

function renderLogin(loginMock = vi.fn().mockResolvedValue(undefined)) {
  const value = {
    token: null,
    user: null,
    ready: true,
    isAuthenticated: false,
    login: loginMock,
    register: vi.fn(),
    updateProfile: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
  }
  return {
    loginMock,
    ...render(
      <AuthContext value={value}>
        <MemoryRouter>
          <LoginPage />
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

  it("prefills customer credentials in development mode", async () => {
    vi.stubEnv("MODE", "development")
    const { loginMock } = renderLogin()

    expect(screen.getByLabelText("Email")).toHaveValue("customer1@gmail.com")
    expect(screen.getByLabelText("Mật khẩu")).toHaveValue("123456123456")

    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect(loginMock).toHaveBeenCalledWith({
      email: "customer1@gmail.com",
      password: "123456123456",
    })
  })

  it("switches credentials between development accounts", async () => {
    vi.stubEnv("MODE", "development")
    const { loginMock } = renderLogin()

    await userEvent.click(screen.getByRole("button", { name: "Admin" }))
    expect(screen.getByLabelText("Email")).toHaveValue("admin@example.com")
    expect(screen.getByLabelText("Mật khẩu")).toHaveValue("AdminPassword123!")

    await userEvent.click(screen.getByRole("button", { name: "Customer" }))
    expect(screen.getByLabelText("Email")).toHaveValue("customer1@gmail.com")
    expect(screen.getByLabelText("Mật khẩu")).toHaveValue("123456123456")

    await userEvent.click(screen.getByRole("button", { name: "Admin" }))
    await userEvent.click(screen.getByRole("button", { name: /đăng nhập/i }))

    expect(loginMock).toHaveBeenCalledWith({
      email: "admin@example.com",
      password: "AdminPassword123!",
    })
  })

  it("omits development credentials and preset controls in production", () => {
    vi.stubEnv("MODE", "production")
    renderLogin()

    expect(screen.getByLabelText("Email")).toHaveValue("")
    expect(screen.getByLabelText("Mật khẩu")).toHaveValue("")
    expect(screen.queryByRole("button", { name: "Customer" })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Admin" })).not.toBeInTheDocument()
  })
})
