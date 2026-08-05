import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router-dom"
import { AuthContext } from "@/context/auth-context"
import LoginPage from "./login-page"

function renderLogin(loginMock = vi.fn().mockResolvedValue(undefined)) {
  const value = {
    token: null,
    isAuthenticated: false,
    login: loginMock,
    register: vi.fn(),
    logout: vi.fn(),
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
})
